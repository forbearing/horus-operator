package backup

import (
	"fmt"
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

// backupFunc
type backupFunc func(backupObj *storagev1alpha1.Backup, pvc string, meta pvdataMeta) error

// backupFactory
func backupFactory(storage types.Storage) backupFunc {
	return func(backupObj *storagev1alpha1.Backup, pvc string, meta pvdataMeta) error {
		beginTime := time.Now().UTC()
		defer func() {
			costedTime = time.Now().UTC().Sub(beginTime)
		}()

		// ==============================
		// for backup to different storage.
		// ==============================
		switch storage {
		case types.StorageNFS:
			logger.WithField("Storage", "NFS")
		case types.StorageMinIO:
			logger.WithField("Storage", "MinIO")
		case types.StorageS3:
			logger.WithField("Storage", "S3")
		case types.StorageCephFS:
			logger.WithField("Storage", "CephFS")
		case types.StorageRClone:
			logger.WithField("Storage", "RClone")
		case types.StorageSFTP:
			logger.WithField("Storage", "SFTP")
		case types.StorageRestServer:
			logger.WithField("Storage", "RestServer")
		default:
			return fmt.Errorf("not support storage type: %s", storage)
		}

		// Block here until waiting for pod/deployment/statefulset/daemonset to be ready and available.
		// These k8s resource are what we should backup to storage.
		var err error
		name := backupObj.Spec.BackupFrom.Name
		resource := backupObj.Spec.BackupFrom.Resource
		namespace := backupObj.GetNamespace()
		switch resource {
		case storagev1alpha1.PodResource:
			if err = podHandler.WithNamespace(namespace).WaitReady(name); err != nil {
				return errors.Wrapf(err, "pod handler wait pod/%s to be ready failed", name)
			}
		case storagev1alpha1.DeploymentResource:
			if err = depHandler.WithNamespace(namespace).WaitReady(name); err != nil {
				return errors.Wrapf(err, "deployment handler wait deployment/%s to be ready failed", name)
			}
		case storagev1alpha1.StatefulSetResource:
			if err = stsHandler.WithNamespace(namespace).WaitReady(name); err != nil {
				return errors.Wrapf(err, "statefulset handler wait statefulset/%s to be ready failed", name)
			}
		case storagev1alpha1.DaemonSetResource:
			if err = dsHandler.WithNamespace(namespace).WaitReady(name); err != nil {
				return errors.Wrapf(err, "daemonset handler wait daemonset/%s to be ready failed", name)
			}
		default:
			return fmt.Errorf("not support backup resource: %s", resource)
		}

		// ==============================
		// for backup to different storage.
		// ==============================
		var execPod *corev1.Pod
		switch storage {
		case types.StorageNFS:
			if execPod, err = createBackup2nfsDeployment(backupObj, meta); err != nil {
				return err
			}
			logger.WithFields(logrus.Fields{"Cost": costedTime.String()}).Debugf("create deployment/%s", backup2nfsName+"-"+meta.nodeName)
		case types.StorageMinIO:
			if execPod, err = createBackup2minioDepoyment(backupObj, meta); err != nil {
				return err
			}
			logger.WithFields(logrus.Fields{"Cost": costedTime.String()}).Debugf("create deployment/%s", backup2minioName+"-"+meta.nodeName)
		}

		// execute restic command to backup persistentvolume data to remote storage within the pod.
		if err = executeBackupCommand(backupObj, execPod, pvc, meta); err != nil {
			return err
		}
		return nil
	}
}

package backup

import (
	"fmt"
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

// BackupFunc
type BackupFunc func(backupObj *storagev1alpha1.Backup, pvc string, meta pvdataMeta) error

// BackupFactory
func BackupFactory(storage types.Storage) BackupFunc {
	return func(backupObj *storagev1alpha1.Backup, pvc string, meta pvdataMeta) error {
		beginTime := time.Now()
		logger := logrus.WithFields(logrus.Fields{
			"Tool": "Restic",
		})
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
		switch backupObj.Spec.BackupFrom.Resource {
		case storagev1alpha1.PodResource:
			if err = podHandler.WithNamespace(backupObj.GetNamespace()).WaitReady(backupObj.Spec.BackupFrom.Name); err != nil {
				logger.Errorf("pod handler wait pod/%s to be ready failed: %s", backupObj.Spec.BackupFrom.Name, err.Error())
				return err
			}
		case storagev1alpha1.DeploymentResource:
			if err = depHandler.WithNamespace(backupObj.GetNamespace()).WaitReady(backupObj.Spec.BackupFrom.Name); err != nil {
				logger.Errorf("deployment handler wait deployment/%s to be ready failed: %s", backupObj.Spec.BackupFrom.Name, err.Error())
				return err
			}
		case storagev1alpha1.StatefulSetResource:
			if err = stsHandler.WithNamespace(backupObj.GetNamespace()).WaitReady(backupObj.Spec.BackupFrom.Name); err != nil {
				logger.Errorf("statefulset handler wait statefulset/%s to be ready failed: %s", backupObj.Spec.BackupFrom.Name, err.Error())
				return err
			}
		case storagev1alpha1.DaemonSetResource:
			if err = dsHandler.WithNamespace(backupObj.GetNamespace()).WaitReady(backupObj.Spec.BackupFrom.Name); err != nil {
				logger.Errorf("daemonset handler wait daemonset/%s to be ready failed: %s", backupObj.Spec.BackupFrom.Name, err.Error())
				return err
			}
		default:
			err := fmt.Errorf("not support backup resource: %s", backupObj.Spec.BackupFrom.Resource)
			return err
		}

		var costedTime time.Duration
		var execPod *corev1.Pod
		// ==============================
		// for backup to different storage.
		// ==============================
		switch storage {
		case types.StorageNFS:
			if execPod, costedTime, err = createBackup2nfsDeployment(backupObj, meta); err != nil {
				return err
			}
			logger.WithFields(logrus.Fields{"Cost": costedTime.String()}).Debugf("create deployment/%s", backup2nfsName+"-"+meta.nodeName)
		case types.StorageMinIO:
			if execPod, costedTime, err = createBackup2minioDepoyment(backupObj, meta); err != nil {
				return err
			}
			logger.WithFields(logrus.Fields{"Cost": costedTime.String()}).Debugf("create deployment/%s", backup2minioName+"-"+meta.nodeName)
		}

		// execute restic command to backup persistentvolume data to remote storage within the pod.
		if costedTime, err = executeBackupCommand(backupObj, execPod, pvc, meta); err != nil {
			return err
		}
		costedTime = time.Now().Sub(beginTime)
		logger.WithFields(logrus.Fields{"Cost": costedTime.String()}).Infof("Successfully backup pvc/%s", pvc)
		return nil
	}
}

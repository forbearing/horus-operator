package backup

import (
	"fmt"
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

// backup2MinIO create deployment/backuptominio to backup every pvc volume data
// there are two condition should meet.
//   1.deployment mount the persistentvolumeclaim we should backup
//   2.execute restic commmand to backup persistentvolumeclaim data
//     - "restic list keys" check whether resitc repository exist
//     - "restic init" initial a resitc repository when repository not exist.
//     - "restic backup" backup the persistentvolume data to nfs storage.
func backup2MinIO(backupObj *storagev1alpha1.Backup, pvc string, meta pvdataMeta) error {
	beginTime := time.Now()
	logger := logrus.WithFields(logrus.Fields{
		"Component": "Backup2MinIO",
		"Tool":      "Restic",
		"Storage":   "MinIO",
	})

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
	if execPod, err = createBackup2minioDepoyment(backupObj, meta); err != nil {
		return err
	}
	logger.WithFields(logrus.Fields{"Cost": costedTime.String()}).Debugf("create deployment/%s", backup2minioName+"-"+meta.nodeName)

	// execute restic command to backup persistentvolume data to remote storage within the pod.
	if err = executeBackupCommand(backupObj, execPod, pvc, meta); err != nil {
		return err
	}
	costedTime = time.Now().Sub(beginTime)
	logger.WithFields(logrus.Fields{"Cost": costedTime.String()}).Infof("Successfully backup pvc/%s", pvc)
	return nil
}

package backup

import (
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

// Backup2NFS create deployment/backuptonfs to backup every pvc volume data.
// there are three condition should meet.
//   1.deployment mount the persistentvolumeclaim we should backup
//   2.deployment mount nfs storage as persistentvolumeclaim
//   3.execute restic commmand to backup persistentvolumeclaim data
//     - "restic list keys" check whether resitc repository exist
//     - "restic init" initial a resitc repository when repository not exist.
//     - "restic backup" backup the persistentvolume data to nfs storage.
func Backup2NFS(backupObj *storagev1alpha1.Backup, pvc string, meta pvdataMeta) (time.Duration, error) {
	beginTime := time.Now()
	logger := logrus.WithFields(logrus.Fields{
		"Component": "Backup",
		"Tool":      "Restic",
		"Storage":   "NFS",
	})

	var err error
	var costedTime time.Duration
	var execPod *corev1.Pod
	if execPod, costedTime, err = createBackup2nfsDeployment(backupObj, meta); err != nil {
		return time.Now().Sub(beginTime), err
	}
	logger.WithFields(logrus.Fields{"Cost": costedTime.String()}).Debugf("create deployment/%s", backup2nfsName+"-"+meta.nodeName)

	// execute restic command to backup persistentvolume data to remote storage within the pod.
	if costedTime, err = executeBackupCommand(backupObj, execPod, pvc, meta, types.StorageNFS); err != nil {
		return time.Now().Sub(beginTime), err
	}
	logger.WithFields(logrus.Fields{"Cost": costedTime.String()}).Infof("Successfully backup pvc/%s", pvc)
	return time.Now().Sub(beginTime), nil
}

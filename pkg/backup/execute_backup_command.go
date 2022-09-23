package backup

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/forbearing/horus-operator/pkg/util"
	"github.com/forbearing/restic"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

// executeBackupCommand
// clusterName as the argument of flag --host.
func executeBackupCommand(backupObj *storagev1alpha1.Backup, execPod *corev1.Pod, pvc string, meta pvdataMeta, storage string) (time.Duration, error) {
	beginTime := time.Now()
	operatorNamespace := util.GetOperatorNamespace()
	podHandler.ResetNamespace(operatorNamespace)
	logger := logrus.WithFields(logrus.Fields{
		"Component":         "execute",
		"OperatorNamespace": operatorNamespace,
		"Node":              meta.nodeName,
	})

	if len(meta.pvdir) == 0 {
		logger.Debug("persistentvolume directory is empty, skip backup")
		return time.Now().Sub(beginTime), nil
	}
	if len(meta.pvname) == 0 {
		logger.Debug("persistentvolume name is empty, skip backup")
		return time.Now().Sub(beginTime), nil
	}
	clusterName := backupObj.Spec.Cluster
	if len(clusterName) == 0 {
		clusterName = types.DefaultClusterName
	}

	pvpath := filepath.Join(mountHostRootPath, meta.pvdir, meta.pvname)
	switch meta.volumeSource {
	// if persistentvolume volume source is "hostPath" or "local", it's mean that
	// the meta.pvdir is pvpath not pvdir, and pvpath = pvdir + pvname.
	case types.VolumeHostPath, types.VolumeLocal:
		pvpath = filepath.Join(mountHostRootPath, meta.pvdir)
	}
	logger.Debugf("the path of persistentvolume data in k8s node: %s", pvpath)
	logger.Debugf("executing restic command to backup persistentvolume data within pod/%s", execPod.GetName())
	res := restic.NewIgnoreNotFound(context.TODO(), &restic.GlobalFlags{NoCache: true})
	tags := []string{string(backupObj.Spec.BackupFrom.Resource), backupObj.Namespace, backupObj.Spec.BackupFrom.Name, pvc}
	cmdCheckRepo := res.Command(restic.List{}.SetArgs("keys")).String()
	cmdInitRepo := res.Command(restic.Init{}).String()
	cmdBackup := res.Command(restic.Backup{Tag: tags, Host: clusterName}.SetArgs(pvpath)).String()

	// if `restic list keys` failed, it's means that the rstic repository not exist,
	// we should execute `restic init` command to init restic repository.
	logger.Debug(cmdCheckRepo)
	if err := podHandler.ExecuteWithStream(execPod.GetName(), "", strings.Split(cmdCheckRepo, " "), os.Stdin, io.Discard, io.Discard); err != nil {
		logger.Debug(cmdInitRepo)
		// if `restic init` failed, the next backup task wil not be continue.
		if err := podHandler.ExecuteWithStream(execPod.GetName(), "", strings.Split(cmdInitRepo, " "), os.Stdin, io.Discard, io.Discard); err != nil {
			logger.Fatal("restic init failed")
			return time.Now().Sub(beginTime), nil
		}
	}
	logger.Debug(cmdBackup)
	if err := podHandler.WithNamespace(operatorNamespace).ExecuteWithStream(execPod.GetName(), "", strings.Split(cmdBackup, " "), os.Stdin, io.Discard, io.Discard); err != nil {
		logger.Errorf("restic backup pvc/%s failed, maybe the directory/file of %s do not exist in k8s node", pvc, pvpath)
	}

	return time.Now().Sub(beginTime), nil
}
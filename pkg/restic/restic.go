package restic

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/forbearing/horus-operator/pkg/util"
	"github.com/forbearing/k8s/pod"
	res "github.com/forbearing/restic"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

var (
	ctx        = context.TODO()
	podHandler = pod.NewOrDie(ctx, "", "")
)

func Snapshots(ctx context.Context, storage types.Storage, cluster []string, tags []string, cmdOutput io.Writer) error {
	var (
		err     error
		execPod *corev1.Pod
		podsObj []*corev1.Pod
	)

	operatorNamespace := util.GetOperatorNamespace()
	podHandler.ResetNamespace(operatorNamespace)

	switch storage {
	case types.StorageNFS:
		if podsObj, err = podHandler.ListByLabel(types.Backup2NFSDeployLabel); err != nil {
			err = errors.Wrapf(err, "pod handler list pods in namespace/%s by labels failed", operatorNamespace)
			logrus.Error(err)
			return err
		}
		execPod = filterRunningPod(podsObj)
		if execPod == nil {
			err = errors.Wrapf(err, "not found running pod in namespace/%s with label %s", operatorNamespace, types.Backup2NFSDeployLabel)
			logrus.Error(err)
			return err
		}
	case types.StorageMinIO:
		if podsObj, err = podHandler.ListByLabel(types.Backup2MinioDeployLabel); err != nil {
			err = errors.Wrapf(err, "pod handler list pods in namespace/%s by labels failed", operatorNamespace)
			logrus.Error(err)
			return err
		}
		execPod = filterRunningPod(podsObj)
		if execPod == nil {
			err = errors.Wrapf(err, "not found running pod in namespace/%s with label %s", operatorNamespace, types.Backup2MinioDeployLabel)
			logrus.Error(err)
			return err
		}
	default:
		err := fmt.Errorf("not support storage type %s", storage)
		logrus.Error(err)
		return err
	}

	r := res.NewIgnoreNotFound(context.TODO(), &res.GlobalFlags{NoCache: true})
	cmdSnapshot := r.Command(res.Snapshots{Tag: tags, Host: cluster}).String()

	logrus.Debugf(`execute command "%s" within "pod/%s"`, cmdSnapshot, execPod.GetName())
	if err := podHandler.ExecuteWithStream(execPod.GetName(), "", strings.Split(cmdSnapshot, " "), os.Stdin, cmdOutput, io.Discard); err != nil {
		err = errors.Wrapf(err, `pod handler exec command "%s" within pod/%s failed`, cmdSnapshot, execPod.GetName())
		logrus.Error(err)
		return err
	}

	return nil
}

func Stats() {

}

// filterRunningPod
func filterRunningPod(podsObj []*corev1.Pod) *corev1.Pod {
	if podsObj == nil {
		return nil
	}
	execPod := &corev1.Pod{}
	for i := range podsObj {
		execPod = podsObj[i]
		if execPod.Status.Phase == corev1.PodRunning {
			return execPod
		}
	}
	return nil
}

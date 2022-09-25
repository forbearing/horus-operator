package restic

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/forbearing/horus-operator/pkg/util"
	"github.com/forbearing/k8s/pod"
	"github.com/forbearing/restic"
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
			logrus.Errorf("pod handler list pods in namespace/%s by labels error: %s", operatorNamespace, err.Error())
			return err
		}
		execPod = filterRunningPod(podsObj)
		if execPod == nil {
			logrus.Errorf(`not found running pod in namespace/%s with label "%s"`, operatorNamespace, types.Backup2NFSDeployLabel)
			return err
		}
	case types.StorageMinIO:
		if podsObj, err = podHandler.ListByLabel(types.Backup2MinioDeployLabel); err != nil {
			logrus.Errorf("pod handler list pods in namespace/%s by labels error: %s", operatorNamespace, err.Error())
			return err
		}
		execPod = filterRunningPod(podsObj)
		if execPod == nil {
			logrus.Errorf(`not found running pod in namespace/%s with label "%s"`, operatorNamespace, types.Backup2MinioDeployLabel)
			return err
		}
	default:
		logrus.Errorf(`not support storage type: %s`, storage)
		return err
	}

	res := restic.NewIgnoreNotFound(context.TODO(), &restic.GlobalFlags{NoCache: true})
	cmdSnapshot := res.Command(restic.Snapshots{Tag: tags, Host: cluster}).String()

	logrus.Debugf(`execute command "%s" within "pod/%s"`, cmdSnapshot, execPod.GetName())
	if err := podHandler.ExecuteWithStream(execPod.GetName(), "", strings.Split(cmdSnapshot, " "), os.Stdin, cmdOutput, cmdOutput); err != nil {
		logrus.Errorf(`pod handler execute command "%s" wthin pod/"%s" failed`, cmdSnapshot, execPod.GetName())
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

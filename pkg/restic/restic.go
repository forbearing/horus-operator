package restic

import (
	"context"
	"strings"

	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/forbearing/horus-operator/pkg/util"
	"github.com/forbearing/k8s/deployment"
	"github.com/forbearing/k8s/pod"
	"github.com/forbearing/restic"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

var (
	ctx        = context.TODO()
	podHandler = pod.NewOrDie(ctx, "", "")
	depHandler = deployment.NewOrDie(ctx, "", "")
)

func Snapshot(storage string, cluster []string, tags []string) {
	podHandler.ResetNamespace(util.GetOperatorNamespace())

	var (
		err     error
		execPod *corev1.Pod
		podsObj []*corev1.Pod
	)

	switch storage {
	case types.StorageNFS:
		if podsObj, err = podHandler.ListByLabel(types.Backup2NFSDeployLabel); err != nil {
			logrus.Error("pod handler list pods by labels error: %s", err.Error())
			return
		}
		execPod = filteRunningPod(podsObj)
		if execPod == nil {
			logrus.Errorf("not found running pod for %s", types.Backup2NFSDeployName)
			return
		}
	case types.StorageMinIO:
		if podsObj, err = podHandler.ListByLabel(types.Backup2MinioDeployLabel); err != nil {
			logrus.Error("pod handler list pods by labels error: %s", err.Error())
			return
		}
		execPod = filteRunningPod(podsObj)
		if execPod == nil {
			logrus.Errorf("not found running pod for %s", types.Backup2MinioDeployName)
			return
		}
	default:
		logrus.Errorf(`not support storage type: %s`, storage)
		return
	}

	res := restic.NewIgnoreNotFound(context.TODO(), &restic.GlobalFlags{NoCache: true})
	cmdSnapshot := res.Command(restic.Snapshots{Tag: tags, Host: cluster}).String()

	logrus.Debugf(`execute command "%s" within "pod/%s"`, cmdSnapshot, execPod.GetName())
	if err := podHandler.Execute(execPod.GetName(), "", strings.Split(cmdSnapshot, " ")); err != nil {
		logrus.Errorf(`pod handler execute command "%s" wthin pod/"%s" error`, cmdSnapshot, execPod.GetName())
		return
	}
}

func Stats() {

}

func filteRunningPod(podsObj []*corev1.Pod) *corev1.Pod {
	execPod := &corev1.Pod{}
	for i := range podsObj {
		execPod = podsObj[i]
		if execPod.Status.Phase == corev1.PodRunning {
			return execPod
		}
	}
	return nil
}

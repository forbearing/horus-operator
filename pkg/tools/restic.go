package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	storagev1alpha "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/k8s/daemonset"
	"github.com/forbearing/k8s/deployment"
	"github.com/forbearing/k8s/persistentvolume"
	"github.com/forbearing/k8s/pod"
	"github.com/forbearing/k8s/statefulset"
	"github.com/forbearing/restic"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	resticBackupSource = "/backup-source"
	resticRepo         = "/restic-repo"
	resticPasswd       = "mypass"
)

const (
	HostBackupToNFS   = "backup-to-nfs"
	HostBackupToS3    = "backup-to-S3"
	HostBackupToMinio = "backup-to-minio"
)

var (
	ErrRepoNotSet   = errors.New("restic repository not set")
	ErrPasswdNotSet = errors.New("restic password not set")
	ErrNotFound     = errors.New(`command "restic" not found`)
)

var (
	createdTimeAnnotation   = "storage.hybfkuf.io/createdAt"
	updatedTimeAnnotation   = "storage.hybfkuf.io/updatedAt"
	restartedTimeAnnotation = "storage.hybfkuf.io/restartedAt"
)

// podObj: 是要备份的 pv 所挂载到到 pod
// nfs: 将数据备份到 NFS
func BackupToNFS(ctx context.Context, operatorNamespace string, backupFrom *storagev1alpha.BackupFrom, nfs *storagev1alpha.NFS) error {
	logrus.SetLevel(logrus.DebugLevel)

	var (
		err           error
		podHandler    *pod.Handler
		deployHandler *deployment.Handler
		stsHandler    *statefulset.Handler
		dsHandler     *daemonset.Handler

		nodeName   string
		podUID     string
		pvpath     = new(bytes.Buffer)
		podObjList []*corev1.Pod

		findpvpathName  = "findpvpath"
		findpvpathImage = "hybfkuf/findpvpath:latest"
		findpvpathPods  = []*corev1.Pod{}
		findpvpathPod   = &corev1.Pod{}

		backuptonfsName  = "backup-to-nfs"
		backuptonfsImage = "hybfkuf/backup-tools-restic:latest"
		backuptonfsPods  = []*corev1.Pod{}
		backuptonfsPod   = &corev1.Pod{}
	)

	// === 准备处理器
	if deployHandler, err = deployment.New(ctx, "", operatorNamespace); err != nil {
		return err
	}
	if stsHandler, err = statefulset.New(ctx, "", operatorNamespace); err != nil {
		return err
	}
	if dsHandler, err = daemonset.New(ctx, "", operatorNamespace); err != nil {
		return err
	}
	if podHandler, err = pod.New(ctx, "", operatorNamespace); err != nil {
		return err
	}

	switch backupFrom.Resource {
	case storagev1alpha.PodResource:
		podObj, err := podHandler.Get(backupFrom.Name)
		if err != nil {
			return err
		}
		podObjList = append(podObjList, podObj)
	case storagev1alpha.DeploymentResource:
		if podObjList, err = deployHandler.GetPods(backupFrom.Name); err != nil {
			return err
		}
	case storagev1alpha.StatefulSetResource:
		if podObjList, err = stsHandler.GetPods(backupFrom.Name); err != nil {
			return err
		}
	case storagev1alpha.DaemonSetResource:
		if podObjList, err = dsHandler.GetPods(backupFrom.Name); err != nil {
			return err
		}
	}

	for _, podObj := range podObjList {
		if nodeName, err = podHandler.GetNodeName(podObj); err != nil {
			return err
		}
		if podUID, err = podHandler.GetUID(podObj); err != nil {
			return err
		}

		// === 1.获取 NFS 的信息
		//nfs.Server
		//nfs.Path

		// === 2.创建一个用来查找 pv 路径的 deployment,
		// deployment 需要挂载 /var/lib/kubelet 目录
		// deployment 需要和 operator 部署在同一个 namespace
		// deployment 配置 nodeName 和需要备份的 pod 在同一个 node 上.
		findpvpathBytes := []byte(fmt.Sprintf(findpvpathDeploymentTemplate,
			findpvpathName, operatorNamespace, updatedTimeAnnotation, time.Now().Format(time.RFC3339), nodeName, findpvpathImage))
		if _, err = deployHandler.Apply(findpvpathBytes); err != nil {
			return err
		}
		logrus.Debug("Wait findpvpath ready.")
		deployHandler.WaitReady(findpvpathName)

		//
		//
		// === 3.获取查找 pv 路径的 pod
		// 在这个 pod 中执行命令, 来获取需要备份的 pod 的 pv 路径.
		if findpvpathPods, err = deployHandler.GetPods(findpvpathName); err != nil {
			return err
		}
		// deployment 即使 ready 了, 获取到的 pod 列表中包含了正在删除状态的 pod, 要把它剔除掉
		for i := range findpvpathPods {
			findpvpathPod = findpvpathPods[i]
			if findpvpathPod.Status.Phase == corev1.PodRunning {
				logrus.Debug(findpvpathPod.Name)
				break
			}
		}
		// pvpath 用来存放命令行的输出, 这个输出中包含了需要备份的 pv 所在 k8s node 上的路径.
		if err := podHandler.ExecuteWithStream(
			findpvpathPod.Name, "", []string{"findpvpath", "--pod-uid", podUID, "--storage-type", "nfs"},
			os.Stdin, pvpath, io.Discard); err != nil {
			return err
		}
		logrus.Info(nodeName)
		logrus.Info(podUID)

		//
		//
		// === 4.创建一个用来备份数据的 deployment,
		// deployment 挂载需要备份的 pod 的 pv,
		// deployment 挂载 NFS 存储
		// 对 deployment 的 pod 执行命令:
		//   restic init 初始化 restic repository
		//   restic backup 将 pv 数据备份到 NFS 存储
		backupSource := strings.TrimSpace(pvpath.String())
		if len(backupSource) == 0 {
			logrus.Info("backup source is empty, skip backup")
			return nil
		}
		logrus.Infof("persistentvolume path: %s\n", backupSource)
		backuptonfsBytes := []byte(fmt.Sprintf(backuptonfsDeploymentTemplate,
			backuptonfsName, operatorNamespace,
			updatedTimeAnnotation, time.Now().Format(time.RFC3339),
			nodeName, backuptonfsImage,
			backupSource, backupSource,
			nfs.Server, nfs.Path))
		logrus.Info(string(backuptonfsBytes))
		if _, err := deployHandler.Apply(backuptonfsBytes); err != nil {
			return err
		}
		logrus.Debug("Wait backuptonfs ready.")
		deployHandler.WaitReady(backuptonfsName)
		if backuptonfsPods, err = deployHandler.GetPods(backuptonfsName); err != nil {
			return err
		}
		// deployment 即使 ready 了, 获取到的 pod 列表中包含了正在删除状态的 pod, 要把它剔除掉
		for i := range backuptonfsPods {
			backuptonfsPod = backuptonfsPods[i]
			if backuptonfsPod.Status.Phase == corev1.PodRunning {
				logrus.Debug(backuptonfsPod.Name)
				break
			}
		}

		// === 5.在 pod/backup-to-nfs 中执行 "restic init"
		if err = backupByRestic(ctx, operatorNamespace, backuptonfsPod.Name, podObj, podHandler, backupSource); err != nil {
			return err
		}

		logrus.Info("Successfully Backup to NFS Server")
	}
	return nil

}

func backupByRestic(ctx context.Context, operatorNamespace, execPod string, object runtime.Object, podHandler *pod.Handler, backupSource string) error {
	var (
		err       error
		cluster   string
		kind      string
		namespace string
		name      string
		tags      []string

		pvc        string
		pvNameList []string
	)
	pvHandler, err := persistentvolume.New(ctx, "")
	if err != nil {
		return err
	}
	res := restic.NewIgnoreNotFound(ctx, &restic.GlobalFlags{NoCache: true, Repo: resticRepo, Verbose: 3})

	logrus.Info(res.Command(restic.Init{}))
	//// 需要输入两遍密码, 一定需要输入两个 "\n", 否则 "restic init" 会一直卡在这里
	podHandler.ExecuteWithStream(execPod, "", strings.Split(res.String(), " "), createPassStdin(resticPasswd, 2), io.Discard, io.Discard)

	for _, pvName := range pvNameList {
		if pvc, err = pvHandler.GetPVC(pvName); err != nil {
			return err
		}
		tags = []string{kind, cluster, namespace, name, pvc}
		//logrus.Info(res.Command(restic.Backup{Tag: tags}).SetArgs(pv))
		logrus.Info(res.Command(restic.Backup{Tag: tags, Host: HostBackupToNFS}.SetArgs(backupSource)))
		podHandler.ExecuteWithStream(execPod, "", strings.Split(res.String(), " "), createPassStdin(resticPasswd), io.Discard, io.Discard)
	}

	return nil
}

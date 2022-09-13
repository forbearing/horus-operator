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
	"github.com/forbearing/k8s/deployment"
	"github.com/forbearing/k8s/pod"
	"github.com/forbearing/k8s/util/annotations"
	"github.com/forbearing/restic"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	resticBackupSource = "/backup-source"
	resticRepo         = "/restic-repo"
	resticPasswd       = "mypass"
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
func BackupToNFS(ctx context.Context, operatorNamespace string, podName string, nfs *storagev1alpha.NFS) error {
	logrus.SetLevel(logrus.DebugLevel)

	var (
		err           error
		deployHandler *deployment.Handler
		podHandler    *pod.Handler
		podObj        *corev1.Pod

		nodeName string
		podUID   string
		pvpath   = new(bytes.Buffer)

		findpvpathName  = "findpvpath"
		findpvpathImage = "hybfkuf/findpvpath:latest"
		findpvpathObj   = &appsv1.Deployment{}
		findpvpathPods  = []*corev1.Pod{}
		findpvpathPod   = &corev1.Pod{}

		backuptonfsName  = "backup-to-nfs"
		backuptonfsImage = "hybfkuf/backup-tools-restic:latest"
		backuptonfsObj   = &appsv1.Deployment{}
		backuptonfsPods  = []*corev1.Pod{}
		backuptonfsPod   = &corev1.Pod{}
	)

	// === 准备处理器
	if deployHandler, err = deployment.New(ctx, "", operatorNamespace); err != nil {
		return err
	}
	if podHandler, err = pod.New(ctx, "", operatorNamespace); err != nil {
		return err
	}
	if podObj, err = podHandler.Get(podName); err != nil {
		return err
	}
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
		findpvpathName, operatorNamespace, nodeName, findpvpathImage))
	if findpvpathObj, err = getOrApplyDeployment(deployHandler, findpvpathBytes); err != nil {
		return err
	}
	annotations.Set(findpvpathObj, fmt.Sprintf("%s=%s", updatedTimeAnnotation, time.Now().Format(time.RFC3339)))
	deployHandler.Apply(setPodTemplateAnnotations(findpvpathObj)) // 设置 annotation 的目的就是为了重启一下
	logrus.Debug("Wait findpvpath ready.")
	deployHandler.WaitReady(findpvpathName)

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
		os.Stdin, pvpath, pvpath); err != nil {
		return err
	}
	logrus.Infof("persistentvolume path: %s\n", pvpath)

	// === 4.创建一个用来备份数据的 deployment,
	// deployment 挂载需要备份的 pod 的 pv,
	// deployment 挂载 NFS 存储
	// 对 deployment 的 pod 执行命令:
	//   restic init 初始化 restic repository
	//   restic backup 将 pv 数据备份到 NFS 存储
	backupSource := strings.TrimSpace(pvpath.String())
	backuptonfsBytes := []byte(fmt.Sprintf(backuptonfsDeploymentTemplate,
		backuptonfsName, operatorNamespace, nodeName, backuptonfsImage, backupSource, nfs.Server, nfs.Path))
	if backuptonfsObj, err = getOrApplyDeployment(deployHandler, backuptonfsBytes); err != nil {
		return err
	}
	annotations.Set(backuptonfsObj, fmt.Sprintf("%s=%s", updatedTimeAnnotation, time.Now().Format(time.RFC3339)))
	deployHandler.Apply(setPodTemplateAnnotations(backuptonfsObj)) // 设置 annotations 的目的就是为了重启一下
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
	// 需要输入两遍密码, 一定需要输入两个 "\n", 否则 "restic init" 会一直卡在这里
	res := restic.NewIgnoreNotFound(ctx, &restic.GlobalFlags{NoCache: true, Repo: resticRepo, Verbose: 3})
	logrus.Info(res.Command(restic.Init{}))
	podHandler.ExecuteWithStream(backuptonfsPod.Name, "", strings.Split(res.String(), " "), createPassStdin(resticPasswd, 2), io.Discard, io.Discard)
	// === 6.在 pod/backup-to-nfs 中执行 "restic backup"
	logrus.Info(res.Command(restic.Backup{}.SetArgs(resticBackupSource)))
	podHandler.ExecuteWithStream(backuptonfsPod.Name, "", strings.Split(res.String(), " "), createPassStdin(resticPasswd), io.Discard, io.Discard)
	logrus.Info("Successfully Backup to NFS Server")
	return nil
}

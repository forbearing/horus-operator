package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
)

const (
	defaultClusterName = "kubernetes"

	resticBackupSource = "/backup-source"
	resticRepo         = "/restic-repo"
	resticPasswd       = "mypass"

	HostBackupToNFS   = "backup-to-nfs"
	HostBackupToS3    = "backup-to-S3"
	HostBackupToMinio = "backup-to-minio"
)

const ()

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
		pvNameList []string
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

	// podObj 为备份对象(比如 Deployment, StatefulSet, DaemonSet, Pod) 的一个或多个 Pod
	for _, podObj := range podObjList {
		// findpvpath 和 backuptonfs 这两个 deployment 都需要 nodeName
		if nodeName, err = podHandler.GetNodeName(podObj); err != nil {
			return err
		}
		// findpvpath 需要 podUID
		if podUID, err = podHandler.GetUID(podObj); err != nil {
			return err
		}
		if pvNameList, err = podHandler.GetPV(podObj); err != nil {
			return err
		}

		//
		// === 1.获取 NFS 的信息
		//nfs.Server
		//nfs.Path

		//
		//
		// === 2.创建 deployment/findpvpath, 用来查找 pod 挂载的 pv 在节点上的路径.
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
		// 找到 deployment/findpvpath  所有的 pod
		if findpvpathPods, err = deployHandler.GetPods(findpvpathName); err != nil {
			return err
		}
		// deployment 即使 ready 了, 获取到的 pod 列表中可能包含了正在删除状态的 pod,
		// 我们需要 Running 状态的 Pod 来执行命令.
		for i := range findpvpathPods {
			findpvpathPod = findpvpathPods[i]
			if findpvpathPod.Status.Phase == corev1.PodRunning {
				logrus.Debug(findpvpathPod.Name)
				break
			}
		}
		// 在这个 pod 中执行命令, 来查找需要备份的 pod 的 pv 挂载在 k8s 节点上的路径.
		// pvpath 用来存放命令行的输出, 这个输出中包含了需要备份的 pv 所在 k8s node 上的路径.
		stdout := new(bytes.Buffer)
		if err := podHandler.ExecuteWithStream(
			findpvpathPod.Name, "", []string{"findpvpath", "--pod-uid", podUID, "--storage-type", "nfs"},
			os.Stdin, stdout, io.Discard); err != nil {
			return err
		}
		logrus.Info(nodeName)
		logrus.Info(podUID)

		//
		//
		// === 4.创建 deployment/backup-to-nfs, 通过 restic 备份工具来备份实际的  pv 数据,
		// deployment 挂载需要备份的 pod 的 pv,
		// deployment 挂载 NFS 存储
		// 对 deployment 的 pod 执行命令:
		//   restic init 初始化 restic repository
		//   restic backup 将 pv 数据备份到 NFS 存储

		// pvpath 示例: /var/lib/kubelet/pods/00b224d7-e9c5-42d3-94ca-516a99274a66/volumes/kubernetes.io~nfs
		// pvpath 格式为: /var/lib/kubelet/pods + pod UID + volumes + pvc 类型
		// 该路径下包含了一个或多个目录, 每个目录的名字就是 pv 的名字. 例如:
		// /var/lib/kubelet/pods/787b3c5d-d11e-4d63-846f-6abd86683dbd/volumes/kubernetes.io~nfs/pvc-19ff22c4-e54f-4c13-8f1d-7a72e874ca08
		pvpath := strings.TrimSpace(stdout.String())
		if len(pvpath) == 0 {
			logrus.Info("backup source is empty, skip backup")
			return nil
		}
		logrus.Infof("persistentvolume path: %s\n", pvpath)
		backuptonfsBytes := []byte(fmt.Sprintf(backuptonfsDeploymentTemplate,
			backuptonfsName, operatorNamespace,
			updatedTimeAnnotation, time.Now().Format(time.RFC3339),
			nodeName, backuptonfsImage,
			pvpath, pvpath,
			nfs.Server, nfs.Path))
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
		if err = backupByRestic(ctx, operatorNamespace, backuptonfsPod.Name, podObj, podHandler, pvpath, pvNameList, HostBackupToNFS); err != nil {
			return err
		}

		logrus.Info("Successfully Backup to NFS Server")
	}
	return nil

}

// hostname 做为 restic backup --host 的参数值
func backupByRestic(ctx context.Context, operatorNamespace, execPod string, podObj *corev1.Pod, podHandler *pod.Handler, pvpath string, pvNameList []string, hostname string) error {
	pvHandler, err := persistentvolume.New(ctx, "")
	if err != nil {
		return err
	}
	res := restic.NewIgnoreNotFound(ctx, &restic.GlobalFlags{NoCache: true, Repo: resticRepo, Verbose: 3})

	logrus.Info(res.Command(restic.Init{}))
	//// 需要输入两遍密码, 一定需要输入两个 "\n", 否则 "restic init" 会一直卡在这里
	podHandler.ExecuteWithStream(execPod, "", strings.Split(res.String(), " "), createPassStdin(resticPasswd, 2), io.Discard, io.Discard)

	var tags []string
	var pvc string
	for _, pvName := range pvNameList {
		if pvc, err = pvHandler.GetPVC(pvName); err != nil {
			return err
		}
		tags = []string{pod.GVK().Kind, defaultClusterName, podObj.Namespace, podObj.Name, pvc}
		logrus.Info(res.Command(restic.Backup{Tag: tags, Host: hostname}.SetArgs(filepath.Join(pvpath, pvName))))
		podHandler.ExecuteWithStream(execPod, "", strings.Split(res.String(), " "), createPassStdin(resticPasswd), io.Discard, io.Discard)
	}

	return nil
}

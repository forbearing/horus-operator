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

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/k8s/daemonset"
	"github.com/forbearing/k8s/deployment"
	"github.com/forbearing/k8s/persistentvolumeclaim"
	"github.com/forbearing/k8s/pod"
	"github.com/forbearing/k8s/replicaset"
	"github.com/forbearing/k8s/statefulset"
	"github.com/forbearing/restic"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type pvdataMeta struct {
	podName  string
	podUID   string
	nodeName string
	pvdir    string
	pvname   string
}

const (
	defaultClusterName = "kubernetes"

	resticBackupSource = "/backup-source"
	resticRepo         = "/restic-repo"
	resticPasswd       = "mypass"

	HostBackupToNFS   = "backup-to-nfs"
	HostBackupToS3    = "backup-to-S3"
	HostBackupToMinio = "backup-to-minio"
)

var (
	createdTimeAnnotation   = "storage.hybfkuf.io/createdAt"
	updatedTimeAnnotation   = "storage.hybfkuf.io/updatedAt"
	restartedTimeAnnotation = "storage.hybfkuf.io/restartedAt"
)

// podObj: 是要备份的 pv 所挂载到到 pod
// nfs: 将数据备份到 NFS
func BackupToNFS(ctx context.Context, operatorNamespace, backupObjNamespace string,
	backupFrom *storagev1alpha1.BackupFrom, nfs *storagev1alpha1.NFS) error {

	logrus.SetLevel(logrus.DebugLevel)
	var (
		err           error
		podHandler    *pod.Handler
		deployHandler *deployment.Handler
		rsHandler     *replicaset.Handler
		stsHandler    *statefulset.Handler
		dsHandler     *daemonset.Handler
		pvcHandler    *persistentvolumeclaim.Handler

		podObjList []*corev1.Pod
		pvcList    []string
	)

	// === 准备处理器
	if podHandler, err = pod.New(ctx, "", operatorNamespace); err != nil {
		return fmt.Errorf("create pod handler error: %s", err.Error())
	}
	if deployHandler, err = deployment.New(ctx, "", operatorNamespace); err != nil {
		return fmt.Errorf("create deployment handler error: %s", err.Error())
	}
	if rsHandler, err = replicaset.New(ctx, "", operatorNamespace); err != nil {
		return fmt.Errorf("create replicaset handler error: %s", err.Error())
	}
	if stsHandler, err = statefulset.New(ctx, "", operatorNamespace); err != nil {
		return fmt.Errorf("create statefulset handler error: %s", err.Error())
	}
	if dsHandler, err = daemonset.New(ctx, "", operatorNamespace); err != nil {
		return fmt.Errorf("create daemonset handler error: %s", err.Error())
	}
	if pvcHandler, err = persistentvolumeclaim.New(ctx, "", backupObjNamespace); err != nil {
		return fmt.Errorf("create persistentvolumeclaim handler error: %s", err.Error())
	}

	switch backupFrom.Resource {
	case storagev1alpha1.PodResource:
		logrus.Infof(`Start to backup "pod/%s"`, backupFrom.Name)
		podObj, err := podHandler.Get(backupFrom.Name)
		if err != nil {
			return fmt.Errorf("pod handler get pod error: %s", err.Error())
		}
		podObjList = append(podObjList, podObj)
		if pvcList, err = podHandler.GetPVC(backupFrom.Name); err != nil {
			return fmt.Errorf("pod handler get persistentvolumeclaim error: %s", err.Error())
		}
	case storagev1alpha1.DeploymentResource:
		logrus.Infof(`Start to backup "deployment/%s"`, backupFrom.Name)
		if podObjList, err = deployHandler.GetPods(backupFrom.Name); err != nil {
			return fmt.Errorf("deployment handler get pod error: %s", err.Error())
		}
		if pvcList, err = deployHandler.GetPVC(backupFrom.Name); err != nil {
			return fmt.Errorf("deployment handler get persistentvolumeclaim error: %s", err.Error())
		}
	case storagev1alpha1.StatefulSetResource:
		logrus.Infof(`Start to backup "statefulset/%s"`, backupFrom.Name)
		if podObjList, err = stsHandler.GetPods(backupFrom.Name); err != nil {
			return fmt.Errorf("statefulset handler get pod error: %s", err.Error())
		}
		if pvcList, err = stsHandler.GetPVC(backupFrom.Name); err != nil {
			return fmt.Errorf("statefulset handler get persistentvolumeclaim error: %s", err.Error())
		}
	case storagev1alpha1.DaemonSetResource:
		logrus.Infof(`Start to backup "daemonset/%s"`, backupFrom.Name)
		if podObjList, err = dsHandler.GetPods(backupFrom.Name); err != nil {
			return fmt.Errorf("daemonset handler get pod error: %s", err.Error())
		}
		if pvcList, err = dsHandler.GetPVC(backupFrom.Name); err != nil {
			return fmt.Errorf("daemonset handler get persistentvolumeclaim error: %s", err.Error())
		}
	default:
		return errors.New("Not Support backup object")
	}

	// pvcpvMap 存在的意义: 不要重复备份同一个 pvc
	// 因为有些 pvc  为 ReadWriteMany 模式, 当一个 deployment 下的多个 pod 同时
	// 挂载了同一个 pvc, 默认会对这个 pvc 备份多次, 这完全没必要, 只需要备份一次即可
	// pvc name 作为 key, pvdataMeta 作为 value
	// 在这里只设置了 pv name
	pvcpvMap := make(map[string]pvdataMeta)
	for _, pvc := range pvcList {
		pvname, err := pvcHandler.GetPV(pvc)
		if err != nil {
			return fmt.Errorf("persistentvolumeclaim handler get persistentvolume error: %s", err.Error())
		}
		pvcpvMap[pvc] = pvdataMeta{pvname: pvname}
	}

	for pvc, meta := range pvcpvMap {
		logrus.Debugf("%v: %v", pvc, meta)
	}

	// podObj 为备份对象(比如 Deployment, StatefulSet, DaemonSet, Pod) 的一个或多个 Pod
	for _, podObj := range podObjList {
		var nodeName, podUID string
		if nodeName, err = podHandler.GetNodeName(podObj); err != nil {
			return err
		}
		if podUID, err = podHandler.GetUID(podObj); err != nil {
			return err
		}

		// === 1.获取 NFS 的信息
		//nfs.Server
		//nfs.Path

		//
		// === 2.创建 deployment/findpvdir, 用来查找 pod 挂载的 pv 在节点上的路径.
		// deployment 需要挂载 /var/lib/kubelet 目录
		// deployment 需要和 operator 部署在同一个 namespace
		// deployment 配置 nodeName 和需要备份的 pod 在同一个 node 上.
		var pvdir string
		if pvdir, err = createFindpvdirDeployment(podHandler, deployHandler, rsHandler,
			operatorNamespace, nodeName, podUID); err != nil {
			return fmt.Errorf("create deployment/findpvdir error: %s", err.Error())
		}
		if len(pvdir) == 0 {
			logrus.Warn("Backup source is empty, skip backup")
			continue
		}
		//
		// === 3.
		pvcList, err := podHandler.GetPVC(podObj)
		if err != nil {
			return fmt.Errorf("pod handler get persistentvolumeclaim faile: %s", err.Error())
		}
		logrus.Debugf(`The persistentvolumeclaim mounted by "pod/%s" are: %v`, podObj.Name, pvcList)

		for _, pvc := range pvcList {
			if _, ok := pvcpvMap[pvc]; !ok {
				logrus.Warnf(`"persistentvolumeclaim/%s" not found`, pvc)
				continue
			}
			pvname := pvcpvMap[pvc].pvname
			pvcpvMap[pvc] = pvdataMeta{
				podName:  podObj.Name,
				podUID:   podUID,
				nodeName: nodeName,
				pvdir:    pvdir,
				pvname:   pvname,
			}
		}
	}
	for pvc, meta := range pvcpvMap {
		logrus.Debugf("%v: %v", pvc, meta)
	}

	//select {}

	for pvc, meta := range pvcpvMap {
		// === 3.创建 deployment/backup-to-nfs, 通过 restic 备份工具来备份实际的  pv 数据,
		// deployment 挂载需要备份的 pod 的 pv,
		// deployment 挂载 NFS 存储
		// 对 deployment 的 pod 执行命令:
		//   restic init 初始化 restic repository
		//   restic backup 将 pv 数据备份到 NFS 存储
		var backuptonfsPod string
		if backuptonfsPod, err = createBackuptonfsDeployment(podHandler, deployHandler, rsHandler,
			nfs, operatorNamespace, meta.pvdir, meta.nodeName); err != nil {
			return err
		}

		// === 5.通过 restic 备份工具开始备份
		time.Sleep(time.Second * 3)
		if err = backupByRestic(ctx, operatorNamespace, backupObjNamespace, podHandler,
			backupFrom, backuptonfsPod, pvc, meta, HostBackupToNFS); err != nil {
			return err
		}
	}
	logrus.Info("Successfully Backup to NFS Server")
	return nil

}

// resticHost 作为 restic backup --host 的参数值
// pvpath + pv 就是实际的 pv 数据的存放路径
func backupByRestic(ctx context.Context, operatorNamespace, backupObjNamespace string,
	podHandler *pod.Handler, backupFrom *storagev1alpha1.BackupFrom,
	execPod string, pvc string, meta pvdataMeta, resticHost string) error {

	res := restic.NewIgnoreNotFound(ctx, &restic.GlobalFlags{NoCache: true, Repo: resticRepo, Verbose: 3})
	logrus.Debug(res.Command(restic.Init{}))
	// 需要输入两遍密码, 一定需要输入两个 "\n", 否则 "restic init" 会一直卡在这里
	podHandler.ExecuteWithStream(execPod, "", strings.Split(res.String(), " "), createPassStdin(resticPasswd, 2), io.Discard, io.Discard)

	tags := []string{string(backupFrom.Resource), defaultClusterName, backupObjNamespace, backupFrom.Name, pvc}
	logrus.Debug(res.Command(restic.Backup{Tag: tags, Host: resticHost}.SetArgs(filepath.Join(meta.pvdir, meta.pvname))))
	if err := podHandler.ExecuteWithStream(execPod, "", strings.Split(res.String(), " "), createPassStdin(resticPasswd), io.Discard, io.Discard); err != nil {
		logrus.Errorf(`backup %s failed, maybe the directories/files do not exist in k8s node`, filepath.Join(meta.pvdir, meta.pvname))
	}

	return nil
}

// createFindpvdirDeployment
func createFindpvdirDeployment(
	podHandler *pod.Handler,
	deployHandler *deployment.Handler,
	rsHandler *replicaset.Handler,
	operatorNamespace, nodeName, podUID string) (string, error) {

	var (
		err             error
		findpvdirName   = "findpvdir"
		findpvdirImage  = "hybfkuf/findpvdir:latest"
		findpvdirObj    = &appsv1.Deployment{}
		findpvdirRsList = []*appsv1.ReplicaSet{}
		findpvdirRS     = &appsv1.ReplicaSet{}
		findpvdirPods   = []*corev1.Pod{}
		findpvdirPod    = &corev1.Pod{}
	)

	findpvdirBytes := []byte(fmt.Sprintf(findpvdirDeploymentTemplate,
		findpvdirName, operatorNamespace,
		updatedTimeAnnotation, time.Now().Format(time.RFC3339),
		nodeName, findpvdirImage))
	if findpvdirObj, err = deployHandler.Apply(findpvdirBytes); err != nil {
		return "", err
	}
	logrus.Debug("Waiting deployment/findpvdir ready.")
	deployHandler.WaitReady(findpvdirName)

	// 使用 findpvdirObj 作为参数传入而不是使用 findpvdirName 作为参数传入,
	// 虽然 GetRS() 可以通过一个 deployment 名字或者 deployment 对象来找到
	// 其管理的 ReplicaSet, 但是如果传入的是 deployment 的名字, GetRS() 需额外
	// 通过 Get API Server 接口找到 Deployment 对象. 具体请看 GetRS() 的源码.
	if findpvdirRsList, err = deployHandler.GetRS(findpvdirObj); err != nil {
		return "", err
	}
	// 只有 ReplicaSet 的 replicas 的值不为 nil 且大于0, 则表明该 ReplicaSet
	// 就是当前 Deployment 正在使用的 ReplicaSet.
	for i := range findpvdirRsList {
		findpvdirRS = findpvdirRsList[i]
		if findpvdirRS.Spec.Replicas != nil && *findpvdirRS.Spec.Replicas > 0 {
			// 优先使用 replicaset.Handler, 而不是 deployment.Handler
			// 因为 deployment.Handler 还额外调用 list API 接口来获取其管理
			// 的所有 replicaset 对象.
			// 我们已经找到了我们需要的 replicaset, 直接用过 replicaset.Handler
			// 来找到该 replicaset 下管理的所有 Pod 即可
			if findpvdirPods, err = rsHandler.GetPods(findpvdirRS); err != nil {
				return "", err
			}
			break
		}
	}
	// 同时要确保该 Pod 是 "Running" 状态
	for i := range findpvdirPods {
		findpvdirPod = findpvdirPods[i]
		if findpvdirPod.Status.Phase == corev1.PodRunning {
			logrus.Debugf(`We will find persistentvolume data directory path by execute command within this pod: "%s"`, findpvdirPod.Name)
			break
		}
	}

	// 在这个 pod 中执行命令, 来查找需要备份的 pod 的 pv 挂载在 k8s 节点上的路径.
	// pvpath 用来存放命令行的输出, 这个输出中包含了需要备份的 pv 所在 k8s node 上的路径.
	stdout := new(bytes.Buffer)
	if err := podHandler.ExecuteWithStream(
		findpvdirPod.Name, "", []string{"findpvdir", "--pod-uid", podUID, "--storage-type", "nfs"},
		os.Stdin, stdout, io.Discard); err != nil {
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}

// createBackuptonfsDeployment
func createBackuptonfsDeployment(
	podHandler *pod.Handler,
	deployHandler *deployment.Handler,
	rsHandler *replicaset.Handler,
	nfs *storagev1alpha1.NFS,
	operatorNamespace, pvdir, nodeName string) (string, error) {

	var (
		err               error
		backuptonfsName   = "backup-to-nfs"
		backuptonfsImage  = "hybfkuf/backup-tools-restic:latest"
		backuptonfsObj    = &appsv1.Deployment{}
		backuptonfsRsList = []*appsv1.ReplicaSet{}
		backuptonfsRS     = &appsv1.ReplicaSet{}
		backuptonfsPods   = []*corev1.Pod{}
		backuptonfsPod    = &corev1.Pod{}
	)

	// pvpath 示例: /var/lib/kubelet/pods/00b224d7-e9c5-42d3-94ca-516a99274a66/volumes/kubernetes.io~nfs
	// pvpath 格式为: /var/lib/kubelet/pods + pod UID + volumes + pvc 类型
	// 该路径下包含了一个或多个目录, 每个目录的名字就是 pv 的名字. 例如:
	// /var/lib/kubelet/pods/787b3c5d-d11e-4d63-846f-6abd86683dbd/volumes/kubernetes.io~nfs/pvc-19ff22c4-e54f-4c13-8f1d-7a72e874ca08
	backuptonfsBytes := []byte(fmt.Sprintf(backuptonfsDeploymentTemplate,
		backuptonfsName, operatorNamespace,
		updatedTimeAnnotation, time.Now().Format(time.RFC3339),
		nodeName, backuptonfsImage,
		pvdir, pvdir,
		nfs.Server, nfs.Path))
	if backuptonfsObj, err = deployHandler.Apply(backuptonfsBytes); err != nil {
		return "", err
	}
	logrus.Debug("Waiting deployment/backuptonfs ready.")
	deployHandler.WaitReady(backuptonfsName)

	// 先找到 backuptonfs 这个 Deployment 下所有管理的 ReplicaSet
	// 使用 backuptonfsObj 而不是 backuptonfsName, 因为前者比后者少一个 List API 请求
	if backuptonfsRsList, err = deployHandler.GetRS(backuptonfsObj); err != nil {
		return "", err
	}
	for i := range backuptonfsRsList {
		backuptonfsRS = backuptonfsRsList[i]
		if backuptonfsRS.Spec.Replicas != nil && *backuptonfsRS.Spec.Replicas > 0 {
			if backuptonfsPods, err = rsHandler.GetPods(backuptonfsRS); err != nil {
				return "", err
			}
			break
		}
	}
	// 确保该 Pod 是 "Running" 状态
	for i := range backuptonfsPods {
		backuptonfsPod = backuptonfsPods[i]
		if backuptonfsPod.Status.Phase == corev1.PodRunning {
			logrus.Debugf(`We will execute restic command to backup persistentvolume data within this pod: "%s"`, backuptonfsPod.Name)
			break
		}

	}
	return backuptonfsPod.Name, nil
}

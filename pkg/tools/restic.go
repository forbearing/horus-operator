package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/k8s/daemonset"
	"github.com/forbearing/k8s/deployment"
	"github.com/forbearing/k8s/persistentvolume"
	"github.com/forbearing/k8s/persistentvolumeclaim"
	"github.com/forbearing/k8s/pod"
	"github.com/forbearing/k8s/replicaset"
	"github.com/forbearing/k8s/statefulset"
	"github.com/forbearing/restic"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type pvdataMeta struct {
	volumeSource string
	nodeName     string
	podName      string
	podUID       string
	pvdir        string
	pvname       string
}

const (
	defaultClusterName = "kubernetes"

	resticBackupSource = "/backup-source"
	resticRepo         = "/restic-repo"
	resticPasswd       = "mypass"
	mountHostRootPath  = "/host-root"

	HostBackupToNFS   = "backup-to-nfs"
	HostBackupToS3    = "backup-to-s3"
	HostBackupToMinio = "backup-to-minio"
)

var (
	createdTimeAnnotation   = "storage.hybfkuf.io/createdAt"
	updatedTimeAnnotation   = "storage.hybfkuf.io/updatedAt"
	restartedTimeAnnotation = "storage.hybfkuf.io/restartedAt"
)

var (
	ctx        = context.TODO()
	podHandler *pod.Handler
	depHandler *deployment.Handler
	rsHandler  *replicaset.Handler
	stsHandler *statefulset.Handler
	dsHandler  *daemonset.Handler
	pvHandler  *persistentvolume.Handler
	pvcHandler *persistentvolumeclaim.Handler
)

func init() {
	podHandler = pod.NewOrDie(ctx, "", "")
	depHandler = deployment.NewOrDie(ctx, "", "")
	rsHandler = replicaset.NewOrDie(ctx, "", "")
	stsHandler = statefulset.NewOrDie(ctx, "", "")
	dsHandler = daemonset.NewOrDie(ctx, "", "")
	pvHandler = persistentvolume.NewOrDie(ctx, "")
	pvcHandler = persistentvolumeclaim.NewOrDie(ctx, "", "")
}

// podObj: 是要备份的 pv 所挂载到到 pod
// nfs: 将数据备份到 NFS
func BackupToNFS(ctx context.Context, operatorNamespace string,
	backupObj *storagev1alpha1.Backup, nfs *storagev1alpha1.NFS) error {
	var (
		err        error
		podObjList []*corev1.Pod

		backupFrom = backupObj.Spec.BackupFrom
		namespace  = backupObj.GetNamespace()
	)

	beginTime := time.Now()
	logger := logrus.WithFields(logrus.Fields{
		"Component": "BackupToNFS",
		"Storage":   "NFS",
		"Resource":  backupFrom.Resource,
		"Namespace": namespace,
		"Name":      backupFrom.Name,
	})

	podHandler.ResetNamespace(backupObj.GetNamespace())
	depHandler.ResetNamespace(backupObj.GetNamespace())
	rsHandler.ResetNamespace(backupObj.GetNamespace())
	stsHandler.ResetNamespace(backupObj.GetNamespace())
	dsHandler.ResetNamespace(backupObj.GetNamespace())
	pvcHandler.ResetNamespace(backupObj.GetNamespace())

	switch backupFrom.Resource {
	case storagev1alpha1.PodResource:
		logger.Infof("Start Backup pod/%s", backupFrom.Name)
		podObj, err := podHandler.Get(backupFrom.Name)
		if err != nil {
			// if the Pod resource not found, skip backup
			if apierrors.IsNotFound(err) {
				logger.Warnf("pod/%s not found in namespace %s, skip backup", backupFrom.Name, namespace)
				return nil
			}
			return fmt.Errorf("pod handler get pod error: %s", err.Error())
		}
		podObjList = append(podObjList, podObj)
	case storagev1alpha1.DeploymentResource:
		logger.Infof("Start Backup deployment/%s", backupFrom.Name)
		if podObjList, err = depHandler.GetPods(backupFrom.Name); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Warnf("deployment/%s not found in namespace %s, skip backup", backupFrom.Name, namespace)
				return nil
			}
			return fmt.Errorf("deployment handler get pod error: %s", err.Error())
		}
	case storagev1alpha1.StatefulSetResource:
		logger.Infof("Start Backup statefulset/%s", backupFrom.Name)
		if podObjList, err = stsHandler.GetPods(backupFrom.Name); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Warnf("statefulset/%s not found in namespace %s, skip backup", backupFrom.Name, namespace)
				return nil
			}
			return fmt.Errorf("statefulset handler get pod error: %s", err.Error())
		}
	case storagev1alpha1.DaemonSetResource:
		logger.Infof("Start Backup daemonset/%s", backupFrom.Name)
		if podObjList, err = dsHandler.GetPods(backupFrom.Name); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Warnf("daemonset/%s not found in namespace %s, skip backup", backupFrom.Name, namespace)
				return nil
			}
			return fmt.Errorf("daemonset handler get pod error: %s", err.Error())
		}
	default:
		logger.Error("Not support backup object(must be pod|deployment|statefulset|daemonset)")
		return nil
	}

	// pvcpvMap 存在的意义: 不要重复备份同一个 pvc
	// 因为有些 pvc  为 ReadWriteMany 模式, 当一个 deployment 下的多个 pod 同时
	// 挂载了同一个 pvc, 默认会对这个 pvc 备份多次, 这完全没必要, 只需要备份一次即可
	// pvc name 作为 key, pvdataMeta 作为 value
	// 在这里只设置了 pv name
	//
	// nodeName, podName, podUID
	// pvc -> pvname -> volumeSource
	// pvdir
	pvcpvMap := make(map[string]pvdataMeta)
	// podObj 为备份对象(比如 Deployment, StatefulSet, DaemonSet, Pod) 的一个或多个 Pod
	for _, podObj := range podObjList {
		// 1. 设置 pvcpvMap 对象: nodeName, podName, podUID
		meta := pvdataMeta{}
		var nodeName, podUID string
		if nodeName, err = podHandler.GetNodeName(podObj); err != nil {
			return err
		}
		if podUID, err = podHandler.GetUID(podObj); err != nil {
			return err
		}
		meta.nodeName = nodeName
		meta.podName = podObj.GetName()
		meta.podUID = podUID

		// === 2. 设置 pvcpvMap 对象: pvname, volumeSource
		// 获取 pod 所有的 pvc, 将 pvc 作为 key 依次遍历 pvcpvMap
		// 一般在 pod 中获取到的 pvc 都会在 pvcpvMap 中存在
		// pvcpvMap 是从 deployment/statefulset/daemonset 中获得的.
		pvcList, err := podHandler.GetPVC(podObj)
		if err != nil {
			return fmt.Errorf("pod handler get persistentvolumeclaim faile: %s", err.Error())
		}
		logger.Debugf("The persistentvolumeclaims mounted by pod/%s are: %v", podObj.Name, pvcList)
		for _, pvc := range pvcList {
			pvname, err := pvcHandler.GetPV(pvc)
			if err != nil {
				logger.Errorf("persistentvolumeclaim get pv error: %s", err.Error())
				continue
			}
			volumeSource, err := pvHandler.GetVolumeSource(pvname)
			if err != nil {
				logger.Errorf("persistentvolume handler get volume source error: %s", err.Error())
				continue
			}
			meta.pvname = pvname
			meta.volumeSource = volumeSource
			pvcpvMap[pvc] = meta
		}

		// === 3.创建 deployment/findpvdir, 用来查找 pod 挂载的 pv 在节点上的路径.
		// deployment 需要挂载 /var/lib/kubelet 目录
		// deployment 需要和 operator 部署在同一个 namespace
		// deployment 配置 nodeName 和需要备份的 pod 在同一个 node 上.
		var costedTime time.Duration
		var pvdir string
		// 一个 pod 如果挂载了多个 pvc, 这多个 pvc 类型不能保证一项, 一些使用 ceph 存储,
		// 一些使用 hostPath, 所以需要遍历每一个 pvc.
		for _, pvc := range pvcList {
			meta := pvcpvMap[pvc]
			if pvdir, costedTime, err = createFindpvdirDeployment(operatorNamespace, backupObj, pvcpvMap[pvc]); err != nil {
				return fmt.Errorf("create deployment/findpvdir-%s error: %s", pvcpvMap[pvc].nodeName, err.Error())
			}
			logger.WithField("Cost", costedTime.String()).Infof("Found pvc/%s data directory path of pod/%s", pvc, podObj.GetName())
			if len(pvdir) == 0 {
				logger.WithField("VolumeSource", meta.volumeSource).Warnf("PVC/%s data directory not found", pvc)
				continue
			}
			logger.Debugf("The persistentvolume dir: %s", pvdir)
			var exist bool
			meta, exist = pvcpvMap[pvc]
			if !exist {
				logger.Warnf("The pvc/%s not found in pvcpvMap", pvc)
				continue
			}
			meta.pvdir = pvdir
			pvcpvMap[pvc] = meta
		}
	}
	// if there is no persistentvolumeclaim mounted by the backup target resource, skip backup
	if len(pvcpvMap) == 0 {
		logger.Warnf("There is no pvc mounted by the %s/%s, skip backup", backupFrom.Resource, backupFrom.Name)
		return nil
	}
	// output pvcpvMap for debug
	for pvc, meta := range pvcpvMap {
		logger.Debugf("%v: %v", pvc, meta)
	}

	// 备份每一个 pvc
	for pvc, meta := range pvcpvMap {
		// === 3.创建 deployment/backup-to-nfs
		// deployment 挂载需要备份的 pod 的 pv
		// deployment 挂载 NFS 存储
		// 对 deployment 的 pod 执行命令:
		//   restic init 初始化 restic repository
		//   restic backup 将 pv 数据备份到 NFS 存储
		var backuptonfsPod string
		var costedTime time.Duration
		if backuptonfsPod, costedTime, err = createBackuptonfsDeployment(operatorNamespace, backupObj, nfs, meta); err != nil {
			return err
		}
		logger.WithField("Cost", costedTime.String()).Infof("Create deployment/%s", "backuptonfs"+"-"+meta.nodeName)
		// === 4.通过 restic 备份工具在 backup-to-nfs 的 pod 上备份 pvc/pv 数据
		if costedTime, err = backupByRestic(ctx, operatorNamespace, backuptonfsPod, backupObj, pvc, meta, HostBackupToNFS); err != nil {
			return err
		}
		logger.WithField("Cost", costedTime.String()).Infof("Backup pvc/%s mounted by pod/%s", pvc, meta.podName)
	}

	logger.WithField("Cost", time.Now().Sub(beginTime).String()).
		Infof("Successfully Backup The PV/PVC Mounted by %s/%s to NFS Server", backupFrom.Resource, backupFrom.Name)
	return nil
}

// createFindpvdirDeployment
func createFindpvdirDeployment(operatorNamespace string, backupObj *storagev1alpha1.Backup, meta pvdataMeta) (string, time.Duration, error) {
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

	beginTime := time.Now()
	podHandler.ResetNamespace(operatorNamespace)
	depHandler.ResetNamespace(operatorNamespace)
	rsHandler.ResetNamespace(operatorNamespace)
	logger := logrus.WithFields(logrus.Fields{
		"Component": findpvdirName,
	})

	deployName := findpvdirName + "-" + meta.nodeName
	findpvdirBytes := []byte(fmt.Sprintf(findpvdirDeploymentTemplate,
		deployName, operatorNamespace,
		updatedTimeAnnotation, time.Now().Format(time.RFC3339),
		meta.nodeName, findpvdirImage, backupObj.Spec.TimeZone))
	if findpvdirObj, err = depHandler.Apply(findpvdirBytes); err != nil {
		return "", time.Now().Sub(beginTime), fmt.Errorf("deployment handler apply deployment/%s failed: %s", findpvdirName, err.Error())
	}
	logger.Debugf("waiting deployment/%s to be available and ready.", deployName)
	if err := depHandler.WaitReady(deployName); err != nil {
		logger.Errorf("createFindpvdirDeployment WaitReady error: %s", err.Error())
	}

	// 使用 findpvdirObj 作为参数传入而不是使用 findpvdirName 作为参数传入,
	// 虽然 GetRS() 可以通过一个 deployment 名字或者 deployment 对象来找到
	// 其管理的 ReplicaSet, 但是如果传入的是 deployment 的名字, GetRS() 需额外
	// 通过 Get API Server 接口找到 Deployment 对象. 具体请看 GetRS() 的源码.
	if findpvdirRsList, err = depHandler.GetRS(findpvdirObj); err != nil {
		return "", time.Now().Sub(beginTime), fmt.Errorf("deployment handler get replicasets failed: %s", err.Error())
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
				return "", time.Now().Sub(beginTime), fmt.Errorf("replicaset handler get pods failed: %s", err.Error())
			}
			break
		}
	}
	// 同时要确保该 Pod 是 "Running" 状态
	for i := range findpvdirPods {
		findpvdirPod = findpvdirPods[i]
		if findpvdirPod.Status.Phase == corev1.PodRunning {
			logger.Debugf("finding persistentvolume data directory path by execute command within pod/%s", findpvdirPod.Name)
			break
		}
	}

	cmdFindpvdir := []string{"findpvdir", "--pod-uid", meta.podUID, "--storage-type", meta.volumeSource}
	switch meta.volumeSource {
	case "hostPath":
		// if persistentvolume volume source, we return value is pvpath(pvpath = pvdir + pvname), not pvdir
		pvObj, err := pvHandler.Get(meta.pvname)
		if err != nil {
			fmt.Errorf("persistentvolume handler get persistentvolume error: %s", err.Error())
		}
		return pvObj.Spec.HostPath.Path, time.Now().Sub(beginTime), nil
	}
	logger.Debugf("executing command %v to find persistentvolume data in node %s", cmdFindpvdir, meta.nodeName)
	//cmdFindpvdir := []string{"findpvdir", "--pod-uid", podUID, "--storage-type", "csi"}
	// 在这个 pod 中执行命令, 来查找需要备份的 pod 的 pv 挂载在 k8s 节点上的路径.
	// pvpath 用来存放命令行的输出, 这个输出中包含了需要备份的 pv 所在 k8s node 上的路径.
	stdout := new(bytes.Buffer)
	if err := podHandler.ExecuteWithStream(findpvdirPod.Name, "", cmdFindpvdir, os.Stdin, stdout, io.Discard); err != nil {
		return "", time.Now().Sub(beginTime), fmt.Errorf("%s find the persistentvolume data directory failed: %s", findpvdirName, err.Error())
	}

	return strings.TrimSpace(stdout.String()), time.Now().Sub(beginTime), nil
}

// createBackuptonfsDeployment
func createBackuptonfsDeployment(operatorNamespace string, backupObj *storagev1alpha1.Backup, nfs *storagev1alpha1.NFS, meta pvdataMeta) (string, time.Duration, error) {
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

	beginTime := time.Now()
	podHandler.ResetNamespace(operatorNamespace)
	depHandler.ResetNamespace(operatorNamespace)
	rsHandler.ResetNamespace(operatorNamespace)
	logger := logrus.WithFields(logrus.Fields{
		"Component": "backup",
	})

	// pvpath 示例: /var/lib/kubelet/pods/00b224d7-e9c5-42d3-94ca-516a99274a66/volumes/kubernetes.io~nfs
	// pvpath 格式为: /var/lib/kubelet/pods + pod UID + volumes + pvc 类型
	// 该路径下包含了一个或多个目录, 每个目录的名字就是 pv 的名字. 例如:
	// /var/lib/kubelet/pods/787b3c5d-d11e-4d63-846f-6abd86683dbd/volumes/kubernetes.io~nfs/pvc-19ff22c4-e54f-4c13-8f1d-7a72e874ca08
	deployName := backuptonfsName + "-" + meta.nodeName
	backuptonfsBytes := []byte(fmt.Sprintf(backuptonfsDeploymentTemplate,
		deployName, operatorNamespace,
		updatedTimeAnnotation, time.Now().Format(time.RFC3339),
		meta.nodeName, backuptonfsImage, backupObj.Spec.TimeZone,
		//pvdir, pvdir,
		nfs.Server, nfs.Path))
	if backuptonfsObj, err = depHandler.Apply(backuptonfsBytes); err != nil {
		return "", time.Now().Sub(beginTime), err
	}
	logger.Debugf("waiting deployment/%s to be available and ready.", deployName)
	if err := depHandler.WaitReady(deployName); err != nil {
		logger.Errorf("createBackuptonfsDeployment WaitReady error: %s", err.Error())
	}

	// 先找到 backuptonfs 这个 Deployment 下所有管理的 ReplicaSet
	// 使用 backuptonfsObj 而不是 backuptonfsName, 因为前者比后者少一个 List API 请求
	if backuptonfsRsList, err = depHandler.GetRS(backuptonfsObj); err != nil {
		return "", time.Now().Sub(beginTime), fmt.Errorf("deployment handler get replicasets error: %s", err.Error())
	}
	for i := range backuptonfsRsList {
		backuptonfsRS = backuptonfsRsList[i]
		if backuptonfsRS.Spec.Replicas != nil && *backuptonfsRS.Spec.Replicas > 0 {
			if backuptonfsPods, err = rsHandler.GetPods(backuptonfsRS); err != nil {
				return "", time.Now().Sub(beginTime), fmt.Errorf("replicaset handler get pods error: %s", err.Error())
			}
			break
		}
	}
	// 确保该 Pod 是 "Running" 状态
	for i := range backuptonfsPods {
		backuptonfsPod = backuptonfsPods[i]
		if backuptonfsPod.Status.Phase == corev1.PodRunning {
			break
		}

	}
	return backuptonfsPod.Name, time.Now().Sub(beginTime), nil
}

// ArgHost 作为 restic backup --host 的参数值
// pvdir + pv 就是实际的 pv 数据的存放路径
func backupByRestic(ctx context.Context, operatorNamespace string, execPod string,
	backupObj *storagev1alpha1.Backup, pvc string, meta pvdataMeta, ArgHost string) (time.Duration, error) {

	beginTime := time.Now()
	podHandler.ResetNamespace(operatorNamespace)
	logger := logrus.WithFields(logrus.Fields{
		"Component": "restic",
		"Node":      meta.nodeName,
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
		clusterName = defaultClusterName
	}
	pvpath := filepath.Join(mountHostRootPath, meta.pvdir, meta.pvname)
	switch meta.volumeSource {
	case "hostPath":
		// if persistentvolume volume source is hostPath, the pvdir is pvpath
		pvpath = filepath.Join(mountHostRootPath, meta.pvdir)
	}
	logger.Debugf("the path of persistentvolume data in k8s node: %s", pvpath)
	logger.Debugf("executing restic command to backup persistentvolume data within pod/%s", execPod)
	res := restic.NewIgnoreNotFound(ctx, &restic.GlobalFlags{NoCache: true, Repo: resticRepo})
	tags := []string{string(backupObj.Spec.BackupFrom.Resource), clusterName, backupObj.Namespace, backupObj.Spec.BackupFrom.Name, pvc}
	CmdCheckRepo := res.Command(restic.List{}.SetArgs("keys")).String()
	CmdInitRepo := res.Command(restic.Init{}).String()
	CmdBackup := res.Command(restic.Backup{Tag: tags, Host: ArgHost}.SetArgs(pvpath)).String()

	logger.Debug(CmdCheckRepo)
	// 如果 restic list keys 失败, 说明 restic repository 不存在,则需要创建一下
	if err := podHandler.ExecuteWithStream(execPod, "", strings.Split(CmdCheckRepo, " "),
		createPassStdin(resticPasswd, 1), io.Discard, io.Discard); err != nil {
		// 需要输入两遍密码, 一定需要输入两个 "\n", 否则 "restic init" 会一直卡在这里
		// 如果 restic list keys 失败, 说明 restic repository 不存在,则需要创建一下
		logger.Debug(CmdInitRepo)
		if err := podHandler.ExecuteWithStream(execPod, "", strings.Split(CmdInitRepo, " "),
			createPassStdin(resticPasswd, 2), io.Discard, io.Discard); err != nil {
			logger.Error("restic init failed")
			return time.Now().Sub(beginTime), nil
		}
	}
	logger.Debug(CmdBackup)
	if err := podHandler.WithNamespace(operatorNamespace).ExecuteWithStream(execPod, "", strings.Split(CmdBackup, " "),
		createPassStdin(resticPasswd), io.Discard, io.Discard); err != nil {
		logger.Errorf("restic backup pvc/%s failed, maybe the directory/file of %s do not exist in k8s node", pvc, pvpath)
	}

	return time.Now().Sub(beginTime), nil
}

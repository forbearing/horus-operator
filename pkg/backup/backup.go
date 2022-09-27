package backup

import (
	"context"
	"fmt"
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/forbearing/k8s/daemonset"
	"github.com/forbearing/k8s/deployment"
	"github.com/forbearing/k8s/dynamic"
	"github.com/forbearing/k8s/persistentvolume"
	"github.com/forbearing/k8s/persistentvolumeclaim"
	"github.com/forbearing/k8s/pod"
	"github.com/forbearing/k8s/replicaset"
	"github.com/forbearing/k8s/secret"
	"github.com/forbearing/k8s/statefulset"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	resticRepo        = "/restic-repo"
	resticPasswd      = "mypass"
	mountHostRootPath = "/host-root"

	HostBackupToNFS   = "backup-to-nfs"
	HostBackupToS3    = "backup-to-s3"
	HostBackupToMinio = "backup-to-minio"

	findpvdirName        = "findpvdir"
	findpvdirImage       = "hybfkuf/findpvdir:latest"
	backup2nfsName       = "backup-to-nfs"
	backup2nfsImage      = "hybfkuf/backup-tools-restic:latest"
	backup2minioName     = "backup-to-minio"
	backup2minioImage    = backup2nfsImage
	secretMinioAccessKey = "MINIO_ACCESS_KEY"
	secretMinioSecretKey = "MINIO_SECRET_KEY"
)

var (
	ctx        = context.TODO()
	podHandler = pod.NewOrDie(ctx, "", "")
	depHandler = deployment.NewOrDie(ctx, "", "")
	rsHandler  = replicaset.NewOrDie(ctx, "", "")
	stsHandler = statefulset.NewOrDie(ctx, "", "")
	dsHandler  = daemonset.NewOrDie(ctx, "", "")
	pvHandler  = persistentvolume.NewOrDie(ctx, "")
	pvcHandler = persistentvolumeclaim.NewOrDie(ctx, "", "")
	secHandler = secret.NewOrDie(ctx, "", "")
	dynHandler = dynamic.NewOrDie(ctx, "", "")
)

var (
	ErrResourceType = errors.New("Backup.spec.backupFrom.resource field value must be pod, deployment, statefulset or daemonset")
	logger          = logrus.WithFields(logrus.Fields{})
	costedTime      time.Duration
)

// Do start to backup k8s pod/deployment/statefulset/daemonset defined in Backup object
// namespace is the k8s resource namespace
// name is the k8s resource name
func Do(ctx context.Context, namespace, name string) error {
	// ==============================
	// 1. dynamic handler get Backup object
	// ==============================
	begin := time.Now()
	gvk := schema.GroupVersionKind{
		Group:   types.GroupStorage,
		Version: types.GroupVersionStorage.Version,
		Kind:    types.KindBackup,
	}
	unstructObj, err := dynHandler.WithNamespace(namespace).WithGVK(gvk).Get(name)
	if err != nil {
		err = errors.Wrapf(err, `dynamic handler get "%s.%s" resource object failed`, types.ResourceBackup, types.GroupStorage)
		logger.Error(err)
		return err
	}
	backupObj := &storagev1alpha1.Backup{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructObj.UnstructuredContent(), backupObj); err != nil {
		err = errors.Wrapf(err, "convert unstructured object to %s.%s resource object failed", types.ResourceBackup, types.GroupStorage)
		logger.Error(err)
		return err
	}
	// setup logger
	backupFrom := backupObj.Spec.BackupFrom
	logger = logger.WithFields(logrus.Fields{
		"name":      name,
		"namespace": namespace,
		"resource":  backupFrom.Resource,
	})
	logger.WithField("cost", time.Now().Sub(begin).String()).Infof("Successfully get Backup object")

	// ==============================
	//  2. prepare pvc and pv metadata
	// ==============================
	begin = time.Now()
	logger.Infof("Start backup %s/%s", backupFrom.Resource, backupFrom.Name)
	pvcpvMap, err := getPvcpvMap(ctx, backupObj)
	if err != nil {
		logger.Error(err)
		return err
	}
	logger.WithField("cost", time.Now().Sub(begin).String()).Infof("Successfully prepare pvc and pv metadata")

	// ==============================
	// 3. backup to remote storage
	// ==============================
	for _, storage := range parseStorage(backupObj) {
		begin := time.Now()
		for pvc, meta := range pvcpvMap {
			if err := backupFactory(storage)(backupObj, pvc, meta); err != nil {
				err = errors.Wrapf(err, "Backup pvc/%s to %s failed", pvc, storage)
				logger.Error(err)
				return err
			}
			logger.WithField("cost", costedTime.String()).Infof("Successfully backup pvc/%s", pvc)
		}
		logger.WithField("cost", time.Now().Sub(begin).String()).Infof("Successfully backup all pvc to %s", storage)
	}

	logger.WithField("cost", time.Now().Sub(begin).String()).
		Infof("Successfully backup %s/%s", backupFrom.Resource, backupFrom.Name)
	return err
}

// The structured object for pv metadata is named pvdataMeta.
// pvdataMeta.volumeSource:
//   every persistentvolume(aka pv) has a backend valume, and the volme source
//   could be "csi", "nfs" or "hostPath"(further more see pv.spec). pvdataMeta.volumeSource
//   value contains the pv backend volume source type, such as csi, "nfs", "rbd" or "hostPath".
// pvdataMeta.nodeName:
//   nodeName indicates the k8s node name that the pod is running. the nodeName
//   is required by deployment/findpvdir to find the the persistentvolume data directory
//   path in the k8s node.
// pvdataMeta.podName:
//   The name of the deployment/statefulset/daemonset owned pod that we should to backup.
//   To backup persistentvolume data to nfs/minio/s3 reqests it.
// pvdataMeta.podUID
//   The UID name of the deployment/statefulset/daemonset owned pod that we should to backup.
//   To find the persistentvolume data directory path in k8s node requests it.
// pvdataMeta.pvdir
//   The persistentvolume data directory path in k8s node thtat found by deployment/findpvdir.
// pvdataMeta.pvname
//   The persistentvolume claimed by persistentvolumeclaim for podis mounts.
//   pod mounted pvc -> pvc claims pv -> k8s admin create pv manually or created by storageclass automatically.
type pvdataMeta struct {
	volumeSource string
	nodeName     string
	podName      string
	podUID       string
	pvdir        string
	pvname       string
}

// getPvcpvMap backup the k8s resource defined in Backup object to nfs storage.
func getPvcpvMap(ctx context.Context, backupObj *storagev1alpha1.Backup) (map[string]pvdataMeta, error) {
	beginTime := time.Now().UTC()
	defer func() {
		costedTime = time.Now().UTC().Sub(beginTime)
	}()

	var (
		err        error
		podObjList []*corev1.Pod
		namespace  = backupObj.GetNamespace()
		backupFrom = backupObj.Spec.BackupFrom
	)
	podHandler.ResetNamespace(namespace)
	depHandler.ResetNamespace(namespace)
	rsHandler.ResetNamespace(namespace)
	stsHandler.ResetNamespace(namespace)
	dsHandler.ResetNamespace(namespace)
	pvcHandler.ResetNamespace(namespace)
	switch backupFrom.Resource {
	case storagev1alpha1.PodResource:
		podObj, err := podHandler.Get(backupFrom.Name)
		if err != nil {
			// if the Pod resource not found, skip backup
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("pod/%s not found in namespace %s, skip backup", backupFrom.Name, namespace)
			}
			return nil, errors.Wrap(err, "pod handler get pod failed")
		}
		podObjList = append(podObjList, podObj)
	case storagev1alpha1.DeploymentResource:
		if podObjList, err = depHandler.GetPods(backupFrom.Name); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("deployment/%s not found in namespace %s, skip backup", backupFrom.Name, namespace)
			}
			return nil, errors.Wrap(err, "deployment handler get pod failed")
		}
	case storagev1alpha1.StatefulSetResource:
		if podObjList, err = stsHandler.GetPods(backupFrom.Name); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("statefulset/%s not found in namespace %s, skip backup", backupFrom.Name, namespace)
			}
			return nil, errors.Wrap(err, "statefulset handler get pod failed")
		}
	case storagev1alpha1.DaemonSetResource:
		if podObjList, err = dsHandler.GetPods(backupFrom.Name); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("daemonset/%s not found in namespace %s, skip backup", backupFrom.Name, namespace)
			}
			return nil, errors.Wrap(err, "daemonset handler get pod failed")
		}
	default:
		return nil, ErrResourceType
	}
	// podObjList contains all pods that managed/owned by the Deployment, StatefulSet or DaemonSet.
	// we iterate over each pod to get its mounted persistentvolumeclaim(aka pvc),
	// and the pvc as the map key, persistentvolume(aka pv) metadata as the value.
	// pv metadata is a structured object that contains necessary info for
	// deployment/findpvdir to find the pv data directory in the k8s node,
	// and for deployment/backuptonfs to create a pod to backup the pv data to nfs server.
	//
	// restic command to backup persistentvolume data to remote storage(nfs/minio/s3, etc.) should
	// specific the backup source.
	// The backup source is a file or directory path in k8s node, and the file or directory path
	// usually join by pvdataMeta.pvdir  + pvdataMeta.podUID +  "volumes" + pvdataMeta.volumeSource +
	// pvdataMeta.pvname.
	//
	// If the persistentvolumeclaim access modes is "ReadWriteMany", pvc may be mounted by
	// many pods. we use a map named pvcpvMap to prevent backup persistentvolume data many times.
	// for example:
	//    pod-a -> pvc-a -> pv-a
	//    pod-b -> pvc-a -> pv-a
	//    pod-c -> pvc-a -> pv-a
	// pod-a, pod-b and pod-c mounted the same pvc and use the same pv and use the same volume data.
	// To iterate every pod managed/owned by deployment/statefulset/daemonset may get the same pvc.

	// pvcpvMap 存在的意义: 不要重复备份同一个 pvc
	// 因为有些 pvc  为 ReadWriteMany 模式, 当一个 deployment 下的多个 pod 同时
	// 挂载了同一个 pvc, 默认会对这个 pvc 备份多次, 这完全没必要, 只需要备份一次即可
	// pvc name 作为 key, pvdataMeta 作为 value
	// 在这里只设置了 pv name
	pvcpvMap := make(map[string]pvdataMeta)
	for _, podObj := range podObjList {
		// 1. get nodeName, podUID
		meta := pvdataMeta{}
		var nodeName, podUID string
		if nodeName, err = podHandler.GetNodeName(podObj); err != nil {
			return nil, errors.Wrap(err, "pod handler get pod's node name failed")
		}
		if podUID, err = podHandler.GetUID(podObj); err != nil {
			return nil, errors.Wrap(err, "pod handler get pod's uid failed")
		}

		// 2. get volumeSource, pvname, set volumeSource, nodeName, podName, podUID, pvname
		pvcList, err := podHandler.GetPVC(podObj)
		if err != nil {
			return nil, errors.Wrap(err, "pod handler get persistentvolumeclaim failed")
		}
		logger.Debugf("The persistentvolumeclaims mounted by pod/%s are: %v", podObj.Name, pvcList)
		for _, pvc := range pvcList {
			// get the persistentvolume name claimed by persistentvolumeclaim resource.
			pvname, err := pvcHandler.GetPV(pvc)
			if err != nil {
				logger.Errorf("pvc handler get pv failed: %s", err.Error())
				continue
			}
			// get the persistentvolume backend volume type, such as "nfs", "csi", "hostPath", "local", etc.
			volumeSource, err := pvHandler.GetVolumeSource(pvname)
			if err != nil {
				logger.Errorf("pv handler get volume source failed: %s", err.Error())
				continue
			}
			meta.volumeSource = volumeSource
			meta.nodeName = nodeName
			meta.podName = podObj.GetName()
			meta.podUID = podUID
			meta.pvname = pvname
			pvcpvMap[pvc] = meta
		}
		// 3. create deployment/findpvdir to find the persistentvolume data directory in k8s node that mounted by pod.
		// the deployment should meet three condition:
		//   1.deployment should mount the k8s node root direcotry(is "/", not "/root")
		//   2.deployment usually deploy in the same namespace to operator
		//   3.deployment.spec.template.spec.nodeName should same to the pod,
		var pvdir string
		for _, pvc := range pvcList {
			meta := pvcpvMap[pvc]
			if pvdir, err = createFindpvdirDeployment(backupObj, meta); err != nil {
				return nil, fmt.Errorf("create deployment/%s failed: %s", findpvdirName+"-"+meta.nodeName, err.Error())
			}
			logger.WithField("cost", costedTime.String()).Infof("Found pvc/%s in pod/%s", pvc, podObj.GetName())
			if len(pvdir) == 0 {
				logger.WithField("VolumeSource", meta.volumeSource).Warnf("PVC/%s data directory not found", pvc)
				continue
			}
			logger.Debugf("The persistentvolume dir: %s", pvdir)
			meta.pvdir = pvdir
			pvcpvMap[pvc] = meta
		}
	}
	// If the length of pvcpvMap is zero, it's means that no persistentvolumeclaim mounted
	// by the backup target resource, skip backup.
	if len(pvcpvMap) == 0 {
		return nil, fmt.Errorf("There is no pvc mounted by the %s/%s, skip backup", backupFrom.Resource, backupFrom.Name)
	}
	// output pvcpvMap for debug
	for pvc, meta := range pvcpvMap {
		logger.Debugf("%v: %v", pvc, meta)
	}

	return pvcpvMap, nil
}

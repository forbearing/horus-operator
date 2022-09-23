package backup

import (
	"context"
	"errors"
	"fmt"
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/forbearing/horus-operator/pkg/util"
	"github.com/forbearing/k8s/daemonset"
	"github.com/forbearing/k8s/deployment"
	"github.com/forbearing/k8s/dynamic"
	"github.com/forbearing/k8s/persistentvolume"
	"github.com/forbearing/k8s/persistentvolumeclaim"
	"github.com/forbearing/k8s/pod"
	"github.com/forbearing/k8s/replicaset"
	"github.com/forbearing/k8s/secret"
	"github.com/forbearing/k8s/statefulset"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

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
//
type pvdataMeta struct {
	volumeSource string
	nodeName     string
	podName      string
	podUID       string
	pvdir        string
	pvname       string
}

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
)

// Do
func Do(ctx context.Context, backupObjNS, backupObjName string) error {
	beginTime := time.Now()
	dynHandler.ResetNamespace(backupObjNS)
	logger := logrus.WithFields(logrus.Fields{
		"Component": "Backup",
	})

	gvk := schema.GroupVersionKind{
		Group:   storagev1alpha1.GroupVersion.Group,
		Version: storagev1alpha1.GroupVersion.Version,
		Kind:    types.KindBackup,
	}
	unstructObj, err := dynHandler.WithGVK(gvk).Get(backupObjName)
	if err != nil {
		logger.Errorf("dynamic handler get Backup object failed: %s", err.Error())
		return nil
	}
	backupObj := &storagev1alpha1.Backup{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructObj.UnstructuredContent(), backupObj); err != nil {
		logger.Errorf("convert unstructured object to Backup object failed: %s", err.Error())
		return nil
	}

	backupFrom := backupObj.Spec.BackupFrom
	logger.WithFields(logrus.Fields{
		"Namespace": backupObj.GetNamespace(),
		"Resource":  backupFrom.Resource,
	})

	// 1. prepare pvc and pv metadata
	pvcpvMap, costedTime, err := getPvcpvMap(ctx, backupObj)
	if err != nil {
		logger.Error(err)
		return nil
	}
	logger.WithField("Cost", costedTime.String()).Infof("Successfully prepare pvc and pv metadata")

	// 2. Do backup
	if costedTime, err = doBackup(backupObj, pvcpvMap); err != nil {
		logger.Error(err)
		return nil
	}

	logger.WithField("Cost", time.Now().Sub(beginTime).String()).
		Infof("Successfully Backup %s/%s", backupFrom.Resource, backupFrom.Name)
	return nil
}

// getPvcpvMap backup the k8s resource defined in Backup object to nfs storage.
func getPvcpvMap(ctx context.Context, backupObj *storagev1alpha1.Backup) (map[string]pvdataMeta, time.Duration, error) {
	var (
		err        error
		podObjList []*corev1.Pod
		backupFrom = backupObj.Spec.BackupFrom
		namespace  = backupObj.GetNamespace()

		// pvcpvMap 存在的意义: 不要重复备份同一个 pvc
		// 因为有些 pvc  为 ReadWriteMany 模式, 当一个 deployment 下的多个 pod 同时
		// 挂载了同一个 pvc, 默认会对这个 pvc 备份多次, 这完全没必要, 只需要备份一次即可
		// pvc name 作为 key, pvdataMeta 作为 value
		// 在这里只设置了 pv name
		//
		pvcpvMap = make(map[string]pvdataMeta)
	)

	beginTime := time.Now()
	podHandler.ResetNamespace(backupObj.GetNamespace())
	depHandler.ResetNamespace(backupObj.GetNamespace())
	rsHandler.ResetNamespace(backupObj.GetNamespace())
	stsHandler.ResetNamespace(backupObj.GetNamespace())
	dsHandler.ResetNamespace(backupObj.GetNamespace())
	pvcHandler.ResetNamespace(backupObj.GetNamespace())
	logger := logrus.WithFields(logrus.Fields{
		"Component": "Backup",
		"Storage":   "NFS",
		"Resource":  backupFrom.Resource,
		"Namespace": namespace,
		"Name":      backupFrom.Name,
	})

	switch backupFrom.Resource {
	case storagev1alpha1.PodResource:
		logger.Infof("Start Backup pod/%s", backupFrom.Name)
		podObj, err := podHandler.Get(backupFrom.Name)
		if err != nil {
			// if the Pod resource not found, skip backup
			if apierrors.IsNotFound(err) {
				return nil, time.Duration(0), fmt.Errorf("pod/%s not found in namespace %s, skip backup", backupFrom.Name, namespace)
			}
			return nil, time.Duration(0), fmt.Errorf("pod handler get pod error: %s", err.Error())
		}
		podObjList = append(podObjList, podObj)
	case storagev1alpha1.DeploymentResource:
		logger.Infof("Start Backup deployment/%s", backupFrom.Name)
		if podObjList, err = depHandler.GetPods(backupFrom.Name); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, time.Duration(0), fmt.Errorf("deployment/%s not found in namespace %s, skip backup", backupFrom.Name, namespace)
			}
			return nil, time.Duration(0), fmt.Errorf("deployment handler get pod error: %s", err.Error())
		}
	case storagev1alpha1.StatefulSetResource:
		logger.Infof("Start Backup statefulset/%s", backupFrom.Name)
		if podObjList, err = stsHandler.GetPods(backupFrom.Name); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, time.Duration(0), fmt.Errorf("statefulset/%s not found in namespace %s, skip backup", backupFrom.Name, namespace)
			}
			return nil, time.Duration(0), fmt.Errorf("statefulset handler get pod error: %s", err.Error())
		}
	case storagev1alpha1.DaemonSetResource:
		logger.Infof("Start Backup daemonset/%s", backupFrom.Name)
		if podObjList, err = dsHandler.GetPods(backupFrom.Name); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, time.Duration(0), fmt.Errorf("daemonset/%s not found in namespace %s, skip backup", backupFrom.Name, namespace)
			}
			return nil, time.Duration(0), fmt.Errorf("daemonset handler get pod error: %s", err.Error())
		}
	default:
		return nil, time.Duration(0), fmt.Errorf(ErrResourceType.Error())
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
	for _, podObj := range podObjList {
		// 1. get nodeName, podUID
		meta := pvdataMeta{}
		var nodeName, podUID string
		if nodeName, err = podHandler.GetNodeName(podObj); err != nil {
			return nil, time.Duration(0), fmt.Errorf("pod handler get pod's node name error: %s", err.Error())
		}
		if podUID, err = podHandler.GetUID(podObj); err != nil {
			return nil, time.Duration(0), fmt.Errorf("pod handler get pod's uid error: %s", err.Error())
		}

		// 2. get volumeSource, pvname, set volumeSource, nodeName, podName, podUID, pvname
		pvcList, err := podHandler.GetPVC(podObj)
		if err != nil {
			return nil, time.Duration(0), fmt.Errorf("pod handler get persistentvolumeclaim error: %s", err.Error())
		}
		logger.Debugf("The persistentvolumeclaims mounted by pod/%s are: %v", podObj.Name, pvcList)
		for _, pvc := range pvcList {
			// get the persistentvolume name claimed by persistentvolumeclaim resource.
			pvname, err := pvcHandler.GetPV(pvc)
			if err != nil {
				logger.Errorf("persistentvolumeclaim get pv error: %s", err.Error())
				continue
			}
			// get the persistentvolume backend volume type, such as "nfs", "csi", "hostPath", "local", etc.
			volumeSource, err := pvHandler.GetVolumeSource(pvname)
			if err != nil {
				logger.Errorf("persistentvolume handler get volume source error: %s", err.Error())
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
		var costedTime time.Duration
		var pvdir string
		for _, pvc := range pvcList {
			meta := pvcpvMap[pvc]
			if pvdir, costedTime, err = createFindpvdirDeployment(backupObj, meta); err != nil {
				return nil, time.Duration(0), fmt.Errorf("create deployment/%s error: %s", findpvdirName+"-"+meta.nodeName, err.Error())
			}
			logger.WithField("Cost", costedTime.String()).Infof("Found pvc/%s in pod/%s", pvc, podObj.GetName())
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
		return nil, time.Duration(0), fmt.Errorf("There is no pvc mounted by the %s/%s, skip backup", backupFrom.Resource, backupFrom.Name)
	}
	// output pvcpvMap for debug
	for pvc, meta := range pvcpvMap {
		logger.Debugf("%v: %v", pvc, meta)
	}

	return pvcpvMap, time.Now().Sub(beginTime), nil
}

// doBackup
func doBackup(backupObj *storagev1alpha1.Backup, pvcpvMap map[string]pvdataMeta) (time.Duration, error) {
	beginTime := time.Now()
	operatorNamespace := util.GetOperatorNamespace()
	podHandler.ResetNamespace(operatorNamespace)
	logger := logrus.WithFields(logrus.Fields{
		"Component":         "backup",
		"Tool":              "restic",
		"OperatorNamespace": operatorNamespace,
	})

	for pvc, meta := range pvcpvMap {
		for _, remoteStorage := range parseBackupTo(backupObj) {
			var err error
			var costedTime time.Duration
			switch remoteStorage {
			case types.StorageNFS:
				if costedTime, err = Backup2NFS(backupObj, pvc, meta); err != nil {
					logger.WithField("Cost", costedTime.String()).Errorf("Backup to NFS failed: %s", err.Error())
					return time.Now().Sub(beginTime), err
				}
			case types.StorageMinIO:
				if costedTime, err = Backup2MinIO(backupObj, pvc, meta); err != nil {
					logger.WithField("Cost", costedTime.String()).Errorf("Backup to MinIO failed: %s", err.Error())
					return time.Now().Sub(beginTime), err
				}
			}
		}
	}
	return time.Now().Sub(beginTime), nil
}

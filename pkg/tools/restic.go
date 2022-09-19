package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/pkg/minio"
	"github.com/forbearing/k8s/daemonset"
	"github.com/forbearing/k8s/deployment"
	"github.com/forbearing/k8s/persistentvolume"
	"github.com/forbearing/k8s/persistentvolumeclaim"
	"github.com/forbearing/k8s/pod"
	"github.com/forbearing/k8s/replicaset"
	"github.com/forbearing/k8s/secret"
	"github.com/forbearing/k8s/statefulset"
	"github.com/forbearing/restic"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

type Storage string

const (
	StorageNFS        Storage = "nfs"
	StorageMinIO      Storage = "minio"
	StorageS3         Storage = "s3"
	StorageCephFS     Storage = "cephfs"
	StorageRestServer Storage = "restServer"
	StorageSFTP       Storage = "sftp"
	StorageRclone     Storage = "rclone"
)

const (
	defaultClusterName = "kubernetes"

	resticBackupSource = "/backup-source"
	resticRepo         = "/restic-repo"
	resticPasswd       = "mypass"
	mountHostRootPath  = "/host-root"

	HostBackupToNFS   = "backup-to-nfs"
	HostBackupToS3    = "backup-to-s3"
	HostBackupToMinio = "backup-to-minio"

	findpvdirName        = "findpvdir"
	findpvdirImage       = "hybfkuf/findpvdir:latest"
	backuptonfsName      = "backup-to-nfs"
	backuptonfsImage     = "hybfkuf/backup-tools-restic:latest"
	backuptominioName    = "backup-to-minio"
	backuptominioImage   = backuptonfsImage
	backuptominioUser    = "minioadmin"
	backuptominioPass    = "minioadmin"
	secretMinioAccessKey = "MINIO_ACCESS_KEY"
	secretMinioSecretKey = "MINIO_SECRET_KEY"

	createdTimeAnnotation   = "storage.hybfkuf.io/createdAt"
	updatedTimeAnnotation   = "storage.hybfkuf.io/updatedAt"
	restartedTimeAnnotation = "storage.hybfkuf.io/restartedAt"

	volumeHostPath = "hostPath"
	volumeLocal    = "local"
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
)

var (
	ResourceTypeError = errors.New("Backup.spec.backupFrom.resource field value must be pod, deployment, statefulset or daemonset")
)

// BackupToNFS backup the k8s resource defined in Backup object to nfs storage.
func BackupToNFS(ctx context.Context, operatorNamespace string,
	backupObj *storagev1alpha1.Backup, nfs *storagev1alpha1.NFS) error {
	var (
		err        error
		podObjList []*corev1.Pod
		backupFrom = backupObj.Spec.BackupFrom
		namespace  = backupObj.GetNamespace()
	)

	beginTime := time.Now()
	podHandler.ResetNamespace(backupObj.GetNamespace())
	depHandler.ResetNamespace(backupObj.GetNamespace())
	rsHandler.ResetNamespace(backupObj.GetNamespace())
	stsHandler.ResetNamespace(backupObj.GetNamespace())
	dsHandler.ResetNamespace(backupObj.GetNamespace())
	pvcHandler.ResetNamespace(backupObj.GetNamespace())
	logger := logrus.WithFields(logrus.Fields{
		"Component": "BackupToNFS",
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
		logger.Error(ResourceTypeError.Error())
		return nil
	}

	// pvcpvMap 存在的意义: 不要重复备份同一个 pvc
	// 因为有些 pvc  为 ReadWriteMany 模式, 当一个 deployment 下的多个 pod 同时
	// 挂载了同一个 pvc, 默认会对这个 pvc 备份多次, 这完全没必要, 只需要备份一次即可
	// pvc name 作为 key, pvdataMeta 作为 value
	// 在这里只设置了 pv name
	//
	pvcpvMap := make(map[string]pvdataMeta)
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
			return err
		}
		if podUID, err = podHandler.GetUID(podObj); err != nil {
			return err
		}

		// 2. get volumeSource, pvname, set volumeSource, nodeName, podName, podUID, pvname
		pvcList, err := podHandler.GetPVC(podObj)
		if err != nil {
			return fmt.Errorf("pod handler get persistentvolumeclaim faile: %s", err.Error())
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
			if pvdir, costedTime, err = createFindpvdirDeployment(operatorNamespace, backupObj, meta); err != nil {
				return fmt.Errorf("create deployment/%s error: %s", findpvdirName+"-"+meta.nodeName, err.Error())
			}
			logger.WithField("Cost", costedTime.String()).Infof("Found pvc/%s mounted by pod/%s", pvc, podObj.GetName())
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
		logger.Warnf("There is no pvc mounted by the %s/%s, skip backup", backupFrom.Resource, backupFrom.Name)
		return nil
	}
	// output pvcpvMap for debug
	for pvc, meta := range pvcpvMap {
		logger.Debugf("%v: %v", pvc, meta)
	}

	if _, err = backupToRemote(operatorNamespace, backupObj, pvcpvMap); err != nil {
		return err
	}

	logger.WithField("Cost", time.Now().Sub(beginTime).String()).
		Infof("Successfully Backup %s/%s", backupFrom.Resource, backupFrom.Name)
	return nil
}

// backupToRemote
func backupToRemote(operatorNamespace string, backupObj *storagev1alpha1.Backup, pvcpvMap map[string]pvdataMeta) (time.Duration, error) {
	beginTime := time.Now()
	podHandler.ResetNamespace(operatorNamespace)
	logger := logrus.WithFields(logrus.Fields{
		"Component": "backup",
		"Tool":      "restic",
	})

	for pvc, meta := range pvcpvMap {
		for _, remoteStorage := range parseBackupTo(backupObj) {
			var err error
			var execPod *corev1.Pod
			var costedTime time.Duration
			switch remoteStorage {
			case string(StorageNFS):
				// create deployment/backuptonfs to backup every pvc volume data.
				// there are three condition should meet.
				//   1.deployment mount the persistentvolumeclaim we should backup
				//   2.deployment mount nfs storage as persistentvolumeclaim
				//   3.execute restic commmand to backup persistentvolumeclaim data
				//     - "restic list keys" check whether resitc repository exist
				//     - "restic init" initial a resitc repository when repository not exist.
				//     - "restic backup" backup the persistentvolume data to nfs storage.
				if execPod, _, err = createBackup2nfsDeployment(operatorNamespace, backupObj, meta); err != nil {
					return time.Now().Sub(beginTime), err
				}
				// execute restic command to backup persistentvolume data to remote storage within the pod.
				if costedTime, err = backupByRestic(operatorNamespace, backupObj, execPod, pvc, meta, StorageNFS); err != nil {
					return time.Now().Sub(beginTime), err
				}
				logger.WithFields(logrus.Fields{
					"Cost":    costedTime.String(),
					"Storage": "NFS",
				}).Infof("Successfully backup pvc/%s", pvc)
			case string(StorageMinIO):
				// create deployment/backuptominio to backup every pvc volume data
				// there are two condition should meet.
				//   1.deployment mount the persistentvolumeclaim we should backup
				//   2.execute restic commmand to backup persistentvolumeclaim data
				//     - "restic list keys" check whether resitc repository exist
				//     - "restic init" initial a resitc repository when repository not exist.
				//     - "restic backup" backup the persistentvolume data to nfs storage.
				if execPod, _, err = createBackup2minioDepoyment(operatorNamespace, backupObj, meta); err != nil {
					return time.Now().Sub(beginTime), err
				}
				logger.WithFields(logrus.Fields{
					"Cost":    costedTime.String(),
					"Storage": "MinIO",
				})
				// execute restic command to backup persistentvolume data to remote storage within the pod.
				if costedTime, err = backupByRestic(operatorNamespace, backupObj, execPod, pvc, meta, StorageMinIO); err != nil {
					return time.Now().Sub(beginTime), err
				}
				logger.WithFields(logrus.Fields{
					"Cost":    costedTime.String(),
					"Storage": "MinIO",
				}).Infof("Successfully backup pvc/%s", pvc)
			}
		}
	}

	return time.Now().Sub(beginTime), nil
}

// createFindpvdirDeployment
func createFindpvdirDeployment(operatorNamespace string, backupObj *storagev1alpha1.Backup, meta pvdataMeta) (string, time.Duration, error) {
	beginTime := time.Now()
	podHandler.ResetNamespace(operatorNamespace)
	logger := logrus.WithFields(logrus.Fields{
		"Component": findpvdirName,
	})

	deployName := findpvdirName + "-" + meta.nodeName
	findpvdirBytes := []byte(fmt.Sprintf(
		// the deployment template
		findpvdirDeploymentTemplate,
		// deployment.metadata.name
		// deployment.metadata.namespace
		// deployment name, deployment namespace
		deployName, operatorNamespace,
		// deployment.spec.template.metadata.annotations
		// pod template annotations
		updatedTimeAnnotation, time.Now().Format(time.RFC3339),
		// deployment.spec.template.spec.nodeName
		// deployment.spec.template.spec.containers.image
		// node name, deployment image
		meta.nodeName, findpvdirImage,
		// deployment.spec.template.spec.containers.env
		// the environment variables passed to pods.
		backupObj.Spec.TimeZone))
	podObj, err := createAndGetRunningPod(operatorNamespace, findpvdirBytes)
	if err != nil {
		return "", time.Now().Sub(beginTime), err
	}

	// if persistentvolume volume source is hostPath or local, the returned value
	// is pvpath not pvdir, and pvpath = pvdir + pvname.
	// And it's no need to find the persistentvolume data directory path now, just return
	// the "hostPath" or "local" in k8s node path.
	cmdFindpvdir := []string{"findpvdir", "--pod-uid", meta.podUID, "--storage-type", meta.volumeSource}
	switch meta.volumeSource {
	case volumeHostPath:
		pvObj, err := pvHandler.Get(meta.pvname)
		if err != nil {
			return "", time.Now().Sub(beginTime), fmt.Errorf("persistentvolume handler get persistentvolume error: %s", err.Error())
		}
		return pvObj.Spec.HostPath.Path, time.Now().Sub(beginTime), nil
	case volumeLocal:
		pvObj, err := pvHandler.Get(meta.pvname)
		if err != nil {
			return "", time.Now().Sub(beginTime), fmt.Errorf("persistentvolume handler get persistentvolume error: %s", err.Error())
		}
		return pvObj.Spec.Local.Path, time.Now().Sub(beginTime), nil
	}
	logger.Debugf("executing command %v to find persistentvolume data in node %s", cmdFindpvdir, meta.nodeName)

	// It will execute command "cmdFindpvdir" within pod to find the persistentvolume data directory path
	// and output it to stdout.
	stdout := new(bytes.Buffer)
	if err := podHandler.ExecuteWithStream(podObj.Name, "", cmdFindpvdir, os.Stdin, stdout, io.Discard); err != nil {
		return "", time.Now().Sub(beginTime), fmt.Errorf("%s find the persistentvolume data directory failed: %s", findpvdirName, err.Error())
	}
	return strings.TrimSpace(stdout.String()), time.Now().Sub(beginTime), nil
}

// createBackup2nfsDeployment
func createBackup2nfsDeployment(operatorNamespace string, backupObj *storagev1alpha1.Backup, meta pvdataMeta) (*corev1.Pod, time.Duration, error) {
	beginTime := time.Now()

	deployName := backuptonfsName + "-" + meta.nodeName
	backuptonfsBytes := []byte(fmt.Sprintf(
		// the deployment template
		backuptonfsDeploymentTemplate,
		// deployment.metadata.name
		// deployment.metadata.namespace
		// deployment name, deployment namespace
		deployName, operatorNamespace,
		// deployment.spec.template.metadata.annotations
		// pod template annotations
		updatedTimeAnnotation, time.Now().Format(time.RFC3339),
		// deployment.spec.template.spec.nodeName
		// deployment.spec.template.spec.containers.image
		// node name, deployment image
		meta.nodeName, backuptonfsImage,
		// deployment.spec.template.spec.containers.env
		// the environment variables passed to pods
		backupObj.Spec.TimeZone, resticRepo,
		// restic repository mount path
		// deployment.spec.template.containers.env
		backupObj.Spec.BackupTo.NFS.CredentialName, resticRepo,
		// deployment.spec.template.volumes
		// the volumes mounted by pod
		backupObj.Spec.BackupTo.NFS.Server, backupObj.Spec.BackupTo.NFS.Path))
	podObj, err := createAndGetRunningPod(operatorNamespace, backuptonfsBytes)
	if err != nil {
		return nil, time.Now().Sub(beginTime), err
	}
	return podObj, time.Now().Sub(beginTime), nil
}

// createBackup2minioDepoyment backup persistentvolume data to minio object storage
func createBackup2minioDepoyment(operatorNamespace string, backupObj *storagev1alpha1.Backup, meta pvdataMeta) (*corev1.Pod, time.Duration, error) {
	beginTime := time.Now()

	scheme := backupObj.Spec.BackupTo.MinIO.Endpoint.Scheme
	address := backupObj.Spec.BackupTo.MinIO.Endpoint.Address
	port := backupObj.Spec.BackupTo.MinIO.Endpoint.Port
	bucket := backupObj.Spec.BackupTo.MinIO.Bucket
	folder := backupObj.Spec.BackupTo.MinIO.Folder
	credentialName := backupObj.Spec.BackupTo.MinIO.CredentialName

	secHandler.ResetNamespace(operatorNamespace)
	secObj, err := secHandler.Get(backupObj.Spec.BackupTo.MinIO.CredentialName)
	if err != nil {
		return nil, time.Duration(0), fmt.Errorf("secret handler get secret error: %s", err.Error())
	}
	accessKey := string(secObj.Data[secretMinioAccessKey])
	secretKey := string(secObj.Data[secretMinioSecretKey])

	endpoint := address + ":" + strconv.Itoa(int(port))
	resticRepo := "s3:" + scheme + "://" + endpoint + "/" + bucket
	if len(folder) != 0 {
		resticRepo = resticRepo + folder
	}
	// create minio bucket
	client := minio.New(endpoint, accessKey, secretKey, false)
	if err := minio.MakeBucket(client, bucket, ""); err != nil {
		return nil, time.Duration(0), fmt.Errorf("make minio bucket error: %s", err.Error())
	}

	deployName := backuptominioName + "-" + meta.nodeName
	backuptominioBytes := []byte(fmt.Sprintf(
		// the deployment template
		backuptominioDeploymentTemplate,
		// deployment.metadata.name
		// deployment.metadata.namespace
		// deployment name, deployment namespace
		deployName, operatorNamespace,
		// deployment.spec.template.metadata.annotations
		// pod template annotations
		updatedTimeAnnotation, time.Now().Format(time.RFC3339),
		// deployment.spec.template.spec.nodeName
		// deployment.spec.template.spec.containers.image
		// node name, deployment image
		meta.nodeName, backuptominioImage,
		// deployment.spec.template.spec.containers.env
		// the environment variables passed to pods
		backupObj.Spec.TimeZone, resticRepo,
		credentialName, credentialName, credentialName, credentialName, credentialName,
	))
	podObj, err := createAndGetRunningPod(operatorNamespace, backuptominioBytes)
	if err != nil {
		return nil, time.Now().Sub(beginTime), err
	}
	return podObj, time.Now().Sub(beginTime), nil
}

// backupByRestic
// clusterName as the --host argument
func backupByRestic(operatorNamespace string, backupObj *storagev1alpha1.Backup, execPod *corev1.Pod, pvc string, meta pvdataMeta, storage Storage) (time.Duration, error) {
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
	// if persistentvolume volume source is "hostPath" or "local", it's mean that
	// the meta.pvdir is pvpath not pvdir, and pvpath = pvdir + pvname.
	case volumeHostPath, volumeLocal:
		pvpath = filepath.Join(mountHostRootPath, meta.pvdir)
	}
	logger.Debugf("the path of persistentvolume data in k8s node: %s", pvpath)
	logger.Debugf("executing restic command to backup persistentvolume data within pod/%s", execPod.GetName())
	res := restic.NewIgnoreNotFound(context.TODO(), &restic.GlobalFlags{NoCache: true})
	tags := []string{string(backupObj.Spec.BackupFrom.Resource), backupObj.Namespace, backupObj.Spec.BackupFrom.Name, pvc}
	cmdCheckRepo := res.Command(restic.List{}.SetArgs("keys")).String()
	cmdInitRepo := res.Command(restic.Init{}).String()
	cmdBackup := res.Command(restic.Backup{Tag: tags, Host: clusterName}.SetArgs(pvpath)).String()

	logger.Debug(cmdCheckRepo)
	// 如果 restic list keys 失败, 说明 restic repository 不存在,则需要创建一下
	if err := podHandler.ExecuteWithStream(execPod.GetName(), "", strings.Split(cmdCheckRepo, " "),
		createPassStdin(resticPasswd, 1), io.Discard, io.Discard); err != nil {
		// 需要输入两遍密码, 一定需要输入两个 "\n", 否则 "restic init" 会一直卡在这里
		// 如果 restic list keys 失败, 说明 restic repository 不存在,则需要创建一下
		logger.Debug(cmdInitRepo)
		if err := podHandler.ExecuteWithStream(execPod.GetName(), "", strings.Split(cmdInitRepo, " "),
			createPassStdin(resticPasswd, 2), io.Discard, io.Discard); err != nil {
			logger.Error("restic init failed")
			return time.Now().Sub(beginTime), nil
		}
	}
	logger.Debug(cmdBackup)
	if err := podHandler.WithNamespace(operatorNamespace).ExecuteWithStream(execPod.GetName(), "", strings.Split(cmdBackup, " "),
		createPassStdin(resticPasswd), io.Discard, io.Discard); err != nil {
		logger.Errorf("restic backup pvc/%s failed, maybe the directory/file of %s do not exist in k8s node", pvc, pvpath)
	}

	return time.Now().Sub(beginTime), nil
}

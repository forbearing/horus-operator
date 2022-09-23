package backup

import (
	"fmt"
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/pkg/template"
	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/forbearing/horus-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
)

// createBackup2nfsDeployment
func createBackup2nfsDeployment(backupObj *storagev1alpha1.Backup, meta pvdataMeta) (*corev1.Pod, time.Duration, error) {
	beginTime := time.Now()

	deployName := backup2nfsName + "-" + meta.nodeName
	operatorNamespace := util.GetOperatorNamespace()
	backup2nfsBytes := []byte(fmt.Sprintf(
		// the deployment template
		template.Backup2nfsDeploymentTemplate,
		// deployment.metadata.name
		// deployment.metadata.namespace
		// deployment name, deployment namespace
		deployName, operatorNamespace,
		// deployment.spec.template.metadata.annotations
		// pod template annotations
		types.AnnotationUpdatedTime, time.Now().Format(time.RFC3339),
		// deployment.spec.template.spec.nodeName
		// deployment.spec.template.spec.containers.image
		// node name, deployment image
		meta.nodeName, backup2nfsImage,
		// deployment.spec.template.spec.containers.env
		// the environment variables passed to pods
		backupObj.Spec.TimeZone, resticRepo,
		// restic repository mount path
		// deployment.spec.template.containers.env
		backupObj.Spec.CredentialName, resticRepo,
		// deployment.spec.template.volumes
		// the volumes mounted by pod
		backupObj.Spec.BackupTo.NFS.Server, backupObj.Spec.BackupTo.NFS.Path))
	podObj, err := createAndGetRunningPod(operatorNamespace, backup2nfsBytes)
	if err != nil {
		return nil, time.Now().Sub(beginTime), err
	}
	return podObj, time.Now().Sub(beginTime), nil
}

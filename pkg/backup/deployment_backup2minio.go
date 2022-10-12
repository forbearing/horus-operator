package backup

import (
	"fmt"
	"strconv"
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/pkg/minio"
	"github.com/forbearing/horus-operator/pkg/template"
	"github.com/forbearing/horus-operator/pkg/types"
	"github.com/forbearing/horus-operator/pkg/util"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

// createBackup2minioDepoyment create a deployment to backup persistentvolume data to minio object storage
func createBackup2minioDepoyment(backupObj *storagev1alpha1.Backup, meta pvdataMeta) (*corev1.Pod, error) {
	beginTime := time.Now().UTC()
	defer func() {
		costedTime = time.Now().UTC().Sub(beginTime)
	}()

	scheme := backupObj.Spec.BackupTo.MinIO.Endpoint.Scheme
	address := backupObj.Spec.BackupTo.MinIO.Endpoint.Address
	port := backupObj.Spec.BackupTo.MinIO.Endpoint.Port
	bucket := backupObj.Spec.BackupTo.MinIO.Bucket
	folder := backupObj.Spec.BackupTo.MinIO.Folder
	credentialName := backupObj.Spec.CredentialName

	operatorNamespace := util.GetOperatorNamespace()
	secHandler.ResetNamespace(operatorNamespace)
	secObj, err := secHandler.Get(backupObj.Spec.CredentialName)
	if err != nil {
		return nil, errors.Wrap(err, "secret handler get secret failed")
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
		return nil, errors.Wrap(err, "make minio bucket failed")
	}
	if err := minio.MakeFolder(client, folder); err != nil {
		return nil, errors.Wrap(err, "make minio folder failed")
	}

	DeployNameBackup2MinIO := theDeployName(backup2minioName, backupObj, meta)
	backup2minioBytes := []byte(fmt.Sprintf(
		// the deployment template
		template.Backup2minioDeploymentTemplate,
		// deployment.metadata.name
		// deployment.metadata.namespace
		// deployment name, deployment namespace
		DeployNameBackup2MinIO, operatorNamespace,
		// deployment.spec.template.metadata.annotations
		// pod template annotations
		types.AnnotationUpdatedTime, time.Now().Format(time.RFC3339),
		// deployment.spec.template.spec.nodeName
		// deployment.spec.template.spec.containers.image
		// node name, deployment image
		meta.nodeName, backup2minioImage,
		// deployment.spec.template.spec.containers.env
		// the environment variables passed to pods
		backupObj.Spec.TimeZone, resticRepo,
		credentialName, credentialName, credentialName, credentialName, credentialName,
	))
	podObj, err := filterRunningPod(operatorNamespace, backup2minioBytes)
	if err != nil {
		return nil, err
	}
	return podObj, nil
}

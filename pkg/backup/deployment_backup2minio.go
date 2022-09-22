package backup

import (
	"fmt"
	"strconv"
	"time"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/pkg/minio"
	"github.com/forbearing/horus-operator/pkg/template"
	corev1 "k8s.io/api/core/v1"
)

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

	deployName := backup2minioName + "-" + meta.nodeName
	backup2minioBytes := []byte(fmt.Sprintf(
		// the deployment template
		template.Backup2minioDeploymentTemplate,
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
		meta.nodeName, backup2minioImage,
		// deployment.spec.template.spec.containers.env
		// the environment variables passed to pods
		backupObj.Spec.TimeZone, resticRepo,
		credentialName, credentialName, credentialName, credentialName, credentialName,
	))
	podObj, err := createAndGetRunningPod(operatorNamespace, backup2minioBytes)
	if err != nil {
		return nil, time.Now().Sub(beginTime), err
	}
	return podObj, time.Now().Sub(beginTime), nil
}

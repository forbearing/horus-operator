package types

import "fmt"

var (
	Backup2NFSDeployName    = "backup-to-nfs"
	Backup2NFSDeployLabel   = fmt.Sprintf("app.kubernetes.io/name=%s", Backup2NFSDeployName)
	Backup2MinioDeployName  = "backup-to-minio"
	Backup2MinioDeployLabel = fmt.Sprintf("app.kubernetes.io/name=%s", Backup2MinioDeployName)
	Backup2S3DeployName     = "backup-to-s3"
	Backup2S3DeployLabel    = fmt.Sprintf("app.kubernetes.io/name=%s", Backup2S3DeployName)
)

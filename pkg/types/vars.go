package types

import (
	"fmt"
	"time"
)

var (
	DefaultNamespace         = "default"
	DefaultOperatorName      = "horus-operator"
	DefaultOperatorNamespace = "horus-operator-system"
	DefaultClusterName       = "kubernetes"

	DefaultBackupFinalizerName    = "backup.storage.hybfkuf.io/finalizer"
	DefaultRestoreFinalizerName   = "restore.storage.hybfkuf.io/finalizer"
	DefaultCloneFinalizerName     = "clone.storage.hybfkuf.io/finalizer"
	DefaultMigrationFinalizerName = "migration.storage.hybfkuf.io/finalizer"
	DefaultTrafficFinalizerName   = "traffic.networking.hybfkuf.io/finalizer"

	DefaultBackupTimeout    = time.Hour
	DefaultRestoreTimeout   = time.Hour
	DefaultCloneTimeout     = time.Hour
	DefaultMigrationTimeout = time.Hour

	DefaultServiceAccountName = "horusctl"

	AnnotationCreatedTime   = "hybfkuf.io/createdAt"
	AnnotationUpdatedTime   = "hybfkuf.io/updatedAt"
	AnnotationRestartedTime = "hybfkuf.io/restartedAt"

	LabelPairPartOf    = "app.kubernetes.io/part-of=horus"
	LabelPairManagedBy = "app.kubernetes.io/managed-by=horus-operator"
)

var (
	Backup2NFSDeployName    = "backup-to-nfs"
	Backup2NFSDeployLabel   = fmt.Sprintf("app.kubernetes.io/name=%s", Backup2NFSDeployName)
	Backup2MinioDeployName  = "backup-to-minio"
	Backup2MinioDeployLabel = fmt.Sprintf("app.kubernetes.io/name=%s", Backup2MinioDeployName)
	Backup2S3DeployName     = "backup-to-s3"
	Backup2S3DeployLabel    = fmt.Sprintf("app.kubernetes.io/name=%s", Backup2S3DeployName)
)

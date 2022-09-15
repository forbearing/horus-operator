package types

import (
	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	networkingv1alpha1 "github.com/forbearing/horus-operator/apis/networking/v1alpha1"
)

type BackupInterface interface {
	DoBackup(backupObj *storagev1alpha1.Backup) error
}

type RestoreInterface interface {
	DoRestore(restoreObj *storagev1alpha1.Restore) error
}

type MigrationInterface interface {
	DoMigration(migrationObj *storagev1alpha1.Migration) error
}

type CloneInterface interface {
	DoClone(cloneObj *storagev1alpha1.Clone) error
}

type TrafficInterface interface {
	DoTraffic(trafficObj *networkingv1alpha1.Traffic)
}

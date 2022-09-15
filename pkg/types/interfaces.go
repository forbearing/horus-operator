package types

import (
	networkingv1alpha1 "github.com/forbearing/horus-operator/apis/networking/v1alpha1"
	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
)

type BackupInterface interface {
	Backup(backupObj *storagev1alpha1.Backup) error
}

type RestoreInterface interface {
	Restore(restoreObj *storagev1alpha1.Restore) error
}

type MigrationInterface interface {
	Migration(migrationObj *storagev1alpha1.Migration) error
}

type CloneInterface interface {
	Clone(cloneObj *storagev1alpha1.Clone) error
}

type TrafficInterface interface {
	Traffic(trafficObj *networkingv1alpha1.Traffic)
}

package tools

import (
	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
)

type BackupInterface interface {
	DoBackup(*storagev1alpha1.BackupFrom, *storagev1alpha1.BackupTo) error
}

type RestoreInterface interface {
	DoRestore(dst, src string) error
}

type MigrationInterface interface {
	DoMigration(dst, src string) error
}

type CloneInterface interface {
	DoClone(dst, src string) error
}

type TrafficInterface interface {
	DoTraffic()
}

func Backup(b BackupInterface, backupFrom *storagev1alpha1.BackupFrom, backupTo *storagev1alpha1.BackupTo) error {
	return b.DoBackup(backupFrom, backupTo)
}

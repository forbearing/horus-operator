package tools

import (
	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
)

type BackupInterface interface {
	DoBackup(*storagev1alpha1.Backup) error
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

func AddBackup(b BackupInterface, backup *storagev1alpha1.Backup) error {
	return b.DoBackup(backup)
}

func DeleteBackup(b BackupInterface, backup *storagev1alpha1.Backup) error {
	return b.DoBackup(backup)
}
func UpdateBackup(b BackupInterface, backup *storagev1alpha1.Backup) error {
	return b.DoBackup(backup)
}

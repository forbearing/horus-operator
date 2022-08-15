package tools

type BackupInterface interface {
	DoBackup(dst, src string) error
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

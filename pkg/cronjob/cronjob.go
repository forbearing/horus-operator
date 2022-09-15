package cronjob

import (
	"github.com/forbearing/horus-operator/pkg/types"
)

func AddBackup(types.BackupInterface)    {}
func DeleteBackup(types.BackupInterface) {}
func UpdateBackup(types.BackupInterface) {}

func AddRestore(types.RestoreInterface)    {}
func DeleteRestore(types.RestoreInterface) {}
func UpdateRestore(types.RestoreInterface) {}

func AddMigration(types.MigrationInterface)    {}
func DeleteMigration(types.MigrationInterface) {}
func UpdateMigration(types.MigrationInterface) {}

func AddClone(types.CloneInterface)    {}
func DeleteClone(types.CloneInterface) {}
func UpdateClone(types.CloneInterface) {}

func AddTraffic(types.TrafficInterface)    {}
func DeleteTraffic(types.TrafficInterface) {}
func UpdateTraffic(types.TrafficInterface) {}

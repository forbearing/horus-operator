package util

import (
	"reflect"
	"strings"

	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
	"github.com/forbearing/horus-operator/pkg/types"
)

// GetBackupToStorage find the storage which should data backup to.
func GetBackupToStorage(backupObj *storagev1alpha1.Backup) types.Storage {
	t := reflect.TypeOf(backupObj.Spec.BackupTo).Elem()
	v := reflect.ValueOf(backupObj.Spec.BackupTo).Elem()

	for i := 0; i < v.NumField(); i++ {
		val := v.Field(i).Interface()
		if !reflect.ValueOf(val).IsNil() {
			tag := t.Field(i).Tag.Get("json")
			switch {
			case strings.Contains(tag, string(types.StorageNFS)):
				return types.StorageNFS
			case strings.Contains(tag, string(types.StorageCephFS)):
				return types.StorageCephFS
			case strings.Contains(tag, string(types.StorageS3)):
				return types.StorageS3
			case strings.Contains(tag, string(types.StorageRestServer)):
				return types.StorageRestServer
			case strings.Contains(tag, string(types.StorageSFTP)):
				return types.StorageSFTP
			case strings.Contains(tag, string(types.StorageRClone)):
				return types.StorageRClone
			default:
				return types.Storage(strings.Split(tag, ",")[0])
			}
		}

	}
	return ""
}

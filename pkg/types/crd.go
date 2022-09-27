package types

import (
	networkingv1alpha1 "github.com/forbearing/horus-operator/apis/networking/v1alpha1"
	storagev1alpha1 "github.com/forbearing/horus-operator/apis/storage/v1alpha1"
)

var (
	KindBackup    = "Backup"
	KindRestore   = "Restore"
	KindClone     = "Clone"
	KindMigration = "Migration"
	KindTraffic   = "Traffic"

	ResourceBackup    = "backups"
	ResourceRestore   = "restores"
	ResourceClone     = "clones"
	ResourceMigration = "migrations"
	ResourceTraffic   = "traffics"

	GroupStorage    = storagev1alpha1.GroupVersion.Group
	GroupNetworking = networkingv1alpha1.GroupVersion.Group

	GroupVersionStorage    = storagev1alpha1.GroupVersion
	GroupVersionNetworking = networkingv1alpha1.GroupVersion
)

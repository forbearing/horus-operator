package types

type Storage string

const (
	StorageNFS        Storage = "nfs"
	StorageMinIO      Storage = "minio"
	StorageS3         Storage = "s3"
	StorageCephFS     Storage = "cephfs"
	StorageRestServer Storage = "restServer"
	StorageSFTP       Storage = "sftp"
	StorageRClone     Storage = "rclone"
)

const (
	VolumeHostPath = "hostPath"
	VolumeLocal    = "local"
)

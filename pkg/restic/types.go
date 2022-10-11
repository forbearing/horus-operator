package restic

import (
	"os"
	"time"
)

// NodeStat the output of restic subcommand `stat`.
// eg: `restic stats --json`
type NodeStat struct {
	TotalSize      int64 `json:"total_size"`
	TotalFileCount int64 `json:"total_file_count"`
	SnapshotsCount int64 `json:"snapshots_count"`
}

// NodeSnapshot represents the output of restic subcommand `snapshots`.
// eg: `restic snapshots --json`
type NodeSnapshot struct {
	Time     time.Time `json:"time"`
	Tree     string    `json:"tree"`
	Paths    []string  `json:"paths"`
	Hostname string    `json:"hostname"`
	Username string    `json:"username"`
	UID      uint32    `json:"uid"`
	GID      uint32    `json:"gid"`
	Tags     []string  `json:"tags"`
	ID       string    `json:"id"`
	ShortID  string    `json:"short_id"`
}

// NodeFind represents the output of restic subcommand `find`.
// eg: `restic find 871dafac zshrc --json`
type NodeFind struct {
	Matches  []Matche `json:"matches"`
	Hits     uint     `json:"hint"`
	Snapshot string   `json:"snapshot"`
}

type Matche struct {
	Path        string      `json:"path"`
	Permissions string      `json:"permissions"`
	Type        string      `json:"type"`
	Mode        os.FileMode `json:"mode"`
	Mtime       time.Time   `json:"mtime"`
	Atime       time.Time   `json:"atime"`
	Ctime       time.Time   `json:"ctime"`
	UID         uint        `json:"uid"`
	GID         uint        `json:"gid"`
	User        string      `json:"user"`
	Group       string      `json:"group"`
	DeviceID    int64       `json:"device_id"`
	Size        uint64      `json:"size"`
	Links       uint        `json:"links"`
}

// NodeLs represents the output of restic subcommand `ls`.
// eg: `restic ls 871dafac --json`
type NodeLs struct {
	UID uint32 `json:"uid"`
	GID uint32 `json:"gid"`

	// fields only for struct_type "node"
	Name        string      `json:"name,omitempty"`
	Type        string      `json:"type,omitempty"`
	Path        string      `json:"path,omitempty"`
	Size        uint64      `json:"size,omitempty"`
	Mode        os.FileMode `json:"mode,omitempty"`
	Permissions string      `json:"permissions,omitempty"`
	ModTime     time.Time   `json:"mtime,omitempty"`
	AccessTime  time.Time   `json:"atime,omitempty"`
	ChangeTime  time.Time   `json:"ctime,omitempty"`

	// fields only for struct_type "snapshot"
	Time     time.Time `json:"time,omitempty"`
	Parent   string    `json:"parent,omitempty"`
	Tree     string    `json:"tree,omitempty"`
	Paths    []string  `json:"paths,omitempty"`
	HostName string    `json:"hostname,omitempty"`
	Username string    `json:"username,omitempty"`
	Tags     []string  `json:"tags,omitempty"`
	ID       string    `json:"id,omitempty"`
	ShortID  string    `json:"short_id,omitempty"`

	StructType string `json:"struct_type"` // "node" or "snapshot"
}

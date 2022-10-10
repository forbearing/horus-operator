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
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Path        string      `json:"path"`
	UID         uint32      `json:"uid"`
	GID         uint32      `json:"gid"`
	Size        uint64      `json:"size,omitempty"`
	Mode        os.FileMode `json:"mode,omitempty"`
	Permissions string      `json:"permissions,omitempty"`
	ModTime     time.Time   `json:"mtime,omitempty"`
	AccessTime  time.Time   `json:"atime,omitempty"`
	ChangeTime  time.Time   `json:"ctime,omitempty"`
	StructType  string      `json:"struct_type"` // "node"
}

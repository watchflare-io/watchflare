package models

import "time"

// ContainerImageUpdate is the latest registry update check for the image a running
// container uses. There is one row per (host_id, container_id), replaced whenever
// the agent reports a changed result (at most every few minutes, rechecked every 6h).
type ContainerImageUpdate struct {
	HostID        string    `gorm:"type:char(36);primaryKey" json:"host_id"`
	ContainerID   string    `gorm:"primaryKey" json:"container_id"`
	ContainerName string    `json:"container_name"`
	Image         string    `json:"image"`
	LocalDigest   string    `json:"local_digest"`
	RemoteDigest  string    `json:"remote_digest"`
	Status        string    `json:"status"`
	Error         string    `json:"error"`
	CheckedAt     time.Time `json:"checked_at"`
	// Versions of the running and the registry image ("" if unknown); CommitsBehind
	// is set for rolling builds without version tags.
	LocalVersion  string `json:"local_version"`
	RemoteVersion string `json:"remote_version"`
	CommitsBehind int    `json:"commits_behind"`
}

// TableName pins the table name.
func (ContainerImageUpdate) TableName() string { return "container_image_updates" }

package models

import "time"

// ContainerState is the current (latest) snapshot of a running container on a
// host. Unlike ContainerMetric (time-series history), there is exactly one row
// per (host_id, container_id), replaced on every SendMetrics report.
type ContainerState struct {
	HostID               string    `gorm:"type:char(36);primaryKey" json:"host_id"`
	ContainerID          string    `gorm:"primaryKey" json:"container_id"`
	ContainerName        string    `json:"container_name"`
	Image                string    `json:"image"`
	CPUPercent           float64   `json:"cpu_percent"`
	MemoryUsedBytes      uint64    `json:"memory_used_bytes"`
	MemoryLimitBytes     uint64    `json:"memory_limit_bytes"`
	NetworkRxBytesPerSec uint64    `json:"network_rx_bytes_per_sec"`
	NetworkTxBytesPerSec uint64    `json:"network_tx_bytes_per_sec"`
	Runtime              string    `json:"runtime"`
	Status               string    `json:"status"`
	Health               string    `json:"health"`
	Ports                string    `json:"ports"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// TableName pins the table name (GORM would otherwise pluralize to
// container_states, which happens to match, but be explicit).
func (ContainerState) TableName() string { return "container_states" }

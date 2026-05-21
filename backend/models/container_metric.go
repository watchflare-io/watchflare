package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ContainerMetric stores per-container metrics for a host
type ContainerMetric struct {
	ID                   string    `gorm:"type:char(36);primaryKey;priority:1" json:"id"`
	Timestamp            time.Time `gorm:"primaryKey;priority:2;not null" json:"timestamp"`
	HostID               string    `gorm:"type:char(36);index:idx_container_metrics_host;not null" json:"host_id"`
	ContainerID          string    `gorm:"not null" json:"container_id"`
	ContainerName        string    `gorm:"not null" json:"container_name"`
	Image                string    `json:"image"`
	CPUPercent           float64   `json:"cpu_percent"`
	MemoryUsedBytes      uint64    `json:"memory_used_bytes"`
	MemoryLimitBytes     uint64    `json:"memory_limit_bytes"`
	NetworkRxBytesPerSec uint64    `json:"network_rx_bytes_per_sec"`
	NetworkTxBytesPerSec uint64    `json:"network_tx_bytes_per_sec"`
	Runtime              string    `json:"runtime"`
}

// BeforeCreate hook to generate UUID
func (cm *ContainerMetric) BeforeCreate(tx *gorm.DB) error {
	if cm.ID == "" {
		cm.ID = uuid.New().String()
	}
	return nil
}


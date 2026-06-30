package models

import "time"

// Service is a tracked systemd service (enabled or running) on a host.
type Service struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	HostID       string    `gorm:"type:char(36);index;not null" json:"host_id"`
	Name         string    `gorm:"not null" json:"name"`
	Description  string    `json:"description"`
	EnabledState string    `json:"enabled_state"`
	ActiveState  string    `json:"active_state"`
	SubState     string    `json:"sub_state"`
	CollectedAt  time.Time `json:"collected_at"`
	CreatedAt    time.Time `json:"created_at"`
}

func (Service) TableName() string { return "services" }

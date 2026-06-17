package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Alert metric type constants.
const (
	MetricTypeHostDown    = "host_down"
	MetricTypeCPUUsage    = "cpu_usage"
	MetricTypeMemoryUsage = "memory_usage"
	MetricTypeDiskUsage   = "disk_usage"
	MetricTypeLoadAvg     = "load_avg"
	MetricTypeLoadAvg5    = "load_avg_5"
	MetricTypeLoadAvg15   = "load_avg_15"
	MetricTypeTemperature = "temperature"
)

// AllMetricTypes lists all valid metric types in display order.
var AllMetricTypes = []string{
	MetricTypeHostDown,
	MetricTypeCPUUsage,
	MetricTypeMemoryUsage,
	MetricTypeDiskUsage,
	MetricTypeLoadAvg,
	MetricTypeLoadAvg5,
	MetricTypeLoadAvg15,
	MetricTypeTemperature,
}

// AlertRule holds the global default threshold for a metric type.
type AlertRule struct {
	MetricType      string    `gorm:"primaryKey;type:varchar(20)" json:"metric_type"`
	Enabled         bool      `gorm:"not null;default:false" json:"enabled"`
	Threshold       float64   `gorm:"not null;default:0" json:"threshold"`
	DurationMinutes int       `gorm:"not null;default:5" json:"duration_minutes"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// HostAlertRule is a per-host override of a global alert rule.
type HostAlertRule struct {
	HostID          string    `gorm:"type:char(36);primaryKey" json:"host_id"`
	MetricType      string    `gorm:"type:varchar(20);primaryKey" json:"metric_type"`
	Enabled         bool      `gorm:"not null;default:false" json:"enabled"`
	Threshold       float64   `gorm:"not null;default:0" json:"threshold"`
	DurationMinutes int       `gorm:"not null;default:5" json:"duration_minutes"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// AlertIncident tracks an active or resolved alert for a host.
type AlertIncident struct {
	ID             string     `gorm:"type:char(36);primaryKey" json:"id"`
	HostID         string     `gorm:"type:char(36);not null;index:idx_alert_incidents_host" json:"host_id"`
	MetricType     string     `gorm:"type:varchar(20);not null" json:"metric_type"`
	StartedAt      time.Time  `gorm:"not null" json:"started_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	PausedAt       *time.Time `json:"paused_at,omitempty"`
	Notified       bool       `gorm:"not null;default:false" json:"-"`
	ThresholdValue float64    `gorm:"not null;default:0" json:"threshold_value"`
	CurrentValue   float64    `gorm:"not null;default:0" json:"current_value"`
}

func (i *AlertIncident) BeforeCreate(_ *gorm.DB) error {
	if i.ID == "" {
		i.ID = uuid.NewString()
	}
	return nil
}

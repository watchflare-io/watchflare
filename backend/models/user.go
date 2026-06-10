package models

import (
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	ID               string    `gorm:"type:char(36);primarykey" json:"id"`
	Email            string    `gorm:"unique;not null" json:"email"`
	Password         string    `gorm:"not null" json:"-"`
	TOTPSecret       *string   `gorm:"type:text" json:"-"`
	TOTPEnabled      bool      `gorm:"default:false" json:"totp_enabled"`
	Username         string    `gorm:"type:varchar(50)" json:"username"`
	DefaultTimeRange string    `gorm:"type:varchar(10);default:'1h'" json:"default_time_range"`
	Theme            string    `gorm:"type:varchar(10);default:'system'" json:"theme"`

	// Display preferences
	TimeFormat              string `gorm:"type:varchar(3);default:'24h'" json:"time_format"`
	TemperatureUnit         string `gorm:"type:varchar(15);default:'celsius'" json:"temperature_unit"`
	NetworkUnit             string `gorm:"type:varchar(5);default:'bytes'" json:"network_unit"`
	DiskUnit                string `gorm:"type:varchar(5);default:'bytes'" json:"disk_unit"`
	GaugeWarningThreshold   int    `gorm:"default:70" json:"gauge_warning_threshold"`
	GaugeCriticalThreshold  int    `gorm:"default:90" json:"gauge_critical_threshold"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate hook to generate UUID before creating a user
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	// Set default preferences if not specified
	if u.DefaultTimeRange == "" {
		u.DefaultTimeRange = "1h"
	}
	if u.Theme == "" {
		u.Theme = "system"
	}
	if u.TimeFormat == "" {
		u.TimeFormat = "24h"
	}
	if u.TemperatureUnit == "" {
		u.TemperatureUnit = "celsius"
	}
	if u.NetworkUnit == "" {
		u.NetworkUnit = "bytes"
	}
	if u.DiskUnit == "" {
		u.DiskUnit = "bytes"
	}
	if u.GaugeWarningThreshold == 0 {
		u.GaugeWarningThreshold = 70
	}
	if u.GaugeCriticalThreshold == 0 {
		u.GaugeCriticalThreshold = 90
	}
	return nil
}

// HashPassword hashes the user's password using bcrypt
func (u *User) HashPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

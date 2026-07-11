package models

import (
	"time"

	"github.com/lib/pq"
)

// SmtpSettings holds the application-wide SMTP configuration.
// It is a singleton: exactly one row with singleton=true is ever stored.
type SmtpSettings struct {
	Singleton         bool           `gorm:"primaryKey;default:true"`
	Host              string         `gorm:"type:varchar(255);not null;default:''"`
	Port              int            `gorm:"not null;default:587"`
	Username          string         `gorm:"type:varchar(255);not null;default:''"`
	EncryptedPassword string         `gorm:"type:text;not null;default:''"`
	FromAddress       string         `gorm:"type:varchar(255);not null;default:''"`
	FromName          string         `gorm:"type:varchar(255);not null;default:''"`
	TLSMode           string         `gorm:"type:varchar(10);not null;default:'starttls'"`
	AuthType          string         `gorm:"type:varchar(10);not null;default:'plain'"`
	HeloName          string         `gorm:"type:varchar(255);not null;default:''"`
	NotificationEmail string         `gorm:"type:varchar(255);not null;default:''"`
	Enabled           bool           `gorm:"not null;default:false"`
	Categories        pq.StringArray `gorm:"column:categories;type:text[];not null;default:'{alerts}'" json:"categories"`
	UpdatedAt         time.Time
}

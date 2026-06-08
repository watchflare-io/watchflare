package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WebhookEndpoint struct {
	ID        string    `gorm:"type:char(36);primaryKey" json:"id"`
	URL       string    `gorm:"type:text;not null" json:"url"`
	Enabled   bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

func (w *WebhookEndpoint) BeforeCreate(_ *gorm.DB) error {
	if w.ID == "" {
		w.ID = uuid.NewString()
	}
	return nil
}

package models

import "time"

type TOTPBackupCode struct {
	ID       string     `gorm:"type:char(36);primarykey"`
	UserID   string     `gorm:"type:char(36);not null;index"`
	CodeHash string     `gorm:"type:varchar(64);not null"`
	UsedAt   *time.Time
}

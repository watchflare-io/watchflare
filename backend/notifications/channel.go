package notifications

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// ErrChannelNotFound is returned when no channel matches the given ID.
var ErrChannelNotFound = errors.New("notification channel not found")

// Channel is a destination for notifications, identified by a shoutrrr URL.
type Channel struct {
	ID           string         `gorm:"type:char(36);primaryKey" json:"id"`
	Name         string         `gorm:"type:varchar(100);not null" json:"name"`
	URLEncrypted string         `gorm:"column:url_encrypted;type:text;not null" json:"-"`
	Categories   pq.StringArray `gorm:"type:text[];not null;default:'{alerts}'" json:"categories"`
	Enabled      bool           `gorm:"not null;default:true" json:"enabled"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

func (Channel) TableName() string {
	return "notification_channels"
}

func (c *Channel) BeforeCreate(_ *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	return nil
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context) ([]Channel, error) {
	var channels []Channel
	if err := r.db.WithContext(ctx).Order("created_at ASC").Find(&channels).Error; err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	return channels, nil
}

func (r *Repository) ListEnabledByCategory(ctx context.Context, category string) ([]Channel, error) {
	var channels []Channel
	if err := r.db.WithContext(ctx).
		Where("enabled = ? AND ? = ANY(categories)", true, category).
		Find(&channels).Error; err != nil {
		return nil, fmt.Errorf("list enabled channels for category %s: %w", category, err)
	}
	return channels, nil
}

func (r *Repository) Get(ctx context.Context, id string) (Channel, error) {
	var c Channel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Channel{}, ErrChannelNotFound
		}
		return Channel{}, fmt.Errorf("get channel: %w", err)
	}
	return c, nil
}

func (r *Repository) Create(ctx context.Context, c *Channel) error {
	if err := r.db.WithContext(ctx).Create(c).Error; err != nil {
		return fmt.Errorf("create channel: %w", err)
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, id string, updates map[string]any) error {
	result := r.db.WithContext(ctx).Model(&Channel{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update channel: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrChannelNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&Channel{})
	if result.Error != nil {
		return fmt.Errorf("delete channel: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrChannelNotFound
	}
	return nil
}

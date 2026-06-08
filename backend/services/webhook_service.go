package services

import (
	"errors"
	"fmt"
	"time"
	"watchflare/backend/database"
	"watchflare/backend/models"
	"watchflare/backend/services/webhook"

	"gorm.io/gorm"
)

// ErrWebhookNotFound is returned when no webhook matches the given ID.
var ErrWebhookNotFound = errors.New("webhook not found")

// WebhookResponse is the API shape for a webhook endpoint.
type WebhookResponse struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	ServiceName string    `json:"service_name"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

func toWebhookResponse(ep models.WebhookEndpoint) WebhookResponse {
	return WebhookResponse{
		ID:          ep.ID,
		URL:         ep.URL,
		ServiceName: webhook.Detect(ep.URL).ServiceName(),
		Enabled:     ep.Enabled,
		CreatedAt:   ep.CreatedAt,
	}
}

// ListWebhooks returns all webhook endpoints in creation order.
func ListWebhooks() ([]WebhookResponse, error) {
	var endpoints []models.WebhookEndpoint
	if err := database.DB.Order("created_at ASC").Find(&endpoints).Error; err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	result := make([]WebhookResponse, len(endpoints))
	for i, ep := range endpoints {
		result[i] = toWebhookResponse(ep)
	}
	return result, nil
}

// GetWebhook returns a single webhook endpoint by ID.
func GetWebhook(id string) (WebhookResponse, error) {
	var ep models.WebhookEndpoint
	if err := database.DB.Where("id = ?", id).First(&ep).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return WebhookResponse{}, ErrWebhookNotFound
		}
		return WebhookResponse{}, fmt.Errorf("get webhook: %w", err)
	}
	return toWebhookResponse(ep), nil
}

// CreateWebhook inserts a new webhook endpoint. Returns the created record and
// whether the service type is unknown (Generic).
func CreateWebhook(rawURL string) (WebhookResponse, bool, error) {
	ep := models.WebhookEndpoint{URL: rawURL, Enabled: true}
	if err := database.DB.Create(&ep).Error; err != nil {
		return WebhookResponse{}, false, fmt.Errorf("create webhook: %w", err)
	}
	isUnknown := !webhook.IsKnownService(rawURL)
	return toWebhookResponse(ep), isUnknown, nil
}

// DeleteWebhook removes a webhook endpoint by ID. Returns an error if not found.
func DeleteWebhook(id string) error {
	result := database.DB.Where("id = ?", id).Delete(&models.WebhookEndpoint{})
	if result.Error != nil {
		return fmt.Errorf("delete webhook: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrWebhookNotFound
	}
	return nil
}

// SetWebhookEnabled updates the enabled flag for a webhook endpoint.
func SetWebhookEnabled(id string, enabled bool) error {
	result := database.DB.Model(&models.WebhookEndpoint{}).
		Where("id = ?", id).
		Updates(map[string]any{"enabled": enabled})
	if result.Error != nil {
		return fmt.Errorf("set webhook enabled: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrWebhookNotFound
	}
	return nil
}

package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"watchflare/backend/services"
	"watchflare/backend/services/webhook"

	"github.com/gin-gonic/gin"
)

// GetWebhooks returns all configured webhook endpoints.
func GetWebhooks(c *gin.Context) {
	webhooks, err := services.ListWebhooks()
	if err != nil {
		slog.Error("failed to list webhooks", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list webhooks"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"webhooks": webhooks})
}

// AddWebhookRequest is the body for POST /settings/webhooks.
type AddWebhookRequest struct {
	URL string `json:"url"`
}

// AddWebhook creates a new webhook endpoint.
func AddWebhook(c *gin.Context) {
	var req AddWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url is required"})
		return
	}
	u, err := url.Parse(req.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url must be a valid http or https URL"})
		return
	}

	ep, isUnknown, err := services.CreateWebhook(req.URL)
	if err != nil {
		slog.Error("failed to create webhook", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create webhook"})
		return
	}

	resp := gin.H{"webhook": ep}
	if isUnknown {
		resp["warning"] = "unknown_service"
	}
	c.JSON(http.StatusCreated, resp)
}

// DeleteWebhook removes a webhook endpoint by ID.
func DeleteWebhook(c *gin.Context) {
	id := c.Param("id")
	if err := services.DeleteWebhook(id); err != nil {
		if errors.Is(err, services.ErrWebhookNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
			return
		}
		slog.Error("failed to delete webhook", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete webhook"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "webhook deleted"})
}

// SetWebhookEnabledRequest is the body for PATCH /settings/webhooks/:id/enabled.
type SetWebhookEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

// SetWebhookEnabled updates the enabled flag for a webhook endpoint.
func SetWebhookEnabled(c *gin.Context) {
	id := c.Param("id")
	var req SetWebhookEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := services.SetWebhookEnabled(id, req.Enabled); err != nil {
		if errors.Is(err, services.ErrWebhookNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
			return
		}
		slog.Error("failed to update webhook", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update webhook"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "webhook updated"})
}

// TestWebhook sends a test notification to a specific webhook endpoint.
func TestWebhook(c *gin.Context) {
	id := c.Param("id")
	ep, err := services.GetWebhook(id)
	if err != nil {
		if errors.Is(err, services.ErrWebhookNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
			return
		}
		slog.Error("failed to get webhook for test", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to find webhook"})
		return
	}

	sender := webhook.Detect(ep.URL)
	event := webhook.WebhookEvent{
		Event:    webhook.EventTest,
		HostName: "Watchflare",
		Title:    "Watchflare test notification",
		Body:     "Your webhook is configured correctly.",
	}
	if err := sender.Send(c.Request.Context(), ep.URL, event); err != nil {
		slog.Error("webhook test failed", "id", id, "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to deliver test notification"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "test notification sent"})
}

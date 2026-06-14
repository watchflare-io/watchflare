package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"

	"watchflare/backend/notifications"
)

// testCooldownPeriod is the minimum interval between two test sends for the
// same notification channel. Prevents accidental double-clicks and discourages
// abuse of the /test endpoint as a notification relay.
const testCooldownPeriod = 10 * time.Second

// testCooldowns tracks the last test send per channel in memory. Cleared on
// process restart. Safe for concurrent access.
var testCooldowns = struct {
	mu   sync.Mutex
	last map[string]time.Time
}{last: map[string]time.Time{}}

// acquireTestSlot returns the remaining cooldown for channelID. When > 0 the
// caller must reject the request. When 0, the slot is reserved (last test
// timestamp updated) and the caller may proceed.
func acquireTestSlot(channelID string) time.Duration {
	testCooldowns.mu.Lock()
	defer testCooldowns.mu.Unlock()
	now := time.Now()
	if last, ok := testCooldowns.last[channelID]; ok {
		if remaining := testCooldownPeriod - now.Sub(last); remaining > 0 {
			return remaining
		}
	}
	testCooldowns.last[channelID] = now
	return 0
}

// channelResponse is the API shape for a NotificationChannel.
// URL is always masked (full URL never leaves the server).
type channelResponse struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	URLMasked  string    `json:"url_masked"`
	Categories []string  `json:"categories"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func toChannelResponse(c notifications.Channel, plainURL string) channelResponse {
	return channelResponse{
		ID:         c.ID,
		Name:       c.Name,
		URLMasked:  notifications.MaskShoutrrrURL(plainURL),
		Categories: []string(c.Categories),
		Enabled:    c.Enabled,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
}

// ListNotificationChannels returns all channels (URL masked).
func ListNotificationChannels(c *gin.Context) {
	channels, err := notifications.Default.Repo().List(c.Request.Context())
	if err != nil {
		slog.Error("list notification channels failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list notification channels"})
		return
	}

	out := make([]channelResponse, 0, len(channels))
	for _, ch := range channels {
		plain, err := notifications.Default.DecryptURL(ch.URLEncrypted)
		if err != nil {
			slog.Warn("decrypt channel for listing failed", "channel_id", ch.ID, "error", err)
			plain = "" // mask returns *** for empty
		}
		out = append(out, toChannelResponse(ch, plain))
	}
	c.JSON(http.StatusOK, gin.H{"channels": out})
}

// GetNotificationChannel returns a single channel by ID.
func GetNotificationChannel(c *gin.Context) {
	id := c.Param("id")
	ch, err := notifications.Default.Repo().Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, notifications.ErrChannelNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "notification channel not found"})
			return
		}
		slog.Error("get notification channel failed", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get notification channel"})
		return
	}
	plain, _ := notifications.Default.DecryptURL(ch.URLEncrypted)
	c.JSON(http.StatusOK, gin.H{"channel": toChannelResponse(ch, plain)})
}

// createChannelRequest is the body for POST /notifications/channels.
type createChannelRequest struct {
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	Categories []string `json:"categories"`
	Enabled    *bool    `json:"enabled"`
}

// CreateNotificationChannel inserts a new channel.
func CreateNotificationChannel(c *gin.Context) {
	var req createChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.URL = strings.TrimSpace(req.URL)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url is required"})
		return
	}
	if len(req.Name) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name must be 100 characters or fewer"})
		return
	}
	if err := notifications.ValidateShoutrrrURL(req.URL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	categories := normalizeCategories(req.Categories)
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	encrypted, err := notifications.Default.EncryptURL(req.URL)
	if err != nil {
		slog.Error("encrypt notification channel url failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store channel"})
		return
	}

	ch := &notifications.Channel{
		Name:         req.Name,
		URLEncrypted: encrypted,
		Categories:   pq.StringArray(categories),
		Enabled:      enabled,
	}
	if err := notifications.Default.Repo().Create(c.Request.Context(), ch); err != nil {
		slog.Error("create notification channel failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create notification channel"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"channel": toChannelResponse(*ch, req.URL)})
}

// updateChannelRequest is the body for PATCH /notifications/channels/:id.
// All fields are optional. URL is updated only when non-empty (empty preserves the stored URL).
type updateChannelRequest struct {
	Name       *string  `json:"name"`
	URL        *string  `json:"url"`
	Categories []string `json:"categories"`
	Enabled    *bool    `json:"enabled"`
}

// UpdateNotificationChannel patches an existing channel.
func UpdateNotificationChannel(c *gin.Context) {
	id := c.Param("id")
	var req updateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	updates := map[string]any{}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name cannot be empty"})
			return
		}
		if len(name) > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name must be 100 characters or fewer"})
			return
		}
		updates["name"] = name
	}
	if req.URL != nil {
		trimmed := strings.TrimSpace(*req.URL)
		if trimmed != "" {
			if err := notifications.ValidateShoutrrrURL(trimmed); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			encrypted, err := notifications.Default.EncryptURL(trimmed)
			if err != nil {
				slog.Error("encrypt notification channel url failed", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store channel"})
				return
			}
			updates["url_encrypted"] = encrypted
		}
	}
	if req.Categories != nil {
		updates["categories"] = pq.StringArray(normalizeCategories(req.Categories))
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}
	updates["updated_at"] = time.Now()

	if err := notifications.Default.Repo().Update(c.Request.Context(), id, updates); err != nil {
		if errors.Is(err, notifications.ErrChannelNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "notification channel not found"})
			return
		}
		slog.Error("update notification channel failed", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update notification channel"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "notification channel updated"})
}

// DeleteNotificationChannel removes a channel by ID.
func DeleteNotificationChannel(c *gin.Context) {
	id := c.Param("id")
	if err := notifications.Default.Repo().Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, notifications.ErrChannelNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "notification channel not found"})
			return
		}
		slog.Error("delete notification channel failed", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete notification channel"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "notification channel deleted"})
}

// TestNotificationChannel sends a fixed test message to a single channel.
// A per-channel cooldown protects against accidental double-clicks and abuse.
func TestNotificationChannel(c *gin.Context) {
	id := c.Param("id")

	if remaining := acquireTestSlot(id); remaining > 0 {
		c.Header("Retry-After", strconv.Itoa(int(remaining.Round(time.Second).Seconds())))
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":               "test cooldown active",
			"retry_after_seconds": int(remaining.Round(time.Second).Seconds()),
		})
		return
	}

	err := notifications.Default.SendToChannel(
		c.Request.Context(),
		id,
		"Watchflare test notification",
		"Your notification channel is configured correctly.",
	)
	if err != nil {
		if errors.Is(err, notifications.ErrChannelNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "notification channel not found"})
			return
		}
		slog.Error("send test notification failed", "id", id, "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to deliver test notification"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "test notification sent"})
}

// normalizeCategories cleans the user-provided categories list:
// trims whitespace, drops empty entries and unknown values, deduplicates,
// and falls back to [CategoryAlerts] when nothing valid remains.
func normalizeCategories(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		v := strings.TrimSpace(raw)
		switch v {
		case notifications.CategoryAlerts, notifications.CategoryTransactional:
		default:
			continue
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	if len(out) == 0 {
		return []string{notifications.CategoryAlerts}
	}
	return out
}

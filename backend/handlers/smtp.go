package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"watchflare/backend/services"

	"github.com/gin-gonic/gin"
)

const defaultSMTPPort = 587

// UpdateSMTPSettingsRequest is the body for PUT /settings/smtp.
type UpdateSMTPSettingsRequest struct {
	Host              string   `json:"host"`
	Port              int      `json:"port"`
	Username          string   `json:"username"`
	Password          string   `json:"password"` // empty = keep existing password
	FromAddress       string   `json:"from_address"`
	FromName          string   `json:"from_name"`
	TLSMode           string   `json:"tls_mode"`
	AuthType          string   `json:"auth_type"`
	HeloName          string   `json:"helo_name"`
	NotificationEmail string   `json:"notification_email"`
	Enabled           bool     `json:"enabled"`
	Categories        []string `json:"categories"`
}

// TestSMTPRequest is the body for POST /settings/smtp/test.
type TestSMTPRequest struct {
	Recipient string `json:"recipient"` // optional — defaults to the authenticated user's email
}

// GetSMTPSettings returns the current SMTP configuration (password masked).
func GetSMTPSettings(c *gin.Context) {
	settings, err := services.GetSMTPSettings()
	if err != nil {
		slog.Error("failed to fetch SMTP settings", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch SMTP settings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"smtp": settings})
}

// UpdateSMTPSettings saves the SMTP configuration.
func UpdateSMTPSettings(c *gin.Context) {
	var req UpdateSMTPSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default port
	if req.Port == 0 {
		req.Port = defaultSMTPPort
	} else if req.Port < 1 || req.Port > 65535 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "port must be between 1 and 65535"})
		return
	}

	// Default and validate TLS mode
	if req.TLSMode == "" {
		req.TLSMode = services.TLSModeStartTLS
	} else if req.TLSMode != services.TLSModeNone &&
		req.TLSMode != services.TLSModeStartTLS &&
		req.TLSMode != services.TLSModeSSL {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tls_mode, valid values: none, starttls, tls"})
		return
	}

	// Default and validate auth type
	if req.AuthType == "" {
		req.AuthType = services.SMTPAuthPlain
	} else if req.AuthType != services.SMTPAuthPlain && req.AuthType != services.SMTPAuthLogin {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid auth_type, valid values: plain, login"})
		return
	}

	// Validate notification email if provided (optional field) and normalize to bare address
	if req.NotificationEmail != "" {
		addr, err := mail.ParseAddress(req.NotificationEmail)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "notification email is not a valid email address"})
			return
		}
		req.NotificationEmail = addr.Address
	}

	// Require fields when enabling
	if req.Enabled && req.FromName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sender name is required when SMTP is enabled"})
		return
	}
	if req.Enabled && req.FromAddress == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sender email is required when SMTP is enabled"})
		return
	}
	if req.Enabled {
		if _, err := mail.ParseAddress(req.FromAddress); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "sender email is not a valid email address"})
			return
		}
	}
	if req.Enabled && req.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host is required when SMTP is enabled"})
		return
	}

	if err := services.UpdateSMTPSettings(services.SMTPSettingsInput{
		Host:              req.Host,
		Port:              req.Port,
		Username:          req.Username,
		Password:          req.Password,
		FromAddress:       req.FromAddress,
		FromName:          req.FromName,
		TLSMode:           req.TLSMode,
		AuthType:          req.AuthType,
		HeloName:          req.HeloName,
		NotificationEmail: req.NotificationEmail,
		Enabled:           req.Enabled,
		Categories:        defaultCategories(req.Categories),
	}); err != nil {
		slog.Error("failed to save SMTP settings", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save SMTP settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "SMTP settings saved successfully"})
}

// TestSMTPConnection sends a test email to verify the SMTP configuration.
func TestSMTPConnection(c *gin.Context) {
	var req TestSMTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate provided recipient
	if req.Recipient != "" {
		if _, err := mail.ParseAddress(req.Recipient); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid recipient email address"})
			return
		}
	}

	// Default recipient to the authenticated user's email
	if req.Recipient == "" {
		userID, ok := getUserID(c)
		if !ok {
			return
		}
		user, err := services.GetUser(userID)
		if err != nil {
			slog.Error("failed to get user for SMTP test", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
			return
		}
		if user.Email == "" {
			slog.Error("user has no email address", "user_id", userID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user has no email address"})
			return
		}
		req.Recipient = user.Email
	}

	if err := services.TestSMTPConnection(req.Recipient); err != nil {
		var configErr *services.SMTPConfigError
		if errors.As(err, &configErr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			slog.Error("SMTP test failed", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send test email"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Test email sent to " + req.Recipient})
}

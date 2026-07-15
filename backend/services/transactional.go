package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"watchflare/backend/config"
	"watchflare/backend/database"
	"watchflare/backend/encryption"
	"watchflare/backend/models"
	"watchflare/backend/notifications"

	"gorm.io/gorm"
)

// AccountEvent identifies a security-relevant account event.
type AccountEvent string

const (
	AccountEventLogin           AccountEvent = "login"
	AccountEventPasswordChanged AccountEvent = "password_changed"
	AccountEventTOTPEnabled     AccountEvent = "totp_enabled"
	AccountEventTOTPDisabled    AccountEvent = "totp_disabled"
	AccountEventEmailChanged    AccountEvent = "email_changed"
)

// AccountEventMeta carries optional context for a transactional notification.
type AccountEventMeta struct {
	IP string
	At time.Time
}

func shouldEmailTransactional(s *models.SmtpSettings) bool {
	return s.Enabled && slices.Contains([]string(s.Categories), notifications.CategoryTransactional)
}

func buildTransactionalContent(event AccountEvent, meta AccountEventMeta) (subject, body string) {
	at := meta.At
	if at.IsZero() {
		at = time.Now()
	}
	ts := at.Format(time.RFC1123)
	switch event {
	case AccountEventLogin:
		subject = "New Watchflare login"
		if meta.IP != "" {
			body = fmt.Sprintf("A new login to your Watchflare account was detected from IP %s at %s. If this was not you, change your password immediately.", meta.IP, ts)
		} else {
			body = fmt.Sprintf("A new login to your Watchflare account was detected at %s. If this was not you, change your password immediately.", ts)
		}
	case AccountEventPasswordChanged:
		subject = "Watchflare password changed"
		body = fmt.Sprintf("Your Watchflare password was changed at %s. If this was not you, secure your account immediately.", ts)
	case AccountEventTOTPEnabled:
		subject = "Two-factor authentication enabled"
		body = fmt.Sprintf("Two-factor authentication was enabled on your Watchflare account at %s.", ts)
	case AccountEventTOTPDisabled:
		subject = "Two-factor authentication disabled"
		body = fmt.Sprintf("Two-factor authentication was disabled on your Watchflare account at %s. If this was not you, secure your account immediately.", ts)
	case AccountEventEmailChanged:
		subject = "Watchflare email changed"
		body = fmt.Sprintf("The email address on your Watchflare account was changed at %s. If this was not you, secure your account immediately.", ts)
	default:
		subject = "Watchflare account notification"
		body = fmt.Sprintf("An account event occurred at %s.", ts)
	}
	return subject, body
}

// NotifyAccountEvent delivers a transactional notification best-effort and
// asynchronously. It returns immediately and never affects the caller.
func NotifyAccountEvent(event AccountEvent, recipients []string, meta AccountEventMeta) {
	go func() {
		subject, body := buildTransactionalContent(event, meta)
		sendTransactionalEmail(recipients, subject, body)
		if notifications.Default != nil {
			for _, err := range notifications.Default.Broadcast(context.Background(), notifications.CategoryTransactional, subject, body) {
				slog.Warn("transactional channel delivery failed", "event", event, "error", err)
			}
		}
	}()
}

func sendTransactionalEmail(recipients []string, subject, body string) {
	var s models.SmtpSettings
	if err := database.DB.First(&s).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Warn("transactional email: failed to load smtp settings", "error", err)
		}
		return
	}
	if !shouldEmailTransactional(&s) {
		return
	}
	var plainPassword string
	if s.EncryptedPassword != "" {
		if config.AppConfig.NotificationEncryptionKey == "" {
			slog.Warn("transactional email: encryption key not configured")
			return
		}
		var err error
		plainPassword, err = encryption.Decrypt(s.EncryptedPassword, config.AppConfig.NotificationEncryptionKey)
		if err != nil {
			slog.Warn("transactional email: failed to decrypt smtp password", "error", err)
			return
		}
	}
	for _, r := range recipients {
		if r == "" {
			continue
		}
		if err := sendEmail(&s, plainPassword, r, subject, body); err != nil {
			slog.Warn("transactional email delivery failed", "recipient", r, "error", err)
		}
	}
}

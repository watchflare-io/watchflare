package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strings"
	"time"

	"watchflare/backend/config"
	"watchflare/backend/database"
	"watchflare/backend/encryption"
	"watchflare/backend/models"
	"watchflare/backend/notifications"

	"gorm.io/gorm"
)

type AccountEvent string

const (
	AccountEventLogin               AccountEvent = "login"
	AccountEventPasswordChanged     AccountEvent = "password_changed"
	AccountEventTOTPEnabled         AccountEvent = "totp_enabled"
	AccountEventTOTPDisabled        AccountEvent = "totp_disabled"
	AccountEventEmailChanged        AccountEvent = "email_changed"
	AccountEventEmailChangedConfirm AccountEvent = "email_changed_confirm"
)

type AccountEventMeta struct {
	IP        string
	UserAgent string
	NewEmail  string
	At        time.Time
}

func displayIP(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip
	}
	if parsed.IsLoopback() {
		return "127.0.0.1"
	}
	if v4 := parsed.To4(); v4 != nil {
		return v4.String()
	}
	return ip
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
	const signoff = "\n\nSent from Watchflare"
	switch event {
	case AccountEventLogin:
		subject = "New login to your Watchflare account"
		var b strings.Builder
		b.WriteString("A new login to your Watchflare account has just been recorded.\n\n")
		fmt.Fprintf(&b, "Time:   %s\n", ts)
		if meta.IP != "" {
			fmt.Fprintf(&b, "IP:     %s\n", displayIP(meta.IP))
		}
		if meta.UserAgent != "" {
			fmt.Fprintf(&b, "Device: %s\n", meta.UserAgent)
		}
		b.WriteString("\nIf this was you, no action is needed. If it wasn't, change your password now to keep your account secure.")
		b.WriteString(signoff)
		body = b.String()
	case AccountEventPasswordChanged:
		subject = "Your Watchflare password was changed"
		body = fmt.Sprintf("Your Watchflare password has just been changed.\n\nTime: %s\n\nIf this was you, no action is needed. If it wasn't, your account may be compromised, secure it right away.%s", ts, signoff)
	case AccountEventTOTPEnabled:
		subject = "Two-factor authentication enabled"
		body = fmt.Sprintf("Two-factor authentication has just been enabled on your Watchflare account.\n\nTime: %s\n\nIf this was you, no action is needed. If it wasn't, secure your account right away.%s", ts, signoff)
	case AccountEventTOTPDisabled:
		subject = "Two-factor authentication disabled"
		body = fmt.Sprintf("Two-factor authentication has just been disabled on your Watchflare account.\n\nTime: %s\n\nIf this was you, no action is needed. If it wasn't, secure your account right away.%s", ts, signoff)
	case AccountEventEmailChanged:
		subject = "Your Watchflare email was changed"
		var b strings.Builder
		if meta.NewEmail != "" {
			fmt.Fprintf(&b, "The email address on your Watchflare account has just been changed to %s.\n\n", meta.NewEmail)
		} else {
			b.WriteString("The email address on your Watchflare account has just been changed.\n\n")
		}
		fmt.Fprintf(&b, "Time: %s\n\n", ts)
		b.WriteString("If this was you, no action is needed. If it wasn't, secure your account right away.")
		b.WriteString(signoff)
		body = b.String()
	case AccountEventEmailChangedConfirm:
		subject = "Your Watchflare email is now active"
		body = fmt.Sprintf("This address is now the email address for your Watchflare account.\n\nTime: %s\n\nIf you did not request this change, secure the account right away.%s", ts, signoff)
	default:
		subject = "Watchflare account notification"
		body = fmt.Sprintf("An account event occurred at %s.%s", ts, signoff)
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

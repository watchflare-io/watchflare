package services

import (
	"errors"
	"fmt"
	"log/slog"
	"time"
	"watchflare/backend/config"
	"watchflare/backend/database"
	"watchflare/backend/encryption"
	"watchflare/backend/models"

	"github.com/lib/pq"
	"github.com/wneessen/go-mail"
	"gorm.io/gorm"
)

const (
	TLSModeNone     = "none"
	TLSModeStartTLS = "starttls"
	TLSModeSSL      = "tls"

	SMTPAuthPlain = "plain"
	SMTPAuthLogin = "login"

	smtpSendTimeout = 10 * time.Second
)

// SMTPConfigError represents a user-fixable configuration or connectivity error.
// Handlers map this type to HTTP 400.
type SMTPConfigError struct{ msg string }

func (e *SMTPConfigError) Error() string { return e.msg }
func newConfigError(msg string) error    { return &SMTPConfigError{msg: msg} }

// SMTPSettingsResponse is the API representation of SMTP settings.
// The password is never returned: only a flag indicating whether one is stored.
type SMTPSettingsResponse struct {
	Host              string   `json:"host"`
	Port              int      `json:"port"`
	Username          string   `json:"username"`
	PasswordSet       bool     `json:"password_set"`
	FromAddress       string   `json:"from_address"`
	FromName          string   `json:"from_name"`
	TLSMode           string   `json:"tls_mode"`
	AuthType          string   `json:"auth_type"`
	HeloName          string   `json:"helo_name"`
	NotificationEmail string   `json:"notification_email"`
	Enabled           bool     `json:"enabled"`
	Categories        []string `json:"categories"`
}

// SMTPSettingsInput carries the data for creating or updating SMTP settings.
type SMTPSettingsInput struct {
	Host              string
	Port              int
	Username          string
	Password          string // empty = keep the existing encrypted password
	FromAddress       string
	FromName          string
	TLSMode           string
	AuthType          string
	HeloName          string
	NotificationEmail string
	Enabled           bool
	Categories        []string
}

// GetSMTPSettings returns the current SMTP settings with the password masked.
// If no settings have been saved yet, sensible defaults are returned.
func GetSMTPSettings() (*SMTPSettingsResponse, error) {
	var s models.SmtpSettings
	err := database.DB.First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &SMTPSettingsResponse{Port: 587, TLSMode: TLSModeStartTLS, AuthType: SMTPAuthPlain, Categories: []string{"alerts"}}, nil
	}
	if err != nil {
		return nil, err
	}
	return &SMTPSettingsResponse{
		Host:              s.Host,
		Port:              s.Port,
		Username:          s.Username,
		PasswordSet:       s.EncryptedPassword != "",
		FromAddress:       s.FromAddress,
		FromName:          s.FromName,
		TLSMode:           s.TLSMode,
		AuthType:          s.AuthType,
		HeloName:          s.HeloName,
		NotificationEmail: s.NotificationEmail,
		Enabled:           s.Enabled,
		Categories:        []string(s.Categories),
	}, nil
}

// UpdateSMTPSettings upserts the singleton SMTP settings row.
// If input.Password is empty the existing encrypted password is preserved.
func UpdateSMTPSettings(input SMTPSettingsInput) error {
	var s models.SmtpSettings
	err := database.DB.First(&s).Error
	isNew := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !isNew {
		return err
	}

	s.Singleton = true
	s.Host = input.Host
	s.Port = input.Port
	s.Username = input.Username
	s.FromAddress = input.FromAddress
	s.FromName = input.FromName
	s.TLSMode = input.TLSMode
	s.AuthType = input.AuthType
	s.HeloName = input.HeloName
	s.NotificationEmail = input.NotificationEmail
	s.Enabled = input.Enabled
	s.Categories = pq.StringArray(input.Categories)

	if input.Password != "" {
		if config.AppConfig.NotificationEncryptionKey == "" {
			return errors.New("NOTIFICATION_ENCRYPTION_KEY is not configured")
		}
		encrypted, err := encryption.Encrypt(input.Password, config.AppConfig.NotificationEncryptionKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt password: %w", err)
		}
		s.EncryptedPassword = encrypted
	}

	if isNew {
		return database.DB.Create(&s).Error
	}
	return database.DB.Save(&s).Error
}

// TestSMTPConnection sends a test email to recipient using the stored SMTP settings.
func TestSMTPConnection(recipient string) error {
	var s models.SmtpSettings
	if err := database.DB.First(&s).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newConfigError("SMTP is not configured")
		}
		return err
	}
	if !s.Enabled {
		return newConfigError("SMTP is disabled")
	}
	if s.Host == "" {
		return newConfigError("SMTP host is not configured")
	}
	if s.FromAddress == "" {
		return newConfigError("from address is not configured")
	}

	var plainPassword string
	if s.EncryptedPassword != "" {
		if config.AppConfig.NotificationEncryptionKey == "" {
			return errors.New("NOTIFICATION_ENCRYPTION_KEY is not configured")
		}
		var err error
		plainPassword, err = encryption.Decrypt(s.EncryptedPassword, config.AppConfig.NotificationEncryptionKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt SMTP password: %w", err)
		}
	}

	return sendEmail(&s, plainPassword, recipient,
		"Watchflare - SMTP test",
		"Your SMTP configuration is working correctly.\n\nThis email was sent from Watchflare.",
	)
}

// sendEmail builds a message and delivers it via the configured SMTP server.
func sendEmail(s *models.SmtpSettings, plainPassword, recipient, subject, body string) error {
	msg := mail.NewMsg(mail.WithCharset(mail.CharsetUTF8), mail.WithNoDefaultUserAgent())

	if s.FromName != "" {
		if err := msg.FromFormat(s.FromName, s.FromAddress); err != nil {
			return fmt.Errorf("invalid from address: %w", err)
		}
	} else {
		if err := msg.From(s.FromAddress); err != nil {
			return fmt.Errorf("invalid from address: %w", err)
		}
	}
	if err := msg.To(recipient); err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, body)

	opts := []mail.Option{
		mail.WithPort(s.Port),
		mail.WithTimeout(smtpSendTimeout),
	}
	if s.HeloName != "" {
		opts = append(opts, mail.WithHELO(s.HeloName))
	}
	switch s.TLSMode {
	case TLSModeNone:
		opts = append(opts, mail.WithTLSPolicy(mail.NoTLS))
	case TLSModeStartTLS:
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	case TLSModeSSL:
		opts = append(opts, mail.WithSSL())
	}
	if s.Username != "" && plainPassword == "" {
		slog.Warn("SMTP username is configured but no password is set: sending without authentication")
	}
	if s.Username != "" && plainPassword != "" {
		authMethod := mail.SMTPAuthPlain
		if s.AuthType == SMTPAuthLogin {
			authMethod = mail.SMTPAuthLogin
		}
		opts = append(opts,
			mail.WithSMTPAuth(authMethod),
			mail.WithUsername(s.Username),
			mail.WithPassword(plainPassword),
		)
	}

	client, err := mail.NewClient(s.Host, opts...)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	return client.DialAndSend(msg)
}

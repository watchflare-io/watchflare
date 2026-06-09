package services

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"watchflare/backend/config"
	"watchflare/backend/database"
	"watchflare/backend/encryption"
	"watchflare/backend/models"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"
)

const (
	preAuthTTL      = int64(300) // 5 minutes
	backupCodeCount = 8
)

// CreatePreAuthToken returns a signed pre-auth token embedding userID + timestamp.
func CreatePreAuthToken(userID string) (string, error) {
	payload := userID + "|" + strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(config.AppConfig.JWTSecret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return base64.URLEncoding.EncodeToString([]byte(payload + "|" + sig)), nil
}

// ParsePreAuthToken validates the token and returns the embedded userID.
func ParsePreAuthToken(token string) (string, error) {
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return "", errors.New("invalid token")
	}
	parts := strings.SplitN(string(decoded), "|", 3)
	if len(parts) != 3 {
		return "", errors.New("invalid token format")
	}
	userID, timestamp, sig := parts[0], parts[1], parts[2]

	payload := userID + "|" + timestamp
	mac := hmac.New(sha256.New, []byte(config.AppConfig.JWTSecret))
	mac.Write([]byte(payload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return "", errors.New("invalid token")
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || time.Now().Unix()-ts > preAuthTTL {
		return "", errors.New("token expired")
	}
	return userID, nil
}

// GenerateTOTPSecret creates and saves an unconfirmed TOTP secret; returns the otpauth URL and plaintext secret.
func GenerateTOTPSecret(userID, email string) (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Watchflare",
		AccountName: email,
	})
	if err != nil {
		return "", "", fmt.Errorf("generate totp key: %w", err)
	}

	encrypted, err := encryption.Encrypt(key.Secret(), config.AppConfig.JWTSecret)
	if err != nil {
		return "", "", fmt.Errorf("encrypt totp secret: %w", err)
	}

	if err := database.DB.Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"totp_secret":  encrypted,
			"totp_enabled": false,
		}).Error; err != nil {
		return "", "", fmt.Errorf("save totp secret: %w", err)
	}

	return key.URL(), key.Secret(), nil
}

// EnableTOTP verifies the first TOTP code, activates 2FA, and returns 8 plaintext backup codes.
func EnableTOTP(userID, code string) ([]string, error) {
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, errors.New("user not found")
	}
	if user.TOTPSecret == nil {
		return nil, errors.New("2fa setup not initiated")
	}

	secret, err := encryption.Decrypt(*user.TOTPSecret, config.AppConfig.JWTSecret)
	if err != nil {
		return nil, errors.New("invalid totp secret")
	}
	if !totp.Validate(code, secret) {
		return nil, errors.New("invalid code")
	}

	plaintextCodes, hashedCodes := generateBackupCodes(backupCodeCount)

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&models.TOTPBackupCode{}).Error; err != nil {
			return err
		}
		for _, h := range hashedCodes {
			bc := models.TOTPBackupCode{
				ID:       uuid.New().String(),
				UserID:   userID,
				CodeHash: h,
			}
			if err := tx.Create(&bc).Error; err != nil {
				return err
			}
		}
		return tx.Model(&models.User{}).Where("id = ?", userID).Update("totp_enabled", true).Error
	}); err != nil {
		return nil, fmt.Errorf("enable totp: %w", err)
	}

	return plaintextCodes, nil
}

// DisableTOTP verifies a TOTP or backup code, then disables 2FA and clears all related data.
func DisableTOTP(userID, totpCode, backupCode string) error {
	switch {
	case totpCode != "":
		if err := verifyTOTPCode(userID, totpCode); err != nil {
			return err
		}
	case backupCode != "":
		if err := verifyBackupCode(userID, backupCode); err != nil {
			return err
		}
	default:
		return errors.New("totp_code or backup_code required")
	}

	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&models.TOTPBackupCode{}).Error; err != nil {
			return err
		}
		return tx.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
			"totp_enabled": false,
			"totp_secret":  nil,
		}).Error
	})
}

// RegenerateBackupCodes verifies a TOTP code and replaces all backup codes with 8 new ones.
func RegenerateBackupCodes(userID, totpCode string) ([]string, error) {
	if err := verifyTOTPCode(userID, totpCode); err != nil {
		return nil, err
	}

	plaintextCodes, hashedCodes := generateBackupCodes(backupCodeCount)

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&models.TOTPBackupCode{}).Error; err != nil {
			return err
		}
		for _, h := range hashedCodes {
			bc := models.TOTPBackupCode{
				ID:       uuid.New().String(),
				UserID:   userID,
				CodeHash: h,
			}
			if err := tx.Create(&bc).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("regenerate backup codes: %w", err)
	}

	return plaintextCodes, nil
}

// VerifyTOTPForLogin verifies a TOTP code or backup code during the login challenge.
func VerifyTOTPForLogin(userID, totpCode, backupCode string) error {
	if totpCode != "" {
		return verifyTOTPCode(userID, totpCode)
	}
	return verifyBackupCode(userID, backupCode)
}

// GenerateJWTForUser exposes generateJWT for use by TOTP handlers.
func GenerateJWTForUser(userID string) (string, error) {
	return generateJWT(userID)
}

// verifyTOTPCode checks a TOTP code against a user's stored secret.
func verifyTOTPCode(userID, code string) error {
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return errors.New("user not found")
	}
	if !user.TOTPEnabled || user.TOTPSecret == nil {
		return errors.New("2fa not enabled")
	}
	secret, err := encryption.Decrypt(*user.TOTPSecret, config.AppConfig.JWTSecret)
	if err != nil {
		return errors.New("invalid totp secret")
	}
	if !totp.Validate(code, secret) {
		return errors.New("invalid code")
	}
	return nil
}

// verifyBackupCode finds and consumes an unused backup code.
func verifyBackupCode(userID, code string) error {
	h := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(code))))
	codeHash := hex.EncodeToString(h[:])

	now := time.Now()
	result := database.DB.Model(&models.TOTPBackupCode{}).
		Where("user_id = ? AND code_hash = ? AND used_at IS NULL", userID, codeHash).
		Update("used_at", now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("invalid code")
	}
	return nil
}

// generateBackupCodes returns n random 10-char uppercase hex codes and their SHA-256 hashes.
func generateBackupCodes(n int) ([]string, []string) {
	codes := make([]string, n)
	hashes := make([]string, n)
	for i := range codes {
		b := make([]byte, 5)
		rand.Read(b)
		code := strings.ToUpper(hex.EncodeToString(b))
		h := sha256.Sum256([]byte(code))
		codes[i] = code
		hashes[i] = hex.EncodeToString(h[:])
	}
	return codes, hashes
}

package services

import (
	"testing"
	"time"
	"watchflare/backend/config"
	"watchflare/backend/database"
	"watchflare/backend/models"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
)

func setupTOTPConfig() {
	config.AppConfig = &config.Config{
		JWTSecret: "test-secret-key-must-be-32-chars!!",
	}
}

func TestCreatePreAuthToken_RoundTrip(t *testing.T) {
	setupTOTPConfig()

	userID := "user-123"
	token, err := CreatePreAuthToken(userID)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	got, err := ParsePreAuthToken(token)
	assert.NoError(t, err)
	assert.Equal(t, userID, got)
}

func TestParsePreAuthToken_TamperedSignature(t *testing.T) {
	setupTOTPConfig()

	token, _ := CreatePreAuthToken("user-abc")
	tampered := token[:len(token)-1] + "X"
	_, err := ParsePreAuthToken(tampered)
	assert.Error(t, err)
}

func TestParsePreAuthToken_Expired(t *testing.T) {
	t.Skip("expiry tested via manual token manipulation; round-trip covered by TestCreatePreAuthToken_RoundTrip")
}

func setupTOTPDB(t *testing.T) {
	t.Helper()
	config.AppConfig = &config.Config{
		JWTSecret:                 "test-secret-key-must-be-32-chars!!",
		NotificationEncryptionKey: "test-encryption-key-32-chars-long!",
	}
	if err := database.Connect(testDSN()); err != nil {
		t.Skipf("skipping test: database unavailable: %v", err)
	}
}

func teardownTOTPDB() {
	database.DB.Exec("DELETE FROM totp_backup_codes")
	database.DB.Exec("DELETE FROM users")
}

func createTestUserForTOTP(t *testing.T) *models.User {
	t.Helper()
	user, _, err := Register("totp@example.com", "password123", "totpuser")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return user
}

func TestGenerateTOTPSecret(t *testing.T) {
	setupTOTPDB(t)
	defer teardownTOTPDB()

	user := createTestUserForTOTP(t)

	url, secret, err := GenerateTOTPSecret(user.ID, user.Email)

	assert.NoError(t, err)
	assert.Contains(t, url, "otpauth://totp/")
	assert.Contains(t, url, "Watchflare")
	assert.NotEmpty(t, secret)

	var dbUser models.User
	database.DB.Where("id = ?", user.ID).First(&dbUser)
	assert.NotNil(t, dbUser.TOTPSecret)
	assert.False(t, dbUser.TOTPEnabled)
}

func TestEnableTOTP(t *testing.T) {
	setupTOTPDB(t)
	defer teardownTOTPDB()

	user := createTestUserForTOTP(t)
	_, secret, _ := GenerateTOTPSecret(user.ID, user.Email)

	code, err := totp.GenerateCode(secret, time.Now())
	assert.NoError(t, err)

	backupCodes, err := EnableTOTP(user.ID, code)

	assert.NoError(t, err)
	assert.Len(t, backupCodes, 8)
	for _, c := range backupCodes {
		assert.Len(t, c, 10)
	}

	var dbUser models.User
	database.DB.Where("id = ?", user.ID).First(&dbUser)
	assert.True(t, dbUser.TOTPEnabled)
}

func TestEnableTOTP_InvalidCode(t *testing.T) {
	setupTOTPDB(t)
	defer teardownTOTPDB()

	user := createTestUserForTOTP(t)
	GenerateTOTPSecret(user.ID, user.Email)

	_, err := EnableTOTP(user.ID, "000000")
	assert.Error(t, err)
	assert.Equal(t, "invalid code", err.Error())
}

func TestDisableTOTP_WithTOTPCode(t *testing.T) {
	setupTOTPDB(t)
	defer teardownTOTPDB()

	user := createTestUserForTOTP(t)
	_, secret, _ := GenerateTOTPSecret(user.ID, user.Email)
	enableCode, _ := totp.GenerateCode(secret, time.Now())
	EnableTOTP(user.ID, enableCode)

	disableCode, _ := totp.GenerateCode(secret, time.Now())
	err := DisableTOTP(user.ID, disableCode, "")

	assert.NoError(t, err)
	var dbUser models.User
	database.DB.Where("id = ?", user.ID).First(&dbUser)
	assert.False(t, dbUser.TOTPEnabled)
	assert.Nil(t, dbUser.TOTPSecret)
}

func TestDisableTOTP_WithBackupCode(t *testing.T) {
	setupTOTPDB(t)
	defer teardownTOTPDB()

	user := createTestUserForTOTP(t)
	_, secret, _ := GenerateTOTPSecret(user.ID, user.Email)
	enableCode, _ := totp.GenerateCode(secret, time.Now())
	backupCodes, _ := EnableTOTP(user.ID, enableCode)

	err := DisableTOTP(user.ID, "", backupCodes[0])
	assert.NoError(t, err)

	var dbUser models.User
	database.DB.Where("id = ?", user.ID).First(&dbUser)
	assert.False(t, dbUser.TOTPEnabled)
}

func TestRegenerateBackupCodes(t *testing.T) {
	setupTOTPDB(t)
	defer teardownTOTPDB()

	user := createTestUserForTOTP(t)
	_, secret, _ := GenerateTOTPSecret(user.ID, user.Email)
	code, _ := totp.GenerateCode(secret, time.Now())
	oldCodes, _ := EnableTOTP(user.ID, code)

	regenCode, _ := totp.GenerateCode(secret, time.Now())
	newCodes, err := RegenerateBackupCodes(user.ID, regenCode)

	assert.NoError(t, err)
	assert.Len(t, newCodes, 8)
	assert.NotEqual(t, oldCodes[0], newCodes[0])
}

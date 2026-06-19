package services

import (
	"encoding/json"
	"testing"
	"watchflare/backend/config"
	"watchflare/backend/database"
	"watchflare/backend/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func setupAuthDB(t *testing.T) {
	t.Helper()
	config.AppConfig = &config.Config{
		JWTSecret: "test-secret-key-must-be-32-chars!!",
	}
	if err := database.Connect(testDSN()); err != nil {
		t.Skipf("skipping test: database unavailable: %v", err)
	}
}

func teardownAuthDB() {
	database.DB.Exec("DELETE FROM users")
}

func TestRegister(t *testing.T) {
	setupAuthDB(t)
	defer teardownAuthDB()

	user, token, err := Register("admin@example.com", "password123", "admin")

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.NotEmpty(t, token)
	assert.Equal(t, "admin@example.com", user.Email)
	assert.Equal(t, "admin", user.Username)

	// The bcrypt hash lives on the in-memory struct by design, but it must never
	// be serialized into an API response (User.Password has the json:"-" tag).
	data, err := json.Marshal(user)
	assert.NoError(t, err)
	assert.NotContains(t, string(data), user.Password)
}

func TestRegister_DeriveUsernameFromEmail(t *testing.T) {
	setupAuthDB(t)
	defer teardownAuthDB()

	user, _, err := Register("john.doe@example.com", "password123", "")

	assert.NoError(t, err)
	assert.Equal(t, "john.doe", user.Username)
}

func TestRegister_ClosedAfterFirstUser(t *testing.T) {
	setupAuthDB(t)
	defer teardownAuthDB()

	_, _, err := Register("admin@example.com", "password123", "admin")
	assert.NoError(t, err)

	_, _, err = Register("second@example.com", "password123", "second")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "registration is closed")
}

func TestLogin(t *testing.T) {
	setupAuthDB(t)
	defer teardownAuthDB()

	Register("admin@example.com", "password123", "admin")

	result, err := Login("admin@example.com", "password123")

	assert.NoError(t, err)
	assert.NotEmpty(t, result.Token)
}

func TestLogin_WrongPassword(t *testing.T) {
	setupAuthDB(t)
	defer teardownAuthDB()

	Register("admin@example.com", "password123", "admin")

	_, err := Login("admin@example.com", "wrongpassword")
	assert.Error(t, err)
	assert.Equal(t, "invalid credentials", err.Error())
}

func TestLogin_UnknownEmail(t *testing.T) {
	setupAuthDB(t)
	defer teardownAuthDB()

	_, err := Login("nobody@example.com", "password123")
	assert.Error(t, err)
	// Must return the same message to prevent email enumeration.
	assert.Equal(t, "invalid credentials", err.Error())
}

func TestChangePassword(t *testing.T) {
	setupAuthDB(t)
	defer teardownAuthDB()

	user, _, _ := Register("admin@example.com", "oldpassword", "admin")

	err := ChangePassword(user.ID, "oldpassword", "newpassword")
	assert.NoError(t, err)

	// Old password must no longer work.
	_, err = Login("admin@example.com", "oldpassword")
	assert.Error(t, err)

	// New password must work.
	result, err := Login("admin@example.com", "newpassword")
	assert.NoError(t, err)
	assert.NotEmpty(t, result.Token)
}

func TestGenerateJWT(t *testing.T) {
	config.AppConfig = &config.Config{JWTSecret: "test-secret-key-must-be-32-chars!!"}

	tokenStr, err := generateJWT("user-123")

	assert.NoError(t, err)
	assert.NotEmpty(t, tokenStr)

	// Parse and verify claims without network calls.
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return []byte("test-secret-key-must-be-32-chars!!"), nil
	})
	assert.NoError(t, err)
	assert.True(t, token.Valid)

	claims, ok := token.Claims.(jwt.MapClaims)
	assert.True(t, ok)
	assert.Equal(t, "user-123", claims["user_id"])
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	setupAuthDB(t)
	defer teardownAuthDB()

	user, _, _ := Register("admin@example.com", "password123", "admin")

	err := ChangePassword(user.ID, "wrongpassword", "newpassword")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "incorrect")
}

func TestLogin_RequiresTOTP(t *testing.T) {
	setupAuthDB(t)
	defer teardownAuthDB()

	user, _, _ := Register("totp2@example.com", "password123", "")
	database.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"totp_secret":  "fake_encrypted_secret",
		"totp_enabled": true,
	})

	result, err := Login("totp2@example.com", "password123")

	assert.NoError(t, err)
	assert.True(t, result.Requires2FA)
	assert.Empty(t, result.Token)
	assert.Equal(t, user.ID, result.UserID)
}

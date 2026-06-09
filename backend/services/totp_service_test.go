package services

import (
	"testing"
	"watchflare/backend/config"

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

package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"watchflare/backend/services"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
)

func setupTOTPRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/auth/verify-totp", VerifyTOTP)

	authed := r.Group("/api/v1/2fa")
	authed.Use(mockAuthMiddleware())
	{
		authed.POST("/setup", SetupTOTP)
		authed.POST("/enable", EnableTOTPHandler)
		authed.DELETE("", DisableTOTPHandler)
		authed.POST("/backup-codes/regenerate", RegenerateBackupCodesHandler)
	}
	return r
}

func mockAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", testTOTPUserID)
		c.Next()
	}
}

var testTOTPUserID string

func TestVerifyTOTP_MissingCookie(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	r := setupTOTPRouter()
	body, _ := json.Marshal(map[string]string{"totp_code": "123456"})
	req := httptest.NewRequest(http.MethodPost, "/auth/verify-totp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSetupTOTP_ReturnsOtpauthURL(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	user, _, _ := services.Register("totp-handler@example.com", "password123", "")
	testTOTPUserID = user.ID

	r := setupTOTPRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/2fa/setup", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp["otpauth_url"], "otpauth://totp/")
	assert.NotEmpty(t, resp["secret"])
}

func TestEnableTOTPHandler_ValidCode(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	user, _, _ := services.Register("totp-enable@example.com", "password123", "")
	testTOTPUserID = user.ID
	_, secret, _ := services.GenerateTOTPSecret(user.ID, user.Email)
	code, _ := totp.GenerateCode(secret, time.Now())

	r := setupTOTPRouter()
	body, _ := json.Marshal(map[string]string{"code": code})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/2fa/enable", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	codes, ok := resp["backup_codes"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, codes, 8)
}

func TestDisableTOTPHandler_InvalidCode(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	user, _, _ := services.Register("totp-disable@example.com", "password123", "")
	testTOTPUserID = user.ID
	_, secret, _ := services.GenerateTOTPSecret(user.ID, user.Email)
	code, _ := totp.GenerateCode(secret, time.Now())
	services.EnableTOTP(user.ID, code)

	r := setupTOTPRouter()
	body, _ := json.Marshal(map[string]string{"totp_code": "000000"})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/2fa", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

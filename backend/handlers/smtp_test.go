package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"watchflare/backend/config"
	"watchflare/backend/database"
	"watchflare/backend/middleware"
	"watchflare/backend/models"
)

func setupSettingsRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	protected := r.Group("")
	protected.Use(middleware.AuthMiddleware())
	{
		protected.GET("/settings/smtp", GetSMTPSettings)
		protected.PUT("/settings/smtp", UpdateSMTPSettings)
		protected.POST("/settings/smtp/test", TestSMTPConnection)
	}
	return r
}

func teardownSMTPSettings() {
	database.DB.Exec("DELETE FROM smtp_settings")
	database.DB.Create(&models.SmtpSettings{Singleton: true})
}

// registerAndGetCookie registers a user and returns an authenticated cookie.
func registerAndGetCookie(t *testing.T, email string) *http.Cookie {
	t.Helper()
	authR := setupRouter()
	body, _ := json.Marshal(map[string]string{"email": email, "password": "password123"})
	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	authR.ServeHTTP(httptest.NewRecorder(), req)
	return loginAndGetCookie(t, authR, email, "password123")
}

func TestGetSMTPSettings_Defaults(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownSMTPSettings()

	r := setupSettingsRouter()
	cookie := registerAndGetCookie(t, "smtp@test.com")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/smtp", nil)
	req.AddCookie(cookie)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	smtp := resp["smtp"].(map[string]interface{})
	assert.Equal(t, float64(587), smtp["port"])
	assert.Equal(t, "starttls", smtp["tls_mode"])
	assert.Equal(t, false, smtp["enabled"])
	assert.Equal(t, false, smtp["password_set"])
}

func TestUpdateSMTPSettings(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownSMTPSettings()

	r := setupSettingsRouter()
	cookie := registerAndGetCookie(t, "smtp2@test.com")

	tests := []struct {
		name           string
		payload        map[string]interface{}
		expectedStatus int
	}{
		{
			name: "Success - valid disabled settings",
			payload: map[string]interface{}{
				"host": "smtp.example.com", "port": 587,
				"from_address": "noreply@example.com", "tls_mode": "starttls",
				"enabled": false,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Success - default port when zero",
			payload: map[string]interface{}{
				"host": "smtp.example.com", "port": 0,
				"from_address": "noreply@example.com",
				"enabled": false,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Fail - enabled without host",
			payload: map[string]interface{}{
				"host": "", "from_address": "noreply@example.com",
				"enabled": true,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Fail - enabled without from_address",
			payload: map[string]interface{}{
				"host": "smtp.example.com", "from_address": "",
				"enabled": true,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Fail - invalid tls_mode",
			payload: map[string]interface{}{
				"host": "smtp.example.com", "tls_mode": "invalid",
				"enabled": false,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Fail - invalid port",
			payload: map[string]interface{}{
				"host": "smtp.example.com", "port": 99999,
				"enabled": false,
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := json.Marshal(tt.payload)
			req, _ := http.NewRequest("PUT", "/settings/smtp", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(cookie)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestUpdateSMTPSettings_PersistsAndMasksPassword(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownSMTPSettings()

	r := setupSettingsRouter()
	cookie := registerAndGetCookie(t, "smtp3@test.com")

	// Save settings with a password
	payload := map[string]interface{}{
		"host": "smtp.example.com", "port": 587,
		"username": "user@example.com", "password": "secret",
		"from_address": "noreply@example.com", "from_name": "Watchflare",
		"tls_mode": "starttls", "enabled": false,
	}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", "/settings/smtp", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// GET should return password_set=true but no plaintext password
	req2, _ := http.NewRequest("GET", "/settings/smtp", nil)
	req2.AddCookie(cookie)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
	smtp := resp["smtp"].(map[string]interface{})
	assert.Equal(t, true, smtp["password_set"])
	assert.Nil(t, smtp["password"])
	assert.Equal(t, "smtp.example.com", smtp["host"])
	assert.Equal(t, "Watchflare", smtp["from_name"])

	// Verify the plaintext password is not stored
	var s models.SmtpSettings
	database.DB.First(&s)
	assert.NotEqual(t, "secret", s.EncryptedPassword)
	assert.NotEmpty(t, s.EncryptedPassword)
}

func TestTestSMTPConnection_NotConfigured(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownSMTPSettings()

	r := setupSettingsRouter()
	cookie := registerAndGetCookie(t, "smtp4@test.com")

	req, _ := http.NewRequest("POST", "/settings/smtp/test", bytes.NewBuffer([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should fail: SMTP is disabled by default
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp["error"])
}

func TestTestSMTPConnection_MissingNotificationEncryptionKey(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownSMTPSettings()

	r := setupSettingsRouter()
	cookie := registerAndGetCookie(t, "smtp5@test.com")

	// Save enabled SMTP settings with a password
	payload := map[string]interface{}{
		"host": "smtp.example.com", "port": 587,
		"username": "user@example.com", "password": "secret",
		"from_address": "noreply@example.com", "from_name": "Watchflare",
		"tls_mode": "starttls", "enabled": true,
	}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", "/settings/smtp", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Simulate server misconfiguration: remove the encryption key
	origKey := config.AppConfig.NotificationEncryptionKey
	config.AppConfig.NotificationEncryptionKey = ""
	defer func() { config.AppConfig.NotificationEncryptionKey = origKey }()

	req2, _ := http.NewRequest("POST", "/settings/smtp/test", bytes.NewBuffer([]byte(`{}`)))
	req2.Header.Set("Content-Type", "application/json")
	req2.AddCookie(cookie)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	// Server misconfiguration must return 500, not 400
	assert.Equal(t, http.StatusInternalServerError, w2.Code)
}

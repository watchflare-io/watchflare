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
	"watchflare/backend/database"
	"watchflare/backend/middleware"
)

func setupWebhookRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	protected := r.Group("")
	protected.Use(middleware.AuthMiddleware())
	{
		protected.GET("/settings/webhooks", GetWebhooks)
		protected.POST("/settings/webhooks", AddWebhook)
		protected.DELETE("/settings/webhooks/:id", DeleteWebhook)
		protected.PATCH("/settings/webhooks/:id/enabled", SetWebhookEnabled)
		protected.POST("/settings/webhooks/:id/test", TestWebhook)
	}
	return r
}

func teardownWebhooks() {
	database.DB.Exec("DELETE FROM webhook_endpoints")
}

// TestGetWebhooks_Empty verifies an empty list is returned when no webhooks exist.
func TestGetWebhooks_Empty(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownWebhooks()

	r := setupWebhookRouter()
	cookie := registerAndGetCookie(t, "webhooks-empty@test.com")

	req, _ := http.NewRequest("GET", "/settings/webhooks", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	webhooks, ok := resp["webhooks"].([]interface{})
	require.True(t, ok)
	assert.Empty(t, webhooks)
}

// TestAddWebhook_Discord verifies a Discord URL is detected and creates the record with no warning.
func TestAddWebhook_Discord(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownWebhooks()

	r := setupWebhookRouter()
	cookie := registerAndGetCookie(t, "webhooks-discord@test.com")

	body, _ := json.Marshal(map[string]string{
		"url": "https://discord.com/api/webhooks/123456789/abcdefghij",
	})
	req, _ := http.NewRequest("POST", "/settings/webhooks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// No warning for a known service.
	assert.Nil(t, resp["warning"])

	webhook, ok := resp["webhook"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Discord", webhook["service_name"])
	assert.NotEmpty(t, webhook["id"])
}

// TestAddWebhook_Generic_HasWarning verifies a generic URL produces a "unknown_service" warning.
func TestAddWebhook_Generic_HasWarning(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownWebhooks()

	r := setupWebhookRouter()
	cookie := registerAndGetCookie(t, "webhooks-generic@test.com")

	body, _ := json.Marshal(map[string]string{
		"url": "https://example.com/my-custom-webhook",
	})
	req, _ := http.NewRequest("POST", "/settings/webhooks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, "unknown_service", resp["warning"])

	webhook, ok := resp["webhook"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Generic", webhook["service_name"])
}

// TestAddWebhook_InvalidURL verifies that invalid URLs return 400.
func TestAddWebhook_InvalidURL(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownWebhooks()

	r := setupWebhookRouter()
	cookie := registerAndGetCookie(t, "webhooks-invalid@test.com")

	invalidURLs := []string{
		"not-a-url",
		"ftp://example.com/webhook",
		"",
	}

	for _, rawURL := range invalidURLs {
		body, _ := json.Marshal(map[string]string{"url": rawURL})
		req, _ := http.NewRequest("POST", "/settings/webhooks", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, "expected 400 for URL: %q", rawURL)
	}
}

// TestDeleteWebhook verifies that a webhook can be deleted and a second delete returns 404.
func TestDeleteWebhook(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownWebhooks()

	r := setupWebhookRouter()
	cookie := registerAndGetCookie(t, "webhooks-delete@test.com")

	// Create a webhook first.
	body, _ := json.Marshal(map[string]string{
		"url": "https://example.com/webhook-to-delete",
	})
	req, _ := http.NewRequest("POST", "/settings/webhooks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var createResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
	webhook := createResp["webhook"].(map[string]interface{})
	id := webhook["id"].(string)

	// First delete — should succeed.
	req, _ = http.NewRequest("DELETE", "/settings/webhooks/"+id, nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var deleteResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &deleteResp))
	assert.Equal(t, "webhook deleted", deleteResp["message"])

	// Second delete — should return 404.
	req, _ = http.NewRequest("DELETE", "/settings/webhooks/"+id, nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestSetWebhookEnabled verifies that the enabled flag can be toggled.
func TestSetWebhookEnabled(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownWebhooks()

	r := setupWebhookRouter()
	cookie := registerAndGetCookie(t, "webhooks-enabled@test.com")

	// Create a webhook.
	body, _ := json.Marshal(map[string]string{
		"url": "https://example.com/webhook-enabled",
	})
	req, _ := http.NewRequest("POST", "/settings/webhooks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var createResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
	webhook := createResp["webhook"].(map[string]interface{})
	id := webhook["id"].(string)

	// Disable it.
	patchBody, _ := json.Marshal(map[string]bool{"enabled": false})
	req, _ = http.NewRequest("PATCH", "/settings/webhooks/"+id+"/enabled", bytes.NewBuffer(patchBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var patchResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &patchResp))
	assert.Equal(t, "webhook updated", patchResp["message"])

	// Verify the state was actually persisted
	req3, _ := http.NewRequest("GET", "/settings/webhooks", nil)
	req3.AddCookie(cookie)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	require.Equal(t, http.StatusOK, w3.Code)
	var listResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w3.Body.Bytes(), &listResp))
	whs := listResp["webhooks"].([]interface{})
	require.Len(t, whs, 1)
	wh := whs[0].(map[string]interface{})
	assert.Equal(t, false, wh["enabled"])
}

// TestTestWebhook_Success verifies that a test notification to a mock server succeeds.
func TestTestWebhook_Success(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownWebhooks()

	// Start a mock HTTP server that accepts any request with 204 No Content.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mockServer.Close()

	r := setupWebhookRouter()
	cookie := registerAndGetCookie(t, "webhooks-test@test.com")

	// Create a webhook pointing at the mock server.
	body, _ := json.Marshal(map[string]string{
		"url": mockServer.URL + "/webhook",
	})
	req, _ := http.NewRequest("POST", "/settings/webhooks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var createResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
	webhook := createResp["webhook"].(map[string]interface{})
	id := webhook["id"].(string)

	// Test the webhook.
	req, _ = http.NewRequest("POST", "/settings/webhooks/"+id+"/test", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var testResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &testResp))
	assert.Equal(t, "test notification sent", testResp["message"])
}

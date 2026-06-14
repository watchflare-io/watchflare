package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"watchflare/backend/database"
	"watchflare/backend/middleware"
	"watchflare/backend/notifications"
)

// recordingNotifier captures Send calls without performing any network IO.
type recordingNotifier struct {
	mu       sync.Mutex
	calls    []notifierCall
	failNext error
}

type notifierCall struct {
	URL     string
	Title   string
	Message string
}

func (r *recordingNotifier) Send(_ context.Context, url, title, message string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failNext != nil {
		err := r.failNext
		r.failNext = nil
		return err
	}
	r.calls = append(r.calls, notifierCall{URL: url, Title: title, Message: message})
	return nil
}

func setupNotificationsRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	protected := r.Group("")
	protected.Use(middleware.AuthMiddleware())
	{
		protected.GET("/notifications/channels", ListNotificationChannels)
		protected.POST("/notifications/channels", CreateNotificationChannel)
		protected.GET("/notifications/channels/:id", GetNotificationChannel)
		protected.PATCH("/notifications/channels/:id", UpdateNotificationChannel)
		protected.DELETE("/notifications/channels/:id", DeleteNotificationChannel)
		protected.POST("/notifications/channels/:id/test", TestNotificationChannel)
	}
	return r
}

func setupNotificationsService(t *testing.T) *recordingNotifier {
	t.Helper()
	require.NoError(t, database.DB.AutoMigrate(&notifications.Channel{}))
	notifier := &recordingNotifier{}
	notifications.Default = notifications.NewService(
		notifications.NewRepository(database.DB),
		notifier,
		"test-encryption-key-32-chars-long!",
	)
	return notifier
}

func teardownNotificationChannels() {
	database.DB.Exec("DELETE FROM notification_channels")
}

func TestCreateNotificationChannel(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownNotificationChannels()
	setupNotificationsService(t)

	r := setupNotificationsRouter()
	cookie := registerAndGetCookie(t, "notif1@test.com")

	payload := map[string]any{
		"name": "Discord ops",
		"url":  "discord://token@123",
	}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/notifications/channels", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	ch := resp["channel"].(map[string]any)
	assert.NotEmpty(t, ch["id"])
	assert.Equal(t, "Discord ops", ch["name"])
	assert.Equal(t, "discord://***", ch["url_masked"])
	assert.Equal(t, true, ch["enabled"])
	cats := ch["categories"].([]any)
	assert.Equal(t, []any{"alerts"}, cats)
}

func TestCreateNotificationChannel_Validation(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownNotificationChannels()
	setupNotificationsService(t)

	r := setupNotificationsRouter()
	cookie := registerAndGetCookie(t, "notif2@test.com")

	cases := []struct {
		name    string
		payload map[string]any
	}{
		{"missing name", map[string]any{"url": "discord://t@1"}},
		{"missing url", map[string]any{"name": "x"}},
		{"empty name", map[string]any{"name": "   ", "url": "discord://t@1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(tc.payload)
			req, _ := http.NewRequest("POST", "/notifications/channels", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(cookie)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestListNotificationChannels_MasksURL(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownNotificationChannels()
	setupNotificationsService(t)

	r := setupNotificationsRouter()
	cookie := registerAndGetCookie(t, "notif3@test.com")

	// Insert one channel directly via the service to control the URL.
	encrypted, err := notifications.Default.EncryptURL("slack://hook:A-B-C@webhook")
	require.NoError(t, err)
	ch := &notifications.Channel{Name: "Slack", URLEncrypted: encrypted, Enabled: true}
	require.NoError(t, notifications.Default.Repo().Create(context.Background(), ch))

	req, _ := http.NewRequest("GET", "/notifications/channels", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	channels := resp["channels"].([]any)
	require.Len(t, channels, 1)
	out := channels[0].(map[string]any)
	assert.Equal(t, "slack://***", out["url_masked"])
	// Plain URL must never appear in the JSON output.
	assert.NotContains(t, w.Body.String(), "hook:A-B-C")
}

func TestUpdateNotificationChannel_PreservesURLWhenEmpty(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownNotificationChannels()
	setupNotificationsService(t)

	r := setupNotificationsRouter()
	cookie := registerAndGetCookie(t, "notif4@test.com")

	encrypted, _ := notifications.Default.EncryptURL("discord://original@1")
	ch := &notifications.Channel{Name: "Original", URLEncrypted: encrypted, Enabled: true}
	require.NoError(t, notifications.Default.Repo().Create(context.Background(), ch))

	emptyURL := ""
	patch := map[string]any{"name": "Renamed", "url": emptyURL}
	b, _ := json.Marshal(patch)
	req, _ := http.NewRequest("PATCH", "/notifications/channels/"+ch.ID, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	got, err := notifications.Default.Repo().Get(context.Background(), ch.ID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", got.Name)
	assert.Equal(t, encrypted, got.URLEncrypted, "URL must be preserved when patch URL is empty")
}

func TestDeleteNotificationChannel(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownNotificationChannels()
	setupNotificationsService(t)

	r := setupNotificationsRouter()
	cookie := registerAndGetCookie(t, "notif5@test.com")

	encrypted, _ := notifications.Default.EncryptURL("discord://t@1")
	ch := &notifications.Channel{Name: "Doomed", URLEncrypted: encrypted}
	require.NoError(t, notifications.Default.Repo().Create(context.Background(), ch))

	req, _ := http.NewRequest("DELETE", "/notifications/channels/"+ch.ID, nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	_, err := notifications.Default.Repo().Get(context.Background(), ch.ID)
	assert.ErrorIs(t, err, notifications.ErrChannelNotFound)
}

func TestTestNotificationChannel_CallsNotifier(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownNotificationChannels()
	notifier := setupNotificationsService(t)

	r := setupNotificationsRouter()
	cookie := registerAndGetCookie(t, "notif6@test.com")

	encrypted, _ := notifications.Default.EncryptURL("discord://t@1")
	ch := &notifications.Channel{Name: "Test target", URLEncrypted: encrypted, Enabled: true}
	require.NoError(t, notifications.Default.Repo().Create(context.Background(), ch))

	req, _ := http.NewRequest("POST", "/notifications/channels/"+ch.ID+"/test", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	require.Len(t, notifier.calls, 1)
	call := notifier.calls[0]
	assert.Equal(t, "discord://t@1", call.URL)
	assert.NotEmpty(t, call.Title)
	assert.NotEmpty(t, call.Message)
}

func TestTestNotificationChannel_NotFound(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownNotificationChannels()
	setupNotificationsService(t)

	r := setupNotificationsRouter()
	cookie := registerAndGetCookie(t, "notif7@test.com")

	req, _ := http.NewRequest("POST", "/notifications/channels/does-not-exist/test", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreateNotificationChannel_InvalidShoutrrrURL(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownNotificationChannels()
	setupNotificationsService(t)

	r := setupNotificationsRouter()
	cookie := registerAndGetCookie(t, "notif-invalid@test.com")

	payload := map[string]any{
		"name": "Broken",
		"url":  "https://discord.com/api/webhooks/123/token", // native URL, not shoutrrr format
	}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/notifications/channels", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["error"], "shoutrrr")
}

func TestUpdateNotificationChannel_InvalidShoutrrrURL(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownNotificationChannels()
	setupNotificationsService(t)

	r := setupNotificationsRouter()
	cookie := registerAndGetCookie(t, "notif-invalid-update@test.com")

	encrypted, _ := notifications.Default.EncryptURL("discord://t@1")
	ch := &notifications.Channel{Name: "Original", URLEncrypted: encrypted, Enabled: true}
	require.NoError(t, notifications.Default.Repo().Create(context.Background(), ch))

	bogus := "https://not-a-shoutrrr-url.example/foo"
	patch := map[string]any{"url": bogus}
	b, _ := json.Marshal(patch)
	req, _ := http.NewRequest("PATCH", "/notifications/channels/"+ch.ID, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTestNotificationChannel_Cooldown(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownNotificationChannels()
	setupNotificationsService(t)

	r := setupNotificationsRouter()
	cookie := registerAndGetCookie(t, "notif-cooldown@test.com")

	encrypted, _ := notifications.Default.EncryptURL("discord://t@1")
	ch := &notifications.Channel{Name: "Cooldown target", URLEncrypted: encrypted, Enabled: true}
	require.NoError(t, notifications.Default.Repo().Create(context.Background(), ch))

	// First test send: accepted.
	req1, _ := http.NewRequest("POST", "/notifications/channels/"+ch.ID+"/test", nil)
	req1.AddCookie(cookie)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusOK, w1.Code)

	// Second test send immediately after: rate-limited.
	req2, _ := http.NewRequest("POST", "/notifications/channels/"+ch.ID+"/test", nil)
	req2.AddCookie(cookie)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
	assert.NotEmpty(t, w2.Header().Get("Retry-After"))
}

func TestTestNotificationChannel_NotifierError(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownNotificationChannels()
	notifier := setupNotificationsService(t)

	r := setupNotificationsRouter()
	cookie := registerAndGetCookie(t, "notif8@test.com")

	encrypted, _ := notifications.Default.EncryptURL("discord://t@1")
	ch := &notifications.Channel{Name: "Will fail", URLEncrypted: encrypted, Enabled: true}
	require.NoError(t, notifications.Default.Repo().Create(context.Background(), ch))

	notifier.failNext = errors.New("simulated delivery failure")

	req, _ := http.NewRequest("POST", "/notifications/channels/"+ch.ID+"/test", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

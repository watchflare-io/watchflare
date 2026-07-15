package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"watchflare/backend/services"

	"github.com/stretchr/testify/require"
)

type capturedNotify struct {
	event      services.AccountEvent
	recipients []string
	meta       services.AccountEventMeta
}

func withNotifySpy(t *testing.T) *[]capturedNotify {
	t.Helper()
	orig := notifyAccountEvent
	captured := &[]capturedNotify{}
	notifyAccountEvent = func(e services.AccountEvent, r []string, m services.AccountEventMeta) {
		*captured = append(*captured, capturedNotify{e, r, m})
	}
	t.Cleanup(func() { notifyAccountEvent = orig })
	return captured
}

func TestLogin_NotifiesLoginEvent(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	spy := withNotifySpy(t)
	r := setupRouter()

	body, _ := json.Marshal(map[string]string{"email": "notif@test.com", "password": "password123"})
	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	loginAndGetCookie(t, r, "notif@test.com", "password123")

	require.Len(t, *spy, 1)
	require.Equal(t, services.AccountEventLogin, (*spy)[0].event)
	require.Equal(t, []string{"notif@test.com"}, (*spy)[0].recipients)
}

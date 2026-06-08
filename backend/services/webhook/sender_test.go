package webhook

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"watchflare/backend/models"
)

func TestDetect_Discord(t *testing.T) {
	s := Detect("https://discord.com/api/webhooks/123456/abcdef")
	assert.Equal(t, "Discord", s.ServiceName())
}

func TestDetect_Slack(t *testing.T) {
	s := Detect("https://hooks.slack.com/services/T000/B000/xxxx")
	assert.Equal(t, "Slack", s.ServiceName())
}

func TestDetect_Telegram(t *testing.T) {
	s := Detect("https://api.telegram.org/bot123:ABC/sendMessage?chat_id=-100")
	assert.Equal(t, "Telegram", s.ServiceName())
}

func TestDetect_Generic(t *testing.T) {
	s := Detect("https://n8n.example.com/webhook/abc")
	assert.Equal(t, "Generic", s.ServiceName())
}

func TestDetect_Generic_UnknownScheme(t *testing.T) {
	s := Detect("ntfy://ntfy.sh/topic")
	assert.Equal(t, "Generic", s.ServiceName())
}

func TestIsKnownService(t *testing.T) {
	assert.True(t, IsKnownService("https://discord.com/api/webhooks/123/abc"))
	assert.True(t, IsKnownService("https://hooks.slack.com/services/T/B/x"))
	assert.True(t, IsKnownService("https://api.telegram.org/bot123/sendMessage?chat_id=1"))
	assert.False(t, IsKnownService("https://example.com/webhook"))
	assert.False(t, IsKnownService("ntfy://ntfy.sh/topic"))
}

func TestBuildAlertContent_HostDown(t *testing.T) {
	start := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	title, body := buildAlertContent("web-01", models.MetricTypeHostDown, 0, 0, start)
	assert.Equal(t, "web-01 is offline", title)
	assert.Contains(t, body, "web-01")
}

func TestBuildAlertContent_CPU(t *testing.T) {
	start := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	title, body := buildAlertContent("web-01", models.MetricTypeCPUUsage, 90, 94.2, start)
	assert.Contains(t, title, "CPU usage")
	assert.Contains(t, body, "94.2%")
}

func TestBuildResolutionContent_HostDown(t *testing.T) {
	start := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Minute)
	title, body := buildResolutionContent("web-01", models.MetricTypeHostDown, start, end)
	assert.Equal(t, "web-01 is back online", title)
	assert.Contains(t, body, "5m0s")
}

func TestBuildResolutionContent_CPU(t *testing.T) {
	start := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	end := start.Add(12 * time.Minute)
	title, body := buildResolutionContent("web-01", models.MetricTypeCPUUsage, start, end)
	assert.Contains(t, title, "back to normal")
	assert.Contains(t, body, "12m0s")
}

package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"
	"watchflare/backend/database"
	"watchflare/backend/models"
)

// Sender delivers a notification to a single webhook endpoint.
type Sender interface {
	Detect(rawURL string) bool
	ServiceName() string
	Send(ctx context.Context, url string, event WebhookEvent) error
}

// Event type constants for WebhookEvent.Event.
const (
	EventAlert   = "alert"
	EventResolve = "resolve"
	EventTest    = "test"
)

// WebhookEvent carries both structured data (for Generic) and pre-formatted text (for chat services).
type WebhookEvent struct {
	Event          string     // EventAlert, EventResolve, or EventTest
	HostName       string
	MetricType     string
	CurrentValue   float64
	ThresholdValue float64
	StartedAt      time.Time
	ResolvedAt     *time.Time // non-nil for resolve events
	Title          string
	Body           string
}

var registry = []Sender{
	&DiscordSender{},
	&SlackSender{},
	&TelegramSender{},
	&GenericSender{}, // fallback — always matches, must be last
}

// Detect returns the appropriate Sender for rawURL.
func Detect(rawURL string) Sender {
	for _, s := range registry {
		if s.Detect(rawURL) {
			return s
		}
	}
	// unreachable: GenericSender always matches
	return &GenericSender{}
}

// IsKnownService returns true if rawURL matches a named service (not Generic).
func IsKnownService(rawURL string) bool {
	for _, s := range registry[:len(registry)-1] {
		if s.Detect(rawURL) {
			return true
		}
	}
	return false
}

// SendAll fires all enabled webhooks for an alert event. Non-blocking on error.
func SendAll(host *models.Host, metricType string, threshold, currentValue float64, startedAt time.Time) {
	endpoints := loadEnabled()
	if len(endpoints) == 0 {
		return
	}
	title, body := buildAlertContent(host.DisplayName, metricType, threshold, currentValue, startedAt)
	go dispatch(endpoints, WebhookEvent{
		Event:          EventAlert,
		HostName:       host.DisplayName,
		MetricType:     metricType,
		CurrentValue:   currentValue,
		ThresholdValue: threshold,
		StartedAt:      startedAt,
		Title:          title,
		Body:           body,
	})
}

// SendAllResolution fires all enabled webhooks for a resolution event. Non-blocking on error.
func SendAllResolution(host *models.Host, metricType string, startedAt, resolvedAt time.Time) {
	endpoints := loadEnabled()
	if len(endpoints) == 0 {
		return
	}
	title, body := buildResolutionContent(host.DisplayName, metricType, startedAt, resolvedAt)
	go dispatch(endpoints, WebhookEvent{
		Event:      EventResolve,
		HostName:   host.DisplayName,
		MetricType: metricType,
		StartedAt:  startedAt,
		ResolvedAt: &resolvedAt,
		Title:      title,
		Body:       body,
	})
}

func loadEnabled() []models.WebhookEndpoint {
	var endpoints []models.WebhookEndpoint
	if err := database.DB.Where("enabled = true").Find(&endpoints).Error; err != nil {
		slog.Error("webhook: failed to load endpoints", "error", err)
		return nil
	}
	return endpoints
}

func dispatch(endpoints []models.WebhookEndpoint, event WebhookEvent) {
	var wg sync.WaitGroup
	for _, ep := range endpoints {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			sender := Detect(ep.URL)
			if err := sender.Send(ctx, ep.URL, event); err != nil {
				slog.Error("webhook: delivery failed",
					"service", sender.ServiceName(),
					"url", sanitizeURL(ep.URL),
					"error", err,
				)
			}
		}()
	}
	wg.Wait()
}

func sanitizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "[invalid URL]"
	}
	return u.Scheme + "://" + u.Host + "/[redacted]"
}

func postJSON(ctx context.Context, rawURL string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Sanitize the URL embedded in *url.Error to prevent token leakage in logs.
		var ue *url.Error
		if errors.As(err, &ue) {
			ue.URL = sanitizeURL(ue.URL)
		}
		return fmt.Errorf("send: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

func buildAlertContent(hostName, metricType string, threshold, currentValue float64, startedAt time.Time) (title, body string) {
	switch metricType {
	case models.MetricTypeHostDown:
		title = fmt.Sprintf("%s is offline", hostName)
		body = fmt.Sprintf("Host %q has been offline since %s.", hostName, startedAt.Format(time.RFC1123))
		return
	}
	label := metricLabel(metricType)
	valueDesc := formatValue(metricType, threshold, currentValue)
	title = fmt.Sprintf("%s — %s exceeded", hostName, label)
	body = fmt.Sprintf("Alert for host %q: %s is %s\nStarted at %s.", hostName, label, valueDesc, startedAt.Format(time.RFC1123))
	return
}

func buildResolutionContent(hostName, metricType string, startedAt, resolvedAt time.Time) (title, body string) {
	duration := resolvedAt.Sub(startedAt).Round(time.Second)
	switch metricType {
	case models.MetricTypeHostDown:
		title = fmt.Sprintf("%s is back online", hostName)
		body = fmt.Sprintf("Host %q is back online.\nWas offline for %s (since %s).", hostName, duration, startedAt.Format(time.RFC1123))
		return
	}
	label := metricLabel(metricType)
	title = fmt.Sprintf("%s — %s back to normal", hostName, label)
	body = fmt.Sprintf("Alert resolved for host %q: %s is back to normal.\nDuration: %s.", hostName, label, duration)
	return
}

func metricLabel(metricType string) string {
	switch metricType {
	case models.MetricTypeCPUUsage:
		return "CPU usage"
	case models.MetricTypeMemoryUsage:
		return "Memory usage"
	case models.MetricTypeDiskUsage:
		return "Disk usage"
	case models.MetricTypeLoadAvg:
		return "Load average (1m)"
	case models.MetricTypeLoadAvg5:
		return "Load average (5m)"
	case models.MetricTypeLoadAvg15:
		return "Load average (15m)"
	case models.MetricTypeTemperature:
		return "CPU temperature"
	}
	return metricType
}

func formatValue(metricType string, threshold, currentValue float64) string {
	switch metricType {
	case models.MetricTypeCPUUsage, models.MetricTypeMemoryUsage, models.MetricTypeDiskUsage:
		return fmt.Sprintf("%.1f%% (threshold: %.0f%%)", currentValue, threshold)
	case models.MetricTypeTemperature:
		return fmt.Sprintf("%.1f°C (threshold: %.0f°C)", currentValue, threshold)
	case models.MetricTypeLoadAvg, models.MetricTypeLoadAvg5, models.MetricTypeLoadAvg15:
		return fmt.Sprintf("%.2f (threshold: %.2f)", currentValue, threshold)
	}
	return fmt.Sprintf("%.2f (threshold: %.2f)", currentValue, threshold)
}

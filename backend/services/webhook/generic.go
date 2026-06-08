package webhook

import (
	"context"
	"net/url"
	"time"
)

type GenericSender struct{}

func (g *GenericSender) ServiceName() string { return "Generic" }

func (g *GenericSender) Detect(_ string) bool { return true }

func (g *GenericSender) Send(ctx context.Context, rawURL string, event WebhookEvent) error {
	payload := map[string]any{
		"event":           event.Event,
		"host_name":       event.HostName,
		"metric_type":     event.MetricType,
		"current_value":   event.CurrentValue,
		"threshold_value": event.ThresholdValue,
		"started_at":      event.StartedAt.UTC().Format(time.RFC3339),
	}
	if event.ResolvedAt != nil {
		payload["resolved_at"] = event.ResolvedAt.UTC().Format(time.RFC3339)
	}

	// Validate it's an HTTP/HTTPS URL before attempting the request
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil // non-HTTP URLs silently skipped
	}

	return postJSON(ctx, rawURL, payload)
}

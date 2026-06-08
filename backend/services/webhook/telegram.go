package webhook

import (
	"context"
	"fmt"
	"net/url"
)

type TelegramSender struct{}

func (t *TelegramSender) ServiceName() string { return "Telegram" }

func (t *TelegramSender) Detect(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Host == "api.telegram.org"
}

// Send expects rawURL in the form:
// https://api.telegram.org/bot{token}/sendMessage?chat_id={chat_id}
// It extracts chat_id from the query string and posts all fields as JSON.
func (t *TelegramSender) Send(ctx context.Context, rawURL string, event WebhookEvent) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("telegram: invalid URL: %w", err)
	}
	chatID := parsed.Query().Get("chat_id")
	if chatID == "" {
		return fmt.Errorf("telegram: missing chat_id query parameter in URL")
	}
	parsed.RawQuery = ""
	endpoint := parsed.String()

	payload := map[string]any{
		"chat_id": chatID,
		"text":    event.Title + "\n\n" + event.Body,
	}
	return postJSON(ctx, endpoint, payload)
}

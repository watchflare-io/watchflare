package notifications

import "context"

// A channel can subscribe to one or more of these categories.
const (
	CategoryAlerts        = "alerts"
	CategoryTransactional = "transactional"
)

// Notifier delivers a single notification to a Shoutrrr URL.
type Notifier interface {
	Send(ctx context.Context, url, title, message string) error
}

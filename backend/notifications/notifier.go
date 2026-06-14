package notifications

import "context"

// Category values for NotificationChannel.Categories.
// A channel can subscribe to one or more categories of events.
const (
	CategoryAlerts        = "alerts"
	CategoryTransactional = "transactional"
)

// Notifier delivers a single notification to a Shoutrrr URL.
type Notifier interface {
	Send(ctx context.Context, url, title, message string) error
}

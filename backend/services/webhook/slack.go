package webhook

import (
	"context"
	"net/url"
)

type SlackSender struct{}

func (s *SlackSender) ServiceName() string { return "Slack" }

func (s *SlackSender) Detect(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Host == "hooks.slack.com"
}

func (s *SlackSender) Send(ctx context.Context, rawURL string, event WebhookEvent) error {
	color := "#ED4245"
	switch event.Event {
	case EventResolve:
		color = "#2EB67D"
	case EventTest:
		color = "#5865F2"
	}
	payload := map[string]any{
		"attachments": []map[string]any{
			{
				"color": color,
				"title": event.Title,
				"text":  event.Body,
			},
		},
	}
	return postJSON(ctx, rawURL, payload)
}

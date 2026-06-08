package webhook

import (
	"context"
	"net/url"
	"strings"
)

const (
	discordColorAlert   = 15548997 // #ED4245 red
	discordColorResolve = 5763719  // #57F287 green
	discordColorTest    = 5793266  // #5865F2 blurple
)

type DiscordSender struct{}

func (d *DiscordSender) ServiceName() string { return "Discord" }

func (d *DiscordSender) Detect(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Host == "discord.com" && strings.HasPrefix(u.Path, "/api/webhooks/")
}

func (d *DiscordSender) Send(ctx context.Context, rawURL string, event WebhookEvent) error {
	color := discordColorAlert
	switch event.Event {
	case EventResolve:
		color = discordColorResolve
	case EventTest:
		color = discordColorTest
	}
	payload := map[string]any{
		"embeds": []map[string]any{
			{
				"title":       event.Title,
				"description": event.Body,
				"color":       color,
			},
		},
	}
	return postJSON(ctx, rawURL, payload)
}

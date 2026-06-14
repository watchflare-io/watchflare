package notifications

import (
	"fmt"
	"net/url"
	"strings"
)

// ConvertNativeURL maps a native service webhook URL (Discord, Slack, Telegram)
// to the equivalent Shoutrrr URL. Unknown HTTP(S) URLs are wrapped as generic://.
// serviceName is one of "discord", "slack", "telegram", "generic".
func ConvertNativeURL(rawURL string) (shoutrrrURL, serviceName string, err error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", "", fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}

	host := strings.ToLower(u.Host)
	switch {
	case host == "discord.com" || host == "discordapp.com":
		return convertDiscord(u)
	case host == "hooks.slack.com":
		return convertSlack(u)
	case host == "api.telegram.org":
		return convertTelegram(u)
	default:
		return convertGeneric(u), "generic", nil
	}
}

// Discord native: https://discord.com/api/webhooks/{id}/{token}
// Shoutrrr:      discord://{token}@{id}
func convertDiscord(u *url.URL) (string, string, error) {
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// Expected: ["api", "webhooks", "{id}", "{token}"]
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "webhooks" {
		return "", "discord", fmt.Errorf("invalid discord webhook path: %s", u.Path)
	}
	id, token := parts[2], parts[3]
	if id == "" || token == "" {
		return "", "discord", fmt.Errorf("missing discord webhook id or token")
	}
	return fmt.Sprintf("discord://%s@%s", token, id), "discord", nil
}

// Slack native: https://hooks.slack.com/services/{A}/{B}/{C}
// Shoutrrr:     slack://hook:{A}-{B}-{C}@webhook
func convertSlack(u *url.URL) (string, string, error) {
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// Expected: ["services", "{A}", "{B}", "{C}"]
	if len(parts) < 4 || parts[0] != "services" {
		return "", "slack", fmt.Errorf("invalid slack webhook path: %s", u.Path)
	}
	a, b, c := parts[1], parts[2], parts[3]
	if a == "" || b == "" || c == "" {
		return "", "slack", fmt.Errorf("missing slack webhook tokens")
	}
	return fmt.Sprintf("slack://hook:%s-%s-%s@webhook", a, b, c), "slack", nil
}

// Telegram native: https://api.telegram.org/bot{TOKEN}/sendMessage?chat_id={CHAT}
// Shoutrrr:        telegram://{TOKEN}@telegram?chats={CHAT}
func convertTelegram(u *url.URL) (string, string, error) {
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// Expected: ["bot{TOKEN}", "sendMessage"]
	if len(parts) < 1 || !strings.HasPrefix(parts[0], "bot") {
		return "", "telegram", fmt.Errorf("invalid telegram url path: %s", u.Path)
	}
	token := strings.TrimPrefix(parts[0], "bot")
	chat := u.Query().Get("chat_id")
	if token == "" {
		return "", "telegram", fmt.Errorf("missing telegram bot token")
	}
	if chat == "" {
		return "", "telegram", fmt.Errorf("missing chat_id query parameter")
	}
	return fmt.Sprintf("telegram://%s@telegram?chats=%s", token, chat), "telegram", nil
}

func convertGeneric(u *url.URL) string {
	out := "generic://" + u.Host + u.Path
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	return out
}

// MaskShoutrrrURL returns a redacted form of a Shoutrrr URL safe for API
// responses and logs. Keeps the scheme; hides the rest because URLs typically
// embed tokens or passwords.
func MaskShoutrrrURL(shoutrrrURL string) string {
	scheme, _, found := strings.Cut(shoutrrrURL, "://")
	if !found || scheme == "" {
		return "***"
	}
	return scheme + "://***"
}

package notifications

import "testing"

func TestConvertNativeURL_Discord(t *testing.T) {
	in := "https://discord.com/api/webhooks/123456789/abcdef-token"
	got, svc, err := ConvertNativeURL(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc != "discord" {
		t.Errorf("expected service discord, got %s", svc)
	}
	want := "discord://abcdef-token@123456789"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestConvertNativeURL_DiscordApp(t *testing.T) {
	in := "https://discordapp.com/api/webhooks/123/token"
	got, _, err := ConvertNativeURL(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "discord://token@123"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestConvertNativeURL_Slack(t *testing.T) {
	in := "https://hooks.slack.com/services/T1AAA/B2BBB/CcccDddd"
	got, svc, err := ConvertNativeURL(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc != "slack" {
		t.Errorf("expected service slack, got %s", svc)
	}
	want := "slack://hook:T1AAA-B2BBB-CcccDddd@webhook"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestConvertNativeURL_Telegram(t *testing.T) {
	in := "https://api.telegram.org/bot1234:abc/sendMessage?chat_id=-1001234"
	got, svc, err := ConvertNativeURL(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc != "telegram" {
		t.Errorf("expected service telegram, got %s", svc)
	}
	want := "telegram://1234:abc@telegram?chats=-1001234"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestConvertNativeURL_Generic(t *testing.T) {
	in := "https://example.com/hook/abc?fmt=json"
	got, svc, err := ConvertNativeURL(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc != "generic" {
		t.Errorf("expected service generic, got %s", svc)
	}
	want := "generic://example.com/hook/abc?fmt=json"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestConvertNativeURL_Errors(t *testing.T) {
	cases := map[string]string{
		"empty":                      "",
		"non-http scheme":            "ftp://example.com/x",
		"discord missing token":      "https://discord.com/api/webhooks/123",
		"discord wrong path":         "https://discord.com/foo/bar",
		"slack wrong path":           "https://hooks.slack.com/foo/T/B/C",
		"telegram missing chat_id":   "https://api.telegram.org/bot1234:abc/sendMessage",
		"telegram missing bot token": "https://api.telegram.org/sendMessage?chat_id=1",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := ConvertNativeURL(in)
			if err == nil {
				t.Errorf("expected error for %q, got nil", in)
			}
		})
	}
}

func TestMaskShoutrrrURL(t *testing.T) {
	cases := map[string]string{
		"discord://secret-token@123":  "discord://***",
		"slack://hook:A-B-C@webhook":  "slack://***",
		"smtp://user:pass@host:25/?x": "smtp://***",
		"generic://example.com/x":     "generic://***",
		"":                            "***",
		"no-scheme":                   "***",
	}
	for in, want := range cases {
		got := MaskShoutrrrURL(in)
		if got != want {
			t.Errorf("MaskShoutrrrURL(%q) = %q, want %q", in, got, want)
		}
	}
}

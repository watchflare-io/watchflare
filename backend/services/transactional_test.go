package services

import (
	"strings"
	"testing"

	"watchflare/backend/models"

	"github.com/lib/pq"
)

func TestShouldEmailTransactional(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		cats    []string
		want    bool
	}{
		{"enabled with transactional", true, []string{"transactional"}, true},
		{"enabled with both", true, []string{"alerts", "transactional"}, true},
		{"enabled without transactional", true, []string{"alerts"}, false},
		{"enabled empty", true, []string{}, false},
		{"disabled with transactional", false, []string{"transactional"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &models.SmtpSettings{Enabled: tc.enabled, Categories: pq.StringArray(tc.cats)}
			if got := shouldEmailTransactional(s); got != tc.want {
				t.Fatalf("shouldEmailTransactional(enabled=%v, cats=%v) = %v, want %v", tc.enabled, tc.cats, got, tc.want)
			}
		})
	}
}

func TestBuildTransactionalContent(t *testing.T) {
	subj, body := buildTransactionalContent(AccountEventLogin, AccountEventMeta{IP: "203.0.113.5", UserAgent: "TestAgent/1.0"})
	if subj == "" || !strings.Contains(body, "203.0.113.5") || !strings.Contains(body, "TestAgent/1.0") {
		t.Fatalf("login content missing subject/IP/device: %q / %q", subj, body)
	}

	_, bodyNoMeta := buildTransactionalContent(AccountEventLogin, AccountEventMeta{})
	if strings.Contains(bodyNoMeta, "IP:") || strings.Contains(bodyNoMeta, "Device:") {
		t.Fatalf("login content should omit IP/Device lines when absent: %q", bodyNoMeta)
	}

	_, bodyV6 := buildTransactionalContent(AccountEventLogin, AccountEventMeta{IP: "::1"})
	if strings.Contains(bodyV6, "::1") || !strings.Contains(bodyV6, "127.0.0.1") {
		t.Fatalf("login content should normalize ::1 to 127.0.0.1: %q", bodyV6)
	}

	_, bodyOld := buildTransactionalContent(AccountEventEmailChanged, AccountEventMeta{NewEmail: "new@example.com"})
	if !strings.Contains(bodyOld, "new@example.com") {
		t.Fatalf("email-changed (old address) should mention the new address: %q", bodyOld)
	}

	for _, ev := range []AccountEvent{
		AccountEventPasswordChanged, AccountEventTOTPEnabled, AccountEventTOTPDisabled,
		AccountEventEmailChanged, AccountEventEmailChangedConfirm,
	} {
		s, b := buildTransactionalContent(ev, AccountEventMeta{})
		if s == "" || b == "" {
			t.Fatalf("event %q produced empty content: %q / %q", ev, s, b)
		}
	}
}

func TestDisplayIP(t *testing.T) {
	cases := map[string]string{
		"::1":               "127.0.0.1",
		"127.0.0.1":         "127.0.0.1",
		"::ffff:10.0.20.11": "10.0.20.11",
		"10.0.20.11":        "10.0.20.11",
		"2001:db8::1":       "2001:db8::1",
		"not-an-ip":         "not-an-ip",
	}
	for in, want := range cases {
		if got := displayIP(in); got != want {
			t.Fatalf("displayIP(%q) = %q, want %q", in, got, want)
		}
	}
}

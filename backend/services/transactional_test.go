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
	subj, body := buildTransactionalContent(AccountEventLogin, AccountEventMeta{IP: "203.0.113.5"})
	if subj == "" || !strings.Contains(body, "203.0.113.5") {
		t.Fatalf("login content missing subject or IP: %q / %q", subj, body)
	}

	_, bodyNoIP := buildTransactionalContent(AccountEventLogin, AccountEventMeta{})
	if strings.Contains(bodyNoIP, "IP ") {
		t.Fatalf("login content should omit IP clause when IP empty: %q", bodyNoIP)
	}

	for _, ev := range []AccountEvent{
		AccountEventPasswordChanged, AccountEventTOTPEnabled, AccountEventTOTPDisabled, AccountEventEmailChanged,
	} {
		s, b := buildTransactionalContent(ev, AccountEventMeta{})
		if s == "" || b == "" {
			t.Fatalf("event %q produced empty content: %q / %q", ev, s, b)
		}
	}
}

package cmd

import "testing"

func TestParseRegisterArgs_EqualSyntax(t *testing.T) {
	token, host, port, _ := parseRegisterArgs([]string{
		"--token=wf_reg_abc123",
		"--host=backend.example.com",
		"--port=50052",
	})

	if token != "wf_reg_abc123" {
		t.Errorf("token: got %q, want %q", token, "wf_reg_abc123")
	}
	if host != "backend.example.com" {
		t.Errorf("host: got %q, want %q", host, "backend.example.com")
	}
	if port != "50052" {
		t.Errorf("port: got %q, want %q", port, "50052")
	}
}

func TestParseRegisterArgs_SpaceSyntax(t *testing.T) {
	token, host, port, _ := parseRegisterArgs([]string{
		"--token", "wf_reg_abc123",
		"--host", "backend.example.com",
		"--port", "50052",
	})

	if token != "wf_reg_abc123" {
		t.Errorf("token: got %q, want %q", token, "wf_reg_abc123")
	}
	if host != "backend.example.com" {
		t.Errorf("host: got %q, want %q", host, "backend.example.com")
	}
	if port != "50052" {
		t.Errorf("port: got %q, want %q", port, "50052")
	}
}

func TestParseRegisterArgs_MixedSyntax(t *testing.T) {
	token, host, port, _ := parseRegisterArgs([]string{
		"--token=wf_reg_abc123",
		"--host", "backend.example.com",
		"--port=50052",
	})

	if token != "wf_reg_abc123" {
		t.Errorf("token: got %q, want %q", token, "wf_reg_abc123")
	}
	if host != "backend.example.com" {
		t.Errorf("host: got %q, want %q", host, "backend.example.com")
	}
	if port != "50052" {
		t.Errorf("port: got %q, want %q", port, "50052")
	}
}

func TestParseRegisterArgs_TokenOnly(t *testing.T) {
	token, host, port, _ := parseRegisterArgs([]string{"--token=wf_reg_abc123"})

	if token != "wf_reg_abc123" {
		t.Errorf("token: got %q, want %q", token, "wf_reg_abc123")
	}
	if host != "" {
		t.Errorf("host: got %q, want empty", host)
	}
	if port != "" {
		t.Errorf("port: got %q, want empty", port)
	}
}

func TestParseRegisterArgs_Empty(t *testing.T) {
	token, host, port, _ := parseRegisterArgs([]string{})

	if token != "" || host != "" || port != "" {
		t.Errorf("expected all empty, got token=%q host=%q port=%q", token, host, port)
	}
}

func TestParseRegisterArgs_UnknownFlagsIgnored(t *testing.T) {
	token, host, port, _ := parseRegisterArgs([]string{
		"--token=tok",
		"--unknown=value",
		"--verbose",
	})

	if token != "tok" {
		t.Errorf("token: got %q, want %q", token, "tok")
	}
	if host != "" || port != "" {
		t.Errorf("expected host and port empty, got host=%q port=%q", host, port)
	}
}

func TestParseRegisterArgs_SpaceSyntax_MissingValue(t *testing.T) {
	// --token at end of args with no following value: should yield empty token
	token, _, _, _ := parseRegisterArgs([]string{"--token"})

	if token != "" {
		t.Errorf("token: got %q, want empty (no value after --token)", token)
	}
}

func TestParseRegisterArgs_LastFlagWins(t *testing.T) {
	// Duplicate flags: last value wins
	token, _, _, _ := parseRegisterArgs([]string{
		"--token=first",
		"--token=second",
	})

	if token != "second" {
		t.Errorf("token: got %q, want %q (last wins)", token, "second")
	}
}

func TestParseRegisterArgs_ContainersFlag(t *testing.T) {
	_, _, _, containers := parseRegisterArgs([]string{"--token=tok", "--containers"})
	if !containers {
		t.Error("expected containers=true when --containers is passed")
	}
}

func TestParseRegisterArgs_ContainersFlagAbsent(t *testing.T) {
	_, _, _, containers := parseRegisterArgs([]string{"--token=tok"})
	if containers {
		t.Error("expected containers=false when --containers is not passed")
	}
}

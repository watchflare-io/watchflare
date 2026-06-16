package packages

import (
	"strings"
	"testing"
)

func TestParseBrewFormulaeJSON_Valid(t *testing.T) {
	output := []byte(`{
		"formulae": [
			{
				"name": "git",
				"desc": "Distributed revision control system",
				"installed": [{"version": "2.43.0"}]
			},
			{
				"name": "curl",
				"desc": "Get a file from an HTTP, HTTPS or FTP server",
				"installed": [{"version": "8.5.0"}]
			}
		],
		"casks": []
	}`)

	pkgs, err := parseBrewFormulaeJSON(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(pkgs))
	}

	if pkgs[0].Name != "git" {
		t.Errorf("name: got %q, want %q", pkgs[0].Name, "git")
	}
	if pkgs[0].Version != "2.43.0" {
		t.Errorf("version: got %q, want %q", pkgs[0].Version, "2.43.0")
	}
	if pkgs[0].PackageManager != "brew-formula" {
		t.Errorf("package manager: got %q, want %q", pkgs[0].PackageManager, "brew-formula")
	}
	if pkgs[0].Source != "homebrew/core" {
		t.Errorf("source: got %q, want %q", pkgs[0].Source, "homebrew/core")
	}
}

func TestParseBrewFormulaeJSON_MissingVersion(t *testing.T) {
	output := []byte(`{
		"formulae": [
			{"name": "noversionpkg", "desc": "A tool", "installed": []}
		],
		"casks": []
	}`)

	pkgs, err := parseBrewFormulaeJSON(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d", len(pkgs))
	}
	if pkgs[0].Version != "unknown" {
		t.Errorf("expected version %q, got %q", "unknown", pkgs[0].Version)
	}
}

func TestParseBrewFormulaeJSON_Empty(t *testing.T) {
	output := []byte(`{"formulae": [], "casks": []}`)

	pkgs, err := parseBrewFormulaeJSON(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 0 {
		t.Errorf("expected 0 packages, got %d", len(pkgs))
	}
}

func TestParseBrewFormulaeJSON_InvalidJSON(t *testing.T) {
	_, err := parseBrewFormulaeJSON([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestParseBrewFormulaeJSON_FullNameTakesPrecedence(t *testing.T) {
	output := []byte(`{
		"formulae": [
			{
				"name": "example-pkg",
				"full_name": "example/tools/example-pkg",
				"desc": "Tap-prefixed package",
				"installed": [{"version": "0.9.0"}]
			},
			{
				"name": "curl",
				"full_name": "",
				"desc": "HTTP client",
				"installed": [{"version": "8.5.0"}]
			}
		],
		"casks": []
	}`)

	pkgs, err := parseBrewFormulaeJSON(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(pkgs))
	}
	if pkgs[0].Name != "example/tools/example-pkg" {
		t.Errorf("full_name should take precedence: got %q, want %q", pkgs[0].Name, "example/tools/example-pkg")
	}
	if pkgs[1].Name != "curl" {
		t.Errorf("should fall back to name when full_name empty: got %q, want %q", pkgs[1].Name, "curl")
	}
}

func TestParseBrewFormulaeJSON_TruncatesDescription(t *testing.T) {
	long := strings.Repeat("x", 200)
	output := []byte(`{"formulae": [{"name": "pkg", "desc": "` + long + `", "installed": [{"version": "1.0"}]}], "casks": []}`)

	pkgs, err := parseBrewFormulaeJSON(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len([]rune(pkgs[0].Description)) > 100 {
		t.Errorf("description not truncated: len=%d", len([]rune(pkgs[0].Description)))
	}
}

func TestParseBrewCasksJSON_Valid(t *testing.T) {
	output := []byte(`{
		"formulae": [],
		"casks": [
			{"token": "visual-studio-code", "installed": "1.85.2", "desc": "Open-source code editor"},
			{"token": "firefox", "installed": "121.0", "desc": "Web browser"}
		]
	}`)

	pkgs, err := parseBrewCasksJSON(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(pkgs))
	}

	if pkgs[0].Name != "visual-studio-code" {
		t.Errorf("name: got %q, want %q", pkgs[0].Name, "visual-studio-code")
	}
	if pkgs[0].Version != "1.85.2" {
		t.Errorf("version: got %q, want %q", pkgs[0].Version, "1.85.2")
	}
	if pkgs[0].PackageManager != "brew-cask" {
		t.Errorf("package manager: got %q, want %q", pkgs[0].PackageManager, "brew-cask")
	}
	if pkgs[0].Source != "homebrew/cask" {
		t.Errorf("source: got %q, want %q", pkgs[0].Source, "homebrew/cask")
	}
}

func TestParseBrewCasksJSON_Empty(t *testing.T) {
	output := []byte(`{"formulae": [], "casks": []}`)

	pkgs, err := parseBrewCasksJSON(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 0 {
		t.Errorf("expected 0 packages, got %d", len(pkgs))
	}
}

func TestParseBrewCasksJSON_InvalidJSON(t *testing.T) {
	_, err := parseBrewCasksJSON([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

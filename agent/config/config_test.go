package config

import (
	"os"
	"path/filepath"
	"testing"
)

// --- GetConfigDir / GetDataDir ---

func TestGetConfigDir_Default(t *testing.T) {
	t.Setenv("WATCHFLARE_CONFIG_DIR", "")

	if got := GetConfigDir(); got != DefaultConfigDir {
		t.Errorf("got %q, want %q", got, DefaultConfigDir)
	}
}

func TestGetConfigDir_EnvOverride(t *testing.T) {
	t.Setenv("WATCHFLARE_CONFIG_DIR", "/tmp/custom-config")

	if got := GetConfigDir(); got != "/tmp/custom-config" {
		t.Errorf("got %q, want %q", got, "/tmp/custom-config")
	}
}

func TestGetDataDir_Default(t *testing.T) {
	t.Setenv("WATCHFLARE_DATA_DIR", "")

	if got := GetDataDir(); got != DefaultDataDir {
		t.Errorf("got %q, want %q", got, DefaultDataDir)
	}
}

func TestGetDataDir_EnvOverride(t *testing.T) {
	t.Setenv("WATCHFLARE_DATA_DIR", "/tmp/custom-data")

	if got := GetDataDir(); got != "/tmp/custom-data" {
		t.Errorf("got %q, want %q", got, "/tmp/custom-data")
	}
}

// --- SetDefaults ---

func TestSetDefaults_AppliesAllDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.HeartbeatInterval != DefaultHeartbeatInterval {
		t.Errorf("HeartbeatInterval: got %d, want %d", cfg.HeartbeatInterval, DefaultHeartbeatInterval)
	}
	if cfg.MetricsInterval != DefaultMetricsInterval {
		t.Errorf("MetricsInterval: got %d, want %d", cfg.MetricsInterval, DefaultMetricsInterval)
	}
	if cfg.WALEnabled == nil || !*cfg.WALEnabled {
		t.Error("WALEnabled: want true")
	}
	if cfg.WALMaxSizeMB != DefaultWALMaxSizeMB {
		t.Errorf("WALMaxSizeMB: got %d, want %d", cfg.WALMaxSizeMB, DefaultWALMaxSizeMB)
	}
	if cfg.ContainerMetrics == nil || *cfg.ContainerMetrics {
		t.Error("ContainerMetrics: want false")
	}
}

func TestSetDefaults_DoesNotOverrideExisting(t *testing.T) {
	enabled := false
	dockerEnabled := true
	cfg := &Config{
		HeartbeatInterval: 10,
		MetricsInterval:   60,
		WALEnabled:        &enabled,
		WALMaxSizeMB:      20,
		ContainerMetrics:     &dockerEnabled,
	}
	cfg.SetDefaults()

	if cfg.HeartbeatInterval != 10 {
		t.Errorf("HeartbeatInterval: got %d, want 10 (should not be overridden)", cfg.HeartbeatInterval)
	}
	if cfg.MetricsInterval != 60 {
		t.Errorf("MetricsInterval: got %d, want 60 (should not be overridden)", cfg.MetricsInterval)
	}
	if *cfg.WALEnabled {
		t.Error("WALEnabled: should not be overridden")
	}
	if cfg.WALMaxSizeMB != 20 {
		t.Errorf("WALMaxSizeMB: got %d, want 20 (should not be overridden)", cfg.WALMaxSizeMB)
	}
	if !*cfg.ContainerMetrics {
		t.Error("ContainerMetrics: should not be overridden")
	}
}

func TestSetDefaults_WALPath_UsesDataDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WATCHFLARE_DATA_DIR", dir)

	cfg := &Config{}
	cfg.SetDefaults()

	want := filepath.Join(dir, "metrics.wal")
	if cfg.WALPath != want {
		t.Errorf("WALPath: got %q, want %q", cfg.WALPath, want)
	}
}

func TestSetDefaults_WALPath_NotOverriddenWhenSet(t *testing.T) {
	cfg := &Config{WALPath: "/custom/path.wal"}
	cfg.SetDefaults()

	if cfg.WALPath != "/custom/path.wal" {
		t.Errorf("WALPath: got %q, want /custom/path.wal", cfg.WALPath)
	}
}

// --- Exists ---

func TestExists_False(t *testing.T) {
	t.Setenv("WATCHFLARE_CONFIG_DIR", t.TempDir())

	if Exists() {
		t.Error("expected false for empty directory")
	}
}

func TestExists_True(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WATCHFLARE_CONFIG_DIR", dir)

	path := filepath.Join(dir, ConfigFile)
	if err := os.WriteFile(path, []byte(""), 0640); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if !Exists() {
		t.Error("expected true when config file exists")
	}
}

// --- Save / Load roundtrip ---

func TestSaveLoad_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WATCHFLARE_CONFIG_DIR", dir)

	enabled := true
	original := &Config{
		ServerHost:        "backend.example.com",
		ServerPort:        "50051",
		AgentID:           "agent-123",
		AgentKey:          "secret-key",
		HeartbeatInterval: 10,
		MetricsInterval:   60,
		CACertFile:        "/etc/watchflare/ca.pem",
		ServerName:        "backend.watchflare.io",
		WALEnabled:        &enabled,
		WALPath:           "/var/lib/watchflare/metrics.wal",
		WALMaxSizeMB:      20,
		LogFile:           "/var/log/watchflare-agent.log",
	}

	if err := Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.ServerHost != original.ServerHost {
		t.Errorf("ServerHost: got %q, want %q", loaded.ServerHost, original.ServerHost)
	}
	if loaded.ServerPort != original.ServerPort {
		t.Errorf("ServerPort: got %q, want %q", loaded.ServerPort, original.ServerPort)
	}
	if loaded.AgentID != original.AgentID {
		t.Errorf("AgentID: got %q, want %q", loaded.AgentID, original.AgentID)
	}
	if loaded.AgentKey != original.AgentKey {
		t.Errorf("AgentKey: got %q, want %q", loaded.AgentKey, original.AgentKey)
	}
	if loaded.HeartbeatInterval != original.HeartbeatInterval {
		t.Errorf("HeartbeatInterval: got %d, want %d", loaded.HeartbeatInterval, original.HeartbeatInterval)
	}
	if loaded.MetricsInterval != original.MetricsInterval {
		t.Errorf("MetricsInterval: got %d, want %d", loaded.MetricsInterval, original.MetricsInterval)
	}
	if loaded.WALMaxSizeMB != original.WALMaxSizeMB {
		t.Errorf("WALMaxSizeMB: got %d, want %d", loaded.WALMaxSizeMB, original.WALMaxSizeMB)
	}
	if loaded.WALEnabled == nil || *loaded.WALEnabled != *original.WALEnabled {
		t.Errorf("WALEnabled: got %v, want %v", loaded.WALEnabled, original.WALEnabled)
	}
	if loaded.LogFile != original.LogFile {
		t.Errorf("LogFile: got %q, want %q", loaded.LogFile, original.LogFile)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	t.Setenv("WATCHFLARE_CONFIG_DIR", t.TempDir())

	_, err := Load()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

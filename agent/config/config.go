package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	// Default system paths (FHS compliant)
	DefaultConfigDir = "/etc/watchflare"
	DefaultDataDir   = "/var/lib/watchflare"

	// File names
	ConfigFile     = "agent.conf"
	DefaultLogFile = "/var/log/watchflare-agent.log" // matches install.LogPath

	// Default intervals (seconds)
	DefaultHeartbeatInterval = 5
	DefaultMetricsInterval   = 30

	// Default WAL settings
	DefaultWALMaxSizeMB = 10
)

// homebrewPrefix returns the Homebrew prefix if the binary is running from a
// Homebrew installation (opt/, Cellar/, or bin/ symlink), or "".
func homebrewPrefix() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	// EvalSymlinks resolves the full chain (os.Executable may return the unresolved
	// symlink path when launched via /opt/homebrew/bin/ depending on macOS version).
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
		if strings.HasPrefix(exe, prefix+"/opt/watchflare-agent/") ||
			strings.HasPrefix(exe, prefix+"/Cellar/watchflare-agent/") {
			return prefix
		}
	}
	// Match /bin/watchflare-agent only for /opt/homebrew (macOS symlink into Cellar).
	// Do NOT match /usr/local/bin/watchflare-agent — that is the standard Linux system path.
	if strings.HasPrefix(exe, "/opt/homebrew/bin/watchflare-agent") {
		return "/opt/homebrew"
	}
	return ""
}

// GetConfigDir returns the configuration directory.
// Priority: WATCHFLARE_CONFIG_DIR env var > Homebrew prefix > default system path
func GetConfigDir() string {
	if dir := os.Getenv("WATCHFLARE_CONFIG_DIR"); dir != "" {
		return dir
	}
	if prefix := homebrewPrefix(); prefix != "" {
		return filepath.Join(prefix, "etc/watchflare")
	}
	return DefaultConfigDir
}

// GetDataDir returns the data directory.
// Priority: WATCHFLARE_DATA_DIR env var > Homebrew prefix > default system path
func GetDataDir() string {
	if dir := os.Getenv("WATCHFLARE_DATA_DIR"); dir != "" {
		return dir
	}
	if prefix := homebrewPrefix(); prefix != "" {
		return filepath.Join(prefix, "var/watchflare")
	}
	return DefaultDataDir
}

// GetLogFile returns the log file path.
// Priority: WATCHFLARE_LOG_FILE env var > Homebrew prefix > default system path
func GetLogFile() string {
	if f := os.Getenv("WATCHFLARE_LOG_FILE"); f != "" {
		return f
	}
	if prefix := homebrewPrefix(); prefix != "" {
		return filepath.Join(prefix, "var/log/watchflare-agent.log")
	}
	return DefaultLogFile
}

// Config holds the agent configuration
type Config struct {
	ServerHost string `toml:"server_host"`
	ServerPort string `toml:"server_port"`
	AgentID    string `toml:"agent_id"`
	AgentKey   string `toml:"agent_key"`

	HeartbeatInterval int `toml:"heartbeat_interval"` // seconds
	MetricsInterval   int `toml:"metrics_interval"`   // seconds

	// TLS Configuration
	CACertFile string `toml:"ca_cert_file"` // Path to CA certificate for TLS
	ServerName string `toml:"server_name"`  // Server name for certificate validation

	// WAL Configuration (simplified V1)
	WALEnabled   *bool  `toml:"wal_enabled"`     // Enable WAL persistence (default: true)
	WALPath      string `toml:"wal_path"`        // WAL file path
	WALMaxSizeMB int    `toml:"wal_max_size_mb"` // Max WAL size before FIFO truncate

	// Log file path (optional — empty means stdout, captured by service manager)
	LogFile string `toml:"log_file"`

	// Log level: "debug", "info", "warn", "error" (default: "info")
	// WATCHFLARE_DEBUG=1 env var overrides this to "debug"
	LogLevel string `toml:"log_level"`

	// Container metrics (opt-in: requires access to the container runtime socket)
	ContainerMetrics *bool `toml:"container_metrics"` // Enable container runtime metrics (default: false)
}

// SetDefaults sets default values for optional configuration fields
func (c *Config) SetDefaults() {
	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = DefaultHeartbeatInterval
	}
	if c.MetricsInterval == 0 {
		c.MetricsInterval = DefaultMetricsInterval
	}

	// WAL defaults
	if c.WALEnabled == nil {
		enabled := true
		c.WALEnabled = &enabled
	}
	if c.WALPath == "" {
		c.WALPath = filepath.Join(GetDataDir(), "metrics.wal")
	}
	if c.WALMaxSizeMB == 0 {
		c.WALMaxSizeMB = DefaultWALMaxSizeMB
	}

	// Container metrics default: disabled
	if c.ContainerMetrics == nil {
		disabled := false
		c.ContainerMetrics = &disabled
	}
}

// Load reads the configuration from disk
func Load() (*Config, error) {
	configPath := filepath.Join(GetConfigDir(), ConfigFile)

	var cfg Config
	if _, err := toml.DecodeFile(configPath, &cfg); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s", configPath)
		}
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	cfg.SetDefaults()
	return &cfg, nil
}

// Save writes the configuration to disk
func Save(cfg *Config) error {
	configDir := GetConfigDir()

	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, ConfigFile)

	file, err := os.OpenFile(configPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	if err := toml.NewEncoder(file).Encode(cfg); err != nil {
		file.Close()
		return fmt.Errorf("failed to encode config: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Set proper ownership when running as root (installation/registration)
	// Linux: root:watchflare, macOS: root:staff
	if os.Geteuid() == 0 {
		var groupName string
		switch runtime.GOOS {
		case "linux":
			groupName = "watchflare"
		case "darwin":
			groupName = "staff"
		}

		if groupName != "" {
			if group, err := user.LookupGroup(groupName); err == nil {
				if gid, err := strconv.Atoi(group.Gid); err == nil {
					// Change ownership to root:group (0 = root UID)
					if err := os.Chown(configPath, 0, gid); err != nil {
						// Don't fail on chown error, just warn
						fmt.Fprintf(os.Stderr, "Warning: failed to set ownership on %s: %v\n", configPath, err)
					}
				}
			}
		}
	}

	return nil
}

// Exists checks if a configuration file already exists
func Exists() bool {
	configPath := filepath.Join(GetConfigDir(), ConfigFile)
	_, err := os.Stat(configPath)
	return err == nil
}

// EnsureDirectories creates all required directories with proper permissions.
// On macOS when running as root (e.g. sudo register), directories under a
// Homebrew prefix are chowned to SUDO_USER so the unprivileged service process
// can write to them.
func EnsureDirectories() error {
	directories := map[string]os.FileMode{
		GetConfigDir(): 0750,
		GetDataDir():   0750,
	}

	for dir, perm := range directories {
		if err := os.MkdirAll(dir, perm); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// On macOS Homebrew the service runs as the invoking user, not root.
	// When registration is run with sudo, chown data/config dirs to SUDO_USER
	// so the service process can read/write them.
	if runtime.GOOS == "darwin" && os.Geteuid() == 0 && homebrewPrefix() != "" {
		sudoUser := os.Getenv("SUDO_USER")
		if sudoUser != "" {
			u, err := user.Lookup(sudoUser)
			if err == nil {
				uid, _ := strconv.Atoi(u.Uid)
				gid, _ := strconv.Atoi(u.Gid)
				for dir := range directories {
					if err := os.Chown(dir, uid, gid); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to set ownership on %s: %v\n", dir, err)
					}
				}
			}
		}
	}

	return nil
}

//go:build darwin

package install

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	darwinTimeout        = 15 * time.Second
	homebrewServiceLabel = "homebrew.mxcl.watchflare-agent"
	// Homebrew writes logs to its var directory, not the system /var/log.
	homebrewLogPath = "/opt/homebrew/var/log/watchflare-agent.log"
)

// DarwinService implements ServiceManager for macOS (Homebrew + launchctl).
type DarwinService struct{}

// RequiresRoot returns false — brew services does not need sudo.
func (s *DarwinService) RequiresRoot() bool { return false }

// IsInstalled checks whether the agent binary is present at known Homebrew
// locations. The plist is removed by "brew services stop" so it cannot be
// used as an install indicator.
func (s *DarwinService) IsInstalled() bool {
	for _, p := range []string{
		"/usr/local/bin/" + BinaryName,    // Intel
		"/opt/homebrew/bin/" + BinaryName, // Apple Silicon
	} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	// Fallback: config file present = registered at some point
	_, err := os.Stat(ConfigDir + "/agent.conf")
	return err == nil
}

// IsRunning uses launchctl to check if the launchd job is running.
func (s *DarwinService) IsRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), darwinTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "launchctl", "list", homebrewServiceLabel).Output()
	if err != nil {
		return false
	}
	// launchctl prints "PID" only when the process is alive
	return strings.Contains(string(out), `"PID"`)
}

// Start delegates to brew services.
func (s *DarwinService) Start() error {
	return s.brewServices("start")
}

// Stop delegates to brew services.
func (s *DarwinService) Stop() error {
	return s.brewServices("stop")
}

// Restart delegates to brew services.
func (s *DarwinService) Restart() error {
	return s.brewServices("restart")
}

// ShowLogs follows the agent log file.
func (s *DarwinService) ShowLogs() error {
	fmt.Println("Following logs (Ctrl+C to exit)...")
	logPath := homebrewLogPath
	if _, err := os.Stat(logPath); err != nil {
		logPath = LogPath // fallback for non-Homebrew installs
	}
	cmd := exec.Command("tail", "-f", logPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Install is not supported — Homebrew handles installation.
func (s *DarwinService) Install() error {
	return fmt.Errorf("on macOS, install via Homebrew:\n  brew install watchflare/tap/watchflare-agent")
}

// Uninstall is not supported — Homebrew handles removal.
func (s *DarwinService) Uninstall() error {
	return fmt.Errorf("on macOS, uninstall via Homebrew:\n  brew uninstall watchflare-agent")
}

// Enable is not supported — Homebrew manages auto-start.
func (s *DarwinService) Enable() error {
	return fmt.Errorf("on macOS, service auto-start is managed by Homebrew")
}

func (s *DarwinService) brewServices(action string) error {
	brew, err := brewPath()
	if err != nil {
		return fmt.Errorf("brew services %s: brew not found in PATH (install Homebrew at https://brew.sh)", action)
	}
	ctx, cancel := context.WithTimeout(context.Background(), darwinTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, brew, "services", action, "watchflare-agent")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("brew services %s failed: %w", action, err)
	}
	return nil
}

// brewPath finds the brew binary on Intel (/usr/local/bin) and Apple Silicon (/opt/homebrew/bin).
func brewPath() (string, error) {
	if p, err := exec.LookPath("brew"); err == nil {
		return p, nil
	}
	for _, p := range []string{"/opt/homebrew/bin/brew", "/usr/local/bin/brew"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("brew not found")
}

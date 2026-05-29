//go:build linux

package install

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const systemctlTimeout = 30 * time.Second

const (
	systemdServiceFile = "/etc/systemd/system/watchflare-agent.service"
	updateServiceFile  = "/etc/systemd/system/watchflare-agent-update.service"
	updatePathFile     = "/etc/systemd/system/watchflare-agent-update.path"
	serviceName        = BinaryName
	updatePathName     = "watchflare-agent-update.path"
)

// LinuxService implements ServiceManager for Linux (systemd)
type LinuxService struct{}

// NewLinuxService creates a new Linux service manager
func NewLinuxService() *LinuxService {
	return &LinuxService{}
}

// Install installs the systemd service
func (s *LinuxService) Install() error {
	// Check if systemd is available
	if !s.hasSystemd() {
		return fmt.Errorf("systemd not available (container environment?)")
	}

	// Create service file content
	serviceContent := fmt.Sprintf(`[Unit]
Description=Watchflare Monitoring Agent
Documentation=https://watchflare.io
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=%s
Group=%s

# Binary
ExecStart=%s/%s

# Restart policy
Restart=always
RestartSec=5s

# Environment variables
Environment="WATCHFLARE_CONFIG_DIR=%s"
Environment="WATCHFLARE_DATA_DIR=%s"

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=%s %s /tmp

# Logging
StandardOutput=append:%s
StandardError=append:%s
SyslogIdentifier=%s

# Resource limits (optional)
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
`, UserName, UserName, InstallDir, BinaryName, ConfigDir, DataDir, DataDir, LogPath, LogPath, LogPath, BinaryName)

	if err := os.WriteFile(systemdServiceFile, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}
	if err := os.Chown(systemdServiceFile, 0, 0); err != nil {
		return fmt.Errorf("failed to set ownership: %w", err)
	}
	fmt.Printf("  → Installed to %s\n", systemdServiceFile)

	// Install updater service (runs as root, triggered by path unit)
	updateServiceContent := fmt.Sprintf(`[Unit]
Description=Watchflare Agent Updater
After=network.target

[Service]
Type=oneshot
ExecStart=%s/%s _apply-update
RemainAfterExit=no
`, InstallDir, BinaryName)

	if err := os.WriteFile(updateServiceFile, []byte(updateServiceContent), 0644); err != nil {
		return fmt.Errorf("failed to write update service file: %w", err)
	}
	if err := os.Chown(updateServiceFile, 0, 0); err != nil {
		return fmt.Errorf("failed to set update service ownership: %w", err)
	}
	fmt.Printf("  → Installed to %s\n", updateServiceFile)

	// Install path unit (watches for update-pending trigger file)
	updatePathContent := fmt.Sprintf(`[Unit]
Description=Watch for Watchflare Agent Update Requests
Wants=watchflare-agent-update.service

[Path]
PathExists=%s/update-pending
Unit=watchflare-agent-update.service

[Install]
WantedBy=multi-user.target
`, DataDir)

	if err := os.WriteFile(updatePathFile, []byte(updatePathContent), 0644); err != nil {
		return fmt.Errorf("failed to write update path file: %w", err)
	}
	if err := os.Chown(updatePathFile, 0, 0); err != nil {
		return fmt.Errorf("failed to set update path ownership: %w", err)
	}
	fmt.Printf("  → Installed to %s\n", updatePathFile)

	// Reload systemd
	ctx, cancel := context.WithTimeout(context.Background(), systemctlTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}
	fmt.Println("  → Systemd daemon reloaded")

	// Enable path unit (auto-starts on boot, watches for trigger file)
	enableCtx, enableCancel := context.WithTimeout(context.Background(), systemctlTimeout)
	defer enableCancel()
	enableCmd := exec.CommandContext(enableCtx, "systemctl", "enable", "--now", updatePathName)
	if err := enableCmd.Run(); err != nil {
		return fmt.Errorf("failed to enable update path unit: %w", err)
	}
	fmt.Printf("  → Enabled %s\n", updatePathName)

	return nil
}

// Uninstall removes the systemd service
func (s *LinuxService) Uninstall() error {
	if !s.hasSystemd() {
		return nil // Nothing to do
	}

	// Stop service if running
	if s.IsRunning() {
		if err := s.Stop(); err != nil {
			return err
		}
	}

	// Disable main service and path unit — errors intentionally ignored
	disableCtx, disableCancel := context.WithTimeout(context.Background(), systemctlTimeout)
	exec.CommandContext(disableCtx, "systemctl", "disable", "--now", updatePathName).Run() //nolint:errcheck
	exec.CommandContext(disableCtx, "systemctl", "disable", serviceName).Run()             //nolint:errcheck
	disableCancel()

	// Remove unit files
	for _, f := range []string{systemdServiceFile, updateServiceFile, updatePathFile} {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %w", f, err)
		}
	}

	// Reload systemd
	reloadCtx, reloadCancel := context.WithTimeout(context.Background(), systemctlTimeout)
	defer reloadCancel()
	cmd := exec.CommandContext(reloadCtx, "systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	fmt.Println("  → Removed systemd service")
	return nil
}

// Start starts the service
func (s *LinuxService) Start() error {
	if !s.hasSystemd() {
		return fmt.Errorf("systemd not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), systemctlTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemctl", "start", serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	fmt.Println("  → Service started")
	return nil
}

// Stop stops the service
func (s *LinuxService) Stop() error {
	if !s.hasSystemd() {
		return fmt.Errorf("systemd not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), systemctlTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemctl", "stop", serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	fmt.Println("  → Service stopped")
	return nil
}

// Enable enables the service to start on boot
func (s *LinuxService) Enable() error {
	if !s.hasSystemd() {
		return fmt.Errorf("systemd not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), systemctlTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemctl", "enable", serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	fmt.Println("  → Service enabled (will start on boot)")
	return nil
}

// IsInstalled checks if the service is installed
func (s *LinuxService) IsInstalled() bool {
	_, err := os.Stat(systemdServiceFile)
	return err == nil
}

// IsRunning checks if the service is running
func (s *LinuxService) IsRunning() bool {
	if !s.hasSystemd() {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), systemctlTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", serviceName)
	return cmd.Run() == nil
}

// Restart restarts the service
func (s *LinuxService) Restart() error {
	if !s.hasSystemd() {
		return fmt.Errorf("systemd not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), systemctlTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemctl", "restart", serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restart service: %w", err)
	}

	fmt.Println("  → Service restarted")
	return nil
}

// ShowLogs displays and follows the service logs
func (s *LinuxService) ShowLogs() error {
	if !s.hasSystemd() {
		return fmt.Errorf("systemd not available")
	}

	fmt.Println("Following logs (Ctrl+C to exit)...")

	cmd := exec.Command("journalctl", "-u", serviceName, "-f", "--no-pager")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RequiresRoot returns true — systemctl requires root on Linux.
func (s *LinuxService) RequiresRoot() bool { return true }

// hasSystemd checks if systemd is available.
// Accepts both "running" and "degraded" states — a degraded system (some units
// failed) still supports service management commands.
func (s *LinuxService) hasSystemd() bool {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), systemctlTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemctl", "is-system-running")
	output, _ := cmd.Output()
	state := strings.TrimSpace(string(output))
	return state == "running" || state == "degraded"
}

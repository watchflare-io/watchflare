package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"watchflare-agent/client"
	"watchflare-agent/config"
	"watchflare-agent/install"
	"watchflare-agent/sysinfo"
	"watchflare-agent/uuid"
)

// AgentVersion is set by main.go from the build-time Version variable
var AgentVersion = "dev"

// Register handles agent registration with the backend
func Register() {
	fmt.Println("Watchflare Agent Registration")
	fmt.Println("==============================")

	token, host, port, containers := parseRegisterArgs(os.Args[2:])

	if token == "" {
		fmt.Fprintln(os.Stderr, "error: --token is required\nUsage: watchflare-agent register --token=TOKEN [--host=HOST] [--port=PORT] [--containers]")
		os.Exit(1)
	}
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "50051"
	}

	reactivated, err := runRegistration(token, host, port, containers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("✅ Registration successful!")
	if reactivated {
		fmt.Println("⚠️  NOTICE: This agent was merged with an existing agent in the system")
		fmt.Printf("   Reason: Agent UUID was found on disk (%s)\n", uuid.GetUUIDPath())
		fmt.Println("   This is the same physical host reconnecting, so the existing agent was reactivated")
		fmt.Println("   If you intended to create a NEW agent, uninstall with data cleanup first")
	}

	if containers {
		if runtime.GOOS == "linux" && os.Geteuid() != 0 {
			fmt.Println()
			fmt.Println("⚠ Container metrics enabled in config, but docker group setup requires root.")
			fmt.Println("  Run the following to complete setup:")
			fmt.Println("  sudo usermod -aG docker watchflare")
			fmt.Println("  sudo systemctl restart watchflare-agent")
		} else {
			if err := install.AddToDockerGroup(); err != nil {
				fmt.Fprintf(os.Stderr, "\nWarning: %v\n", err)
				fmt.Println("  Run manually: sudo usermod -aG docker watchflare")
			}
		}
	}

	if isInstalledViaBrew() {
		fmt.Println("\nYou can now start the agent with: brew services start watchflare-agent")
	} else if runtime.GOOS == "linux" {
		fmt.Println("\nYou can now start the agent with: sudo systemctl enable --now watchflare-agent")
	} else {
		fmt.Println("\nYou can now start the agent with: ./watchflare-agent")
	}
}

// runRegistration performs agent registration with the backend.
// Called by Register() (standalone command) and Install() (inline during installation).
func runRegistration(token, host, port string, containers bool) (bool, error) {
	if err := config.EnsureDirectories(); err != nil {
		return false, fmt.Errorf("failed to create directories: %w", err)
	}

	slog.Info("collecting system information")
	info, err := sysinfo.Collect()
	if err != nil {
		return false, fmt.Errorf("failed to collect system info: %w", err)
	}

	slog.Info("system info",
		"hostname", info.Hostname,
		"platform", info.Platform+" "+info.PlatformVersion,
		"arch", info.KernelArch,
		"ipv4", info.IPv4Address)
	if info.IPv6Address != "" {
		slog.Info("IPv6 detected", "ipv6", info.IPv6Address)
	}

	slog.Info("connecting to backend", "host", host, "port", port)
	grpcClient, err := client.NewForRegistration(host, port)
	if err != nil {
		return false, fmt.Errorf("failed to connect to backend: %w", err)
	}
	defer grpcClient.Close()

	env := sysinfo.DetectEnvironment()
	slog.Info("environment detected", "type", env.String())

	existingUUID, err := uuid.Load()
	if err != nil {
		slog.Warn("failed to load existing UUID", "error", err)
		existingUUID = ""
	}
	if existingUUID != "" {
		slog.Info("found existing agent UUID, will reactivate if still valid", "agent_id", existingUUID)
	}

	slog.Info("registering agent")
	regResp, err := grpcClient.Register(client.RegisterRequest{
		Token:                token,
		Hostname:             info.Hostname,
		IPv4:                 info.IPv4Address,
		IPv6:                 info.IPv6Address,
		OS:                   info.OS,
		Platform:             info.Platform,
		PlatformFamily:       info.PlatformFamily,
		PlatformVersion:      info.PlatformVersion,
		KernelVersion:        info.KernelVersion,
		KernelArch:           info.KernelArch,
		VirtualizationSystem: info.VirtualizationSystem,
		VirtualizationRole:   info.VirtualizationRole,
		HostID:               info.HostID,
		EnvironmentType:      string(env.Type),
		ContainerRuntime:     env.ContainerRuntime,
		CPUModelName:         info.CPUModelName,
		CPUPhysicalCount:     int32(info.CPUPhysicalCount),
		CPULogicalCount:      int32(info.CPULogicalCount),
		CPUMhz:               info.CPUMhz,
		ExistingUUID:         existingUUID,
		AgentVersion:         AgentVersion,
	})
	if err != nil {
		return false, fmt.Errorf("registration failed: %w", err)
	}

	caCertPath := filepath.Join(config.GetConfigDir(), "ca.pem")
	slog.Info("saving CA certificate", "path", caCertPath)
	if err := client.SaveCACertificate(regResp.CACert, caCertPath); err != nil {
		return false, fmt.Errorf("failed to save CA certificate: %w", err)
	}

	cfg := &config.Config{
		ServerHost: host,
		ServerPort: port,
		AgentID:    regResp.AgentID,
		AgentKey:   regResp.AgentKey,
		CACertFile: caCertPath,
		ServerName: regResp.ServerName,
		LogFile:    config.GetLogFile(),
	}
	if containers {
		enabled := true
		cfg.ContainerMetrics = &enabled
	}
	cfg.SetDefaults()

	slog.Info("saving configuration")
	if err := config.Save(cfg); err != nil {
		return false, fmt.Errorf("failed to save config: %w", err)
	}

	slog.Info("saving agent UUID")
	if err := uuid.Save(regResp.AgentID); err != nil {
		slog.Warn("failed to save UUID", "error", err)
		// Not fatal — agent will work, but will create new UUID on next registration
	}

	slog.Info("agent registered",
		"agent_id", regResp.AgentID,
		"config", filepath.Join(config.GetConfigDir(), config.ConfigFile),
		"tls_server", regResp.ServerName)

	return regResp.Reactivated, nil
}

// parseRegisterArgs parses --token, --host, --port, --containers from a slice of arguments.
// Supports both --flag=value and --flag value forms.
func parseRegisterArgs(args []string) (token, host, port string, containers bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case strings.HasPrefix(arg, "--token="):
			token = strings.TrimPrefix(arg, "--token=")
		case arg == "--token" && i+1 < len(args):
			i++
			token = args[i]
		case strings.HasPrefix(arg, "--host="):
			host = strings.TrimPrefix(arg, "--host=")
		case arg == "--host" && i+1 < len(args):
			i++
			host = args[i]
		case strings.HasPrefix(arg, "--port="):
			port = strings.TrimPrefix(arg, "--port=")
		case arg == "--port" && i+1 < len(args):
			i++
			port = args[i]
		case arg == "--containers":
			containers = true
		}
	}
	return
}

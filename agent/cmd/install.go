package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"watchflare-agent/config"
	"watchflare-agent/install"
)

const installLogReadLimit = 1 * 1024 * 1024 // 1 MB

// Install handles agent installation
func Install() {
	if runtime.GOOS == "darwin" {
		fmt.Println("On macOS, the agent is installed and managed via Homebrew:")
		fmt.Println()
		fmt.Println("  brew tap watchflare-io/watchflare")
		fmt.Println("  brew install watchflare-agent")
		fmt.Println("  watchflare-agent register --token=YOUR_TOKEN --host=YOUR_HOST")
		fmt.Println("  brew services start watchflare-agent")
		return
	}

	fmt.Println("=== Watchflare Agent Installation ===")
	fmt.Println()

	fmt.Println("[1/7] Checking permissions...")
	if err := install.CheckRoot(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  → Running as root")

	// Parse command line arguments (supports both --flag=value and --flag value)
	token, host, port, containers := parseRegisterArgs(os.Args[2:])

	svcMgr, err := install.GetServiceManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if svcMgr.IsInstalled() {
		fmt.Println("  → Found existing installation")
		if svcMgr.IsRunning() {
			fmt.Println("  → Stopping existing service...")
			if err := svcMgr.Stop(); err != nil {
				fmt.Printf("Warning: failed to stop service: %v\n", err)
			}
			time.Sleep(time.Second)
		}
	}

	fmt.Println("\n[2/7] Creating system user...")
	if err := install.CreateUser(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if containers {
		if err := install.AddToDockerGroup(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			fmt.Println("  → Container metrics will not work until the docker group is set up")
		}
	}

	fmt.Println("\n[3/7] Creating directories...")
	if err := install.CreateDirectories(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n[4/7] Installing binary...")

	binaryPath, err := install.GetBinaryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get binary path: %v\n", err)
		os.Exit(1)
	}

	if err := install.InstallBinary(binaryPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := install.CreateLogFile(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n[5/7] Installing service...")
	if err := svcMgr.Install(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n[6/7] Agent registration...")
	needsRegistration := true

	configPath := filepath.Join(install.ConfigDir, config.ConfigFile)
	if _, err := os.Stat(configPath); err == nil {
		fmt.Println("  → Configuration file already exists")
		needsRegistration = false
	} else if token != "" {
		fmt.Println("  → Registering agent with backend...")

		if host == "" {
			host = "localhost"
		}
		if port == "" {
			port = "50051"
		}

		reactivated, err := runRegistration(token, host, port, containers)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: registration failed: %v\n", err)
			os.Exit(1)
		}
		needsRegistration = false

		if reactivated {
			fmt.Println("  → Registration successful (existing agent reactivated)")
			fmt.Println("  ⚠️  NOTICE: Agent UUID was found on disk - merged with existing agent")
		} else {
			fmt.Println("  → Registration successful")
		}
	} else {
		fmt.Println("  ⚠ No configuration file found")
		fmt.Println("  → To register now, run:")
		fmt.Printf("     sudo %s/watchflare-agent register --token=YOUR_TOKEN --host=YOUR_HOST\n", install.InstallDir)
	}

	fmt.Println("\n[7/7] Starting service...")
	if !needsRegistration {
		if err := svcMgr.Enable(); err != nil {
			fmt.Printf("Warning: %v\n", err)
		}

		if err := svcMgr.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		time.Sleep(2 * time.Second)

		if svcMgr.IsRunning() {
			fmt.Println("  → Service started successfully")

			fmt.Print("  → Checking agent health...")
			time.Sleep(8 * time.Second)
			logContent, err := func() ([]byte, error) {
				f, err := os.Open(install.LogPath)
				if err != nil {
					return nil, err
				}
				defer f.Close()
				return io.ReadAll(io.LimitReader(f, installLogReadLimit))
			}()
			if err == nil && hasClockSyncError(string(logContent)) {
				fmt.Println(" ⚠")
				fmt.Println()
				fmt.Println("  ⚠ WARNING: Clock synchronization error detected!")
				fmt.Println("  The system clock is out of sync with the backend (>5min difference).")
				fmt.Println("  Ensure the system clock is synchronized and restart the agent.")
			} else {
				fmt.Println(" ✓")
			}
		} else {
			fmt.Println("  → Service failed to start")
			fmt.Printf("  → Check logs: tail -f %s\n", install.LogPath)
		}
	} else {
		fmt.Println("  → Skipped (needs registration first)")
	}

	fmt.Println("\n=== Installation Complete ===")
	fmt.Println()
	fmt.Println("Installation paths:")
	fmt.Printf("  Binary:        %s/watchflare-agent\n", install.InstallDir)
	fmt.Printf("  Configuration: %s/\n", install.ConfigDir)
	fmt.Printf("  Data:          %s/\n", install.DataDir)
	fmt.Printf("  Logs:          %s\n", install.LogPath)
	fmt.Println()

	if needsRegistration {
		fmt.Println("Next steps:")
		fmt.Println("  1. Register the agent:")
		fmt.Printf("     sudo %s/watchflare-agent register --token=YOUR_TOKEN --host=YOUR_HOST\n", install.InstallDir)
		fmt.Println()
		fmt.Println("  2. Start the service:")
		fmt.Println("     sudo systemctl enable watchflare-agent")
		fmt.Println("     sudo systemctl start watchflare-agent")
		fmt.Println()
	} else {
		if token != "" {
			fmt.Println("Registration details:")
			fmt.Printf("  Backend: %s:%s\n", host, port)
			fmt.Println()
		}

		fmt.Println("Service management:")
		fmt.Println("  Status:  sudo systemctl status watchflare-agent")
		fmt.Println("  Stop:    sudo systemctl stop watchflare-agent")
		fmt.Println("  Start:   sudo systemctl start watchflare-agent")
		fmt.Println("  Restart: sudo systemctl restart watchflare-agent")
		fmt.Printf("  Logs:    tail -f %s\n", install.LogPath)
		fmt.Println()
	}

	fmt.Println("Installation successful!")
}

// hasClockSyncError returns true if the log content indicates a clock
// synchronization error between the agent and the backend.
func hasClockSyncError(logContent string) bool {
	return strings.Contains(logContent, "clock out of sync with backend")
}

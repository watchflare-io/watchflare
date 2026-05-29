package cmd

import (
	"fmt"
	"os"

	"watchflare-agent/install"
)

// Status displays the current status of the agent service
func Status() {
	svcMgr, err := install.GetServiceManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Watchflare Agent Status ===")
	fmt.Println()

	if !svcMgr.IsInstalled() {
		fmt.Println("Status: Not installed")
		fmt.Println()
		fmt.Println("To install the agent, run:")
		fmt.Println("  sudo watchflare-agent install --token=YOUR_TOKEN")
		return
	}

	fmt.Println("Installation: ✓ Installed")

	if svcMgr.IsRunning() {
		fmt.Println("Status:       ✓ Running")
	} else {
		fmt.Println("Status:       ✗ Stopped")
	}

	fmt.Println()
	fmt.Println("Paths:")
	fmt.Printf("  Binary:        %s/%s\n", install.InstallDir, install.BinaryName)
	fmt.Printf("  Configuration: %s/\n", install.ConfigDir)
	fmt.Printf("  Data:          %s/\n", install.DataDir)
	fmt.Printf("  Logs:          %s\n", install.LogPath)
}

// StartService starts the agent service
func StartService() {
	svcMgr, err := install.GetServiceManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if svcMgr.RequiresRoot() {
		if err := install.CheckRoot(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	if !svcMgr.IsInstalled() {
		fmt.Fprintln(os.Stderr, "Error: agent is not installed. Run 'sudo watchflare-agent install' first.")
		os.Exit(1)
	}

	if svcMgr.IsRunning() {
		fmt.Println("Agent is already running")
		return
	}

	if err := svcMgr.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Agent started successfully")
}

// StopService stops the agent service
func StopService() {
	svcMgr, err := install.GetServiceManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if svcMgr.RequiresRoot() {
		if err := install.CheckRoot(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	if !svcMgr.IsInstalled() {
		fmt.Fprintln(os.Stderr, "Error: agent is not installed")
		os.Exit(1)
	}

	if !svcMgr.IsRunning() {
		fmt.Println("Agent is already stopped")
		return
	}

	if err := svcMgr.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Agent stopped successfully")
}

// RestartService restarts the agent service
func RestartService() {
	svcMgr, err := install.GetServiceManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if svcMgr.RequiresRoot() {
		if err := install.CheckRoot(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	if !svcMgr.IsInstalled() {
		fmt.Fprintln(os.Stderr, "Error: agent is not installed")
		os.Exit(1)
	}

	if err := svcMgr.Restart(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Agent restarted successfully")
}

// Logs displays and follows the agent logs
func Logs() {
	svcMgr, err := install.GetServiceManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := svcMgr.ShowLogs(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"os"

	"watchflare-agent/cmd"
	"watchflare-agent/logger"
)

func main() {
	logger.Init()

	// Check for subcommands
	if len(os.Args) > 1 {
		subcommand := os.Args[1]

		switch subcommand {
		case "install":
			cmd.AgentVersion = Version
			cmd.Install()
			return

		case "uninstall":
			cmd.Uninstall()
			return

		case "register":
			cmd.AgentVersion = Version
			cmd.Register()
			return

		case "status":
			cmd.Status()
			return

		case "start":
			cmd.StartService()
			return

		case "stop":
			cmd.StopService()
			return

		case "restart":
			cmd.RestartService()
			return

		case "logs":
			cmd.Logs()
			return

		case "update":
			cmd.AgentVersion = Version
			cmd.Update()
			return

		case "_apply-update":
			cmd.ApplyUpdate()
			return

		case "help", "-h", "--help":
			printHelp()
			return

		case "version", "-v", "--version":
			printVersion()
			return

		default:
			fmt.Printf("Unknown command: %s\n\n", subcommand)
			printHelp()
			os.Exit(1)
		}
	}

	// No subcommand = run normal agent
	cmd.AgentVersion = Version
	cmd.Run()
}

func printHelp() {
	fmt.Println("Watchflare Agent - Host Monitoring Agent")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  watchflare-agent <command> [options]")
	fmt.Println()
	fmt.Println("Installation & Setup:")
	fmt.Println("  install    --token=TOKEN [--host=HOST] [--port=PORT] [--containers]  Install and register the agent")
	fmt.Println("  uninstall                                                             Remove the agent")
	fmt.Println("  register   --token=TOKEN [--host=HOST] [--port=PORT] [--containers]  Re-register with a new backend")
	fmt.Println()
	fmt.Println("Service Control:")
	fmt.Println("  status     Show agent status")
	fmt.Println("  start      Start the agent service")
	fmt.Println("  stop       Stop the agent service")
	fmt.Println("  restart    Restart the agent service")
	fmt.Println("  logs       Follow agent logs")
	fmt.Println()
	fmt.Println("Updates:")
	fmt.Println("  update           Update to the latest version")
	fmt.Println("  update --check   Check for updates without installing")
	fmt.Println()
	fmt.Println("Other:")
	fmt.Println("  (no args)  Run agent in foreground")
	fmt.Println("  version    Show version information")
	fmt.Println("  help       Show this help message")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  sudo watchflare-agent install --token=wf_reg_xxx --host=monitor.example.com")
	fmt.Println()
}

// Version is set at build time via ldflags: -X 'main.Version=...'
var Version = "dev"

func printVersion() {
	fmt.Printf("Watchflare Agent v%s\n", Version)
	fmt.Println("https://watchflare.io")
}

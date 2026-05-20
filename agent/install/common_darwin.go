//go:build darwin

package install

import "fmt"

// GetServiceManager is not supported on macOS — the agent is managed via Homebrew.
func GetServiceManager() (ServiceManager, error) {
	return nil, fmt.Errorf("on macOS, use Homebrew to manage the agent:\n" +
		"  brew services [start|stop|restart] watchflare-agent")
}

// CreateUser is not supported on macOS — manual installation is not yet implemented.
func CreateUser() error {
	return fmt.Errorf("manual installation is not supported on macOS, use Homebrew")
}

// AddToDockerGroup is a no-op on macOS: the agent runs as the invoking user
// who already has access to the Docker socket via Docker Desktop.
func AddToDockerGroup() error {
	return nil
}

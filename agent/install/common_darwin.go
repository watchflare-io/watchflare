//go:build darwin

package install

import "fmt"

// GetServiceManager returns a macOS service manager backed by Homebrew and launchctl.
func GetServiceManager() (ServiceManager, error) {
	return &DarwinService{}, nil
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

//go:build linux

package install

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
)

// GetServiceManager returns the Linux service manager
func GetServiceManager() (ServiceManager, error) {
	return NewLinuxService(), nil
}

// CreateUser creates the watchflare system user
func CreateUser() error {
	// Check if user already exists
	if _, err := user.Lookup(UserName); err == nil {
		fmt.Printf("  → User '%s' already exists\n", UserName)
		return nil
	}

	return createUserLinux(UserName)
}

// createUserLinux creates a system user on Linux
func createUserLinux(username string) error {
	// Create group first
	groupCtx, groupCancel := context.WithTimeout(context.Background(), execTimeout)
	defer groupCancel()
	cmd := exec.CommandContext(groupCtx, "groupadd", "--system", username)
	if err := cmd.Run(); err != nil {
		// Ignore if group already exists
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != exitCodeAlreadyExists {
			return fmt.Errorf("failed to create group: %w", err)
		}
	}

	// Create user
	userCtx, userCancel := context.WithTimeout(context.Background(), execTimeout)
	defer userCancel()
	cmd = exec.CommandContext(userCtx, "useradd",
		"--system",
		"--gid", username,
		"--home-dir", "/var/empty",
		"--shell", "/usr/sbin/nologin",
		"--comment", "Watchflare Agent",
		username,
	)

	if err := cmd.Run(); err != nil {
		// Ignore if user already exists
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != exitCodeAlreadyExists {
			return fmt.Errorf("failed to create user: %w", err)
		}
	}

	fmt.Printf("  → Created user '%s'\n", username)
	return nil
}

// AddToDockerGroup adds the watchflare user to the docker group so the agent
// can access the Docker socket and collect container metrics.
func AddToDockerGroup() error {
	if _, err := user.LookupGroup("docker"); err != nil {
		return fmt.Errorf("docker group not found: is Docker installed?")
	}

	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "usermod", "-aG", "docker", UserName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add %s to docker group: %w\n%s", UserName, err, out)
	}

	fmt.Printf("  → Added '%s' to the docker group\n", UserName)
	return nil
}

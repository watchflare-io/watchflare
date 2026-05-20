package sysinfo

import (
	"io"
	"os"
	"runtime"
	"strings"
)

const (
	cgroupReadLimit = 64 * 1024 // 64 KB
	dmiReadLimit    = 4 * 1024  // 4 KB — DMI values are always tiny
)

// EnvironmentType represents the type of environment where the agent runs
type EnvironmentType string

const (
	EnvPhysical                EnvironmentType = "physical"                  // Bare metal host
	EnvPhysicalWithContainers  EnvironmentType = "physical_with_containers"  // Physical host running containers
	EnvVM                      EnvironmentType = "vm"                        // Virtual machine
	EnvVMWithContainers        EnvironmentType = "vm_with_containers"        // VM running containers
	EnvContainer               EnvironmentType = "container"                 // Inside a container
)

// Environment holds information about the runtime environment
type Environment struct {
	Type                 EnvironmentType
	IsPhysical           bool
	IsVM                 bool
	IsContainer          bool
	HasContainerRuntime  bool
	ContainerRuntime     string // "docker", "lxc", "podman", etc.
	Hypervisor           string // "kvm", "vmware", "virtualbox", "hyperv", "xen", etc.
}

// DetectEnvironment detects the type of environment the agent is running in
func DetectEnvironment() *Environment {
	env := &Environment{}

	// 1. Detect if running inside a container
	env.IsContainer = isRunningInContainer()
	if env.IsContainer {
		env.ContainerRuntime = detectContainerRuntime()
	}

	// 2. Detect if running in a VM
	if !env.IsContainer {
		env.IsVM = isRunningInVM()
		if env.IsVM {
			env.Hypervisor = detectHypervisor()
		}
	}

	// 3. Check if a container runtime is available on this host
	env.HasContainerRuntime = hasContainerRuntime()

	// 4. Determine if physical
	env.IsPhysical = !env.IsContainer && !env.IsVM

	// 5. Determine final type
	env.Type = determineType(env)

	return env
}

// determineType determines the final environment type
func determineType(env *Environment) EnvironmentType {
	if env.IsContainer {
		return EnvContainer
	}

	if env.IsVM {
		if env.HasContainerRuntime {
			return EnvVMWithContainers
		}
		return EnvVM
	}

	// Physical host
	if env.HasContainerRuntime {
		return EnvPhysicalWithContainers
	}
	return EnvPhysical
}

// readFileLimited reads a file with a size limit and returns its content as a string.
func readFileLimited(path string, limit int64) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit))
	if err != nil {
		return ""
	}
	return string(data)
}

// readCgroup reads /proc/1/cgroup with a size limit.
func readCgroup() string {
	return readFileLimited("/proc/1/cgroup", cgroupReadLimit)
}

// isRunningInContainer detects if running inside a container
func isRunningInContainer() bool {
	// Method 1: Check for /.dockerenv file (Docker)
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Method 2: Check cgroup for container indicators
	content := readCgroup()
	if strings.Contains(content, "docker") ||
		strings.Contains(content, "lxc") ||
		strings.Contains(content, "kubepods") ||
		strings.Contains(content, "podman") {
		return true
	}

	return false
}

// detectContainerRuntime identifies the container runtime
func detectContainerRuntime() string {
	if content := readCgroup(); content != "" {
		if strings.Contains(content, "docker") {
			return "docker"
		}
		if strings.Contains(content, "lxc") {
			return "lxc"
		}
		if strings.Contains(content, "kubepods") {
			return "kubernetes"
		}
		if strings.Contains(content, "podman") {
			return "podman"
		}
	}

	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker"
	}

	return "unknown"
}

// isRunningInVM detects if running in a virtual machine
func isRunningInVM() bool {
	// Linux: check multiple indicators
	if runtime.GOOS == "linux" {
		// Method 1: Check /sys/class/dmi/id/product_name
		if product := strings.ToLower(readFileLimited("/sys/class/dmi/id/product_name", dmiReadLimit)); product != "" {
			if strings.Contains(product, "vmware") ||
				strings.Contains(product, "virtualbox") ||
				strings.Contains(product, "kvm") ||
				strings.Contains(product, "qemu") ||
				strings.Contains(product, "virtual") ||
				strings.Contains(product, "bochs") {
				return true
			}
		}

		// Method 2: Check /sys/class/dmi/id/sys_vendor
		if vendor := strings.ToLower(readFileLimited("/sys/class/dmi/id/sys_vendor", dmiReadLimit)); vendor != "" {
			if strings.Contains(vendor, "vmware") ||
				strings.Contains(vendor, "innotek") || // VirtualBox
				strings.Contains(vendor, "qemu") ||
				strings.Contains(vendor, "microsoft") || // Hyper-V
				strings.Contains(vendor, "xen") {
				return true
			}
		}

		// Method 3: Check systemd-detect-virt if available
		// (We'll add this if needed)
	}

	// macOS: Check for virtualization
	if runtime.GOOS == "darwin" {
		// macOS VMs are less common, but we can check sysctl
		// This would require additional implementation
	}

	return false
}

// detectHypervisor identifies the hypervisor type
func detectHypervisor() string {
	if runtime.GOOS == "linux" {
		// Check product name
		if product := strings.ToLower(readFileLimited("/sys/class/dmi/id/product_name", dmiReadLimit)); product != "" {
			if strings.Contains(product, "vmware") {
				return "vmware"
			}
			if strings.Contains(product, "virtualbox") {
				return "virtualbox"
			}
			if strings.Contains(product, "kvm") || strings.Contains(product, "qemu") {
				return "kvm"
			}
		}

		// Check sys vendor
		if vendor := strings.ToLower(readFileLimited("/sys/class/dmi/id/sys_vendor", dmiReadLimit)); vendor != "" {
			if strings.Contains(vendor, "vmware") {
				return "vmware"
			}
			if strings.Contains(vendor, "innotek") {
				return "virtualbox"
			}
			if strings.Contains(vendor, "qemu") {
				return "kvm"
			}
			if strings.Contains(vendor, "microsoft") {
				return "hyperv"
			}
			if strings.Contains(vendor, "xen") {
				return "xen"
			}
		}
	}

	return "unknown"
}

// hasContainerRuntime checks if any supported container runtime socket is accessible.
// Currently checks Docker and rootful Podman.
func hasContainerRuntime() bool {
	for _, path := range []string{
		"/var/run/docker.sock",
		"/run/podman/podman.sock",
	} {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

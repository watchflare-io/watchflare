package sysinfo

// MetricsConfig defines which metrics to collect based on environment type
type MetricsConfig struct {
	CollectCPU         bool
	CollectMemory      bool
	CollectDisk        bool
	CollectDiskIO      bool
	CollectNetwork     bool
	CollectSwap        bool
	CollectLoadAvg     bool
	CollectTemperature bool

	// Container-specific (when the agent itself runs inside a container)
	CollectContainerCPU    bool
	CollectContainerMemory bool

	// Container runtime metrics (opt-in: for hosts running Docker, Podman, etc.)
	CollectRuntimeCPU     bool
	CollectRuntimeMemory  bool
	CollectRuntimeNetwork bool

	// ContainerRuntime is the runtime the agent is running inside ("docker", "kubernetes", etc.)
	// Empty for physical/VM hosts. Detected once at startup and included in every HostInfo update.
	ContainerRuntime string
}

// GetMetricsConfig returns the appropriate metrics configuration based on environment.
// containerMetrics controls whether container runtime metrics are collected (opt-in).
func GetMetricsConfig(env *Environment, containerMetrics bool) *MetricsConfig {
	config := &MetricsConfig{}

	switch env.Type {
	case EnvPhysical:
		// Physical host: collect everything
		config.CollectCPU = true
		config.CollectMemory = true
		config.CollectDisk = true
		config.CollectDiskIO = true
		config.CollectNetwork = true
		config.CollectSwap = true
		config.CollectLoadAvg = true
		config.CollectTemperature = true

	case EnvPhysicalWithContainers:
		// Physical host running a container runtime: collect everything
		config.CollectCPU = true
		config.CollectMemory = true
		config.CollectDisk = true
		config.CollectDiskIO = true
		config.CollectNetwork = true
		config.CollectSwap = true
		config.CollectLoadAvg = true
		config.CollectTemperature = true

	case EnvVM:
		// VM without containers: collect most things except temperature
		config.CollectCPU = true
		config.CollectMemory = true
		config.CollectDisk = true
		config.CollectDiskIO = true
		config.CollectNetwork = true
		config.CollectLoadAvg = true
		config.CollectSwap = true
		config.CollectTemperature = false // Can't read physical sensors

	case EnvVMWithContainers:
		// VM running a container runtime: collect VM metrics
		config.CollectCPU = true
		config.CollectMemory = true
		config.CollectDisk = true
		config.CollectDiskIO = true
		config.CollectNetwork = true
		config.CollectLoadAvg = true
		config.CollectSwap = true
		config.CollectTemperature = false

	case EnvContainer:
		if isSystemContainer(env.ContainerRuntime) {
			// System container (LXC): shares the host kernel but has its own
			// rootfs and network namespace, so it is monitored like a
			// full host. CPU, memory, swap, disk usage and network are isolated
			// per container; disk I/O, load average and temperature come from the
			// shared host kernel/hardware but are still surfaced for visibility.
			config.CollectCPU = true
			config.CollectMemory = true
			config.CollectDisk = true
			config.CollectDiskIO = true
			config.CollectNetwork = true
			config.CollectSwap = true
			config.CollectLoadAvg = true
			config.CollectTemperature = true
		} else {
			// Application container (Docker, Podman, Kubernetes): disk, I/O and
			// network are shared with the host or not meaningful for the container.
			config.CollectCPU = true
			config.CollectMemory = true
			config.CollectDisk = false
			config.CollectDiskIO = false
			config.CollectNetwork = false
			config.CollectSwap = false
			config.CollectLoadAvg = true
			config.CollectTemperature = false
			config.CollectContainerCPU = true
			config.CollectContainerMemory = true
		}
	}

	// Container runtime metrics (Docker, Podman, etc.) are opt-in via config flag.
	// Applied regardless of environment detection — the collector itself checks socket
	// availability at collection time. Not applicable when the agent runs inside a container.
	if containerMetrics && env.Type != EnvContainer {
		config.CollectRuntimeCPU = true
		config.CollectRuntimeMemory = true
		config.CollectRuntimeNetwork = true
	}

	return config
}

// String returns a human-readable description of the environment
func (e *Environment) String() string {
	switch e.Type {
	case EnvPhysical:
		return "Physical Host"
	case EnvPhysicalWithContainers:
		return "Physical Host with Containers"
	case EnvVM:
		return "Virtual Machine (" + e.Hypervisor + ")"
	case EnvVMWithContainers:
		return "Virtual Machine with Containers (" + e.Hypervisor + ")"
	case EnvContainer:
		return "Container (" + e.ContainerRuntime + ")"
	default:
		return "Unknown"
	}
}

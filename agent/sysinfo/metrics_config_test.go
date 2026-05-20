package sysinfo

import "testing"

// --- GetMetricsConfig ---

func TestGetMetricsConfig_Physical(t *testing.T) {
	cfg := GetMetricsConfig(&Environment{Type: EnvPhysical}, false)

	if !cfg.CollectCPU || !cfg.CollectMemory || !cfg.CollectDisk || !cfg.CollectDiskIO ||
		!cfg.CollectNetwork || !cfg.CollectSwap || !cfg.CollectLoadAvg || !cfg.CollectTemperature {
		t.Error("physical: all base metrics must be enabled")
	}
	if cfg.CollectRuntimeCPU || cfg.CollectRuntimeMemory || cfg.CollectRuntimeNetwork {
		t.Error("physical: container runtime metrics must be disabled")
	}
}

func TestGetMetricsConfig_PhysicalWithContainers_DockerOptIn(t *testing.T) {
	cfg := GetMetricsConfig(&Environment{Type: EnvPhysicalWithContainers}, true)

	if !cfg.CollectTemperature {
		t.Error("physical with containers: temperature must be enabled")
	}
	if !cfg.CollectRuntimeCPU || !cfg.CollectRuntimeMemory || !cfg.CollectRuntimeNetwork {
		t.Error("physical with containers + containerMetrics=true: runtime metrics must be enabled")
	}
}

func TestGetMetricsConfig_PhysicalWithContainers_DockerOptOut(t *testing.T) {
	cfg := GetMetricsConfig(&Environment{Type: EnvPhysicalWithContainers}, false)

	if cfg.CollectRuntimeCPU || cfg.CollectRuntimeMemory || cfg.CollectRuntimeNetwork {
		t.Error("physical with containers + containerMetrics=false: runtime metrics must be disabled")
	}
}

func TestGetMetricsConfig_VM(t *testing.T) {
	cfg := GetMetricsConfig(&Environment{Type: EnvVM}, false)

	if !cfg.CollectCPU || !cfg.CollectMemory || !cfg.CollectDisk || !cfg.CollectNetwork {
		t.Error("VM: basic metrics must be enabled")
	}
	if !cfg.CollectSwap {
		t.Error("VM: swap must be enabled")
	}
	if cfg.CollectTemperature {
		t.Error("VM: temperature must be disabled")
	}
}

func TestGetMetricsConfig_VMWithContainers_DockerOptIn(t *testing.T) {
	cfg := GetMetricsConfig(&Environment{Type: EnvVMWithContainers}, true)

	if cfg.CollectTemperature {
		t.Error("VM with containers: temperature must be disabled")
	}
	if !cfg.CollectSwap {
		t.Error("VM with containers: swap must be enabled")
	}
	if !cfg.CollectRuntimeCPU || !cfg.CollectRuntimeMemory || !cfg.CollectRuntimeNetwork {
		t.Error("VM with containers + containerMetrics=true: runtime metrics must be enabled")
	}
}

func TestGetMetricsConfig_Container(t *testing.T) {
	cfg := GetMetricsConfig(&Environment{Type: EnvContainer}, false)

	if !cfg.CollectCPU || !cfg.CollectMemory {
		t.Error("container: CPU and memory must be enabled")
	}
	if cfg.CollectDisk || cfg.CollectDiskIO || cfg.CollectNetwork || cfg.CollectSwap || cfg.CollectTemperature {
		t.Error("container: disk, network, swap, temperature must be disabled")
	}
	if !cfg.CollectContainerCPU || !cfg.CollectContainerMemory {
		t.Error("container: container-specific metrics must be enabled")
	}
}

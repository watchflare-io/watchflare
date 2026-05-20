package sysinfo

import "testing"

// --- determineType ---

func TestDetermineType_Container(t *testing.T) {
	env := &Environment{IsContainer: true, HasContainerRuntime: true}
	if got := determineType(env); got != EnvContainer {
		t.Errorf("expected %q, got %q", EnvContainer, got)
	}
}

func TestDetermineType_VMWithContainers(t *testing.T) {
	env := &Environment{IsVM: true, HasContainerRuntime: true}
	if got := determineType(env); got != EnvVMWithContainers {
		t.Errorf("expected %q, got %q", EnvVMWithContainers, got)
	}
}

func TestDetermineType_VM(t *testing.T) {
	env := &Environment{IsVM: true, HasContainerRuntime: false}
	if got := determineType(env); got != EnvVM {
		t.Errorf("expected %q, got %q", EnvVM, got)
	}
}

func TestDetermineType_PhysicalWithContainers(t *testing.T) {
	env := &Environment{IsPhysical: true, HasContainerRuntime: true}
	if got := determineType(env); got != EnvPhysicalWithContainers {
		t.Errorf("expected %q, got %q", EnvPhysicalWithContainers, got)
	}
}

func TestDetermineType_Physical(t *testing.T) {
	env := &Environment{IsPhysical: true, HasContainerRuntime: false}
	if got := determineType(env); got != EnvPhysical {
		t.Errorf("expected %q, got %q", EnvPhysical, got)
	}
}

// --- Environment.String ---

func TestEnvironmentString(t *testing.T) {
	tests := []struct {
		env  *Environment
		want string
	}{
		{&Environment{Type: EnvPhysical}, "Physical Host"},
		{&Environment{Type: EnvPhysicalWithContainers}, "Physical Host with Containers"},
		{&Environment{Type: EnvVM, Hypervisor: "kvm"}, "Virtual Machine (kvm)"},
		{&Environment{Type: EnvVMWithContainers, Hypervisor: "vmware"}, "Virtual Machine with Containers (vmware)"},
		{&Environment{Type: EnvContainer, ContainerRuntime: "docker"}, "Container (docker)"},
	}
	for _, tt := range tests {
		if got := tt.env.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

package sysinfo

import (
	"net"
	"testing"
)

func TestGetIPAddresses_ReturnsValidAddresses(t *testing.T) {
	ipv4, ipv6 := GetIPAddresses()

	if ipv4 != "" {
		if net.ParseIP(ipv4) == nil {
			t.Errorf("invalid IPv4 address: %q", ipv4)
		}
		if net.ParseIP(ipv4).To4() == nil {
			t.Errorf("expected IPv4, got %q", ipv4)
		}
	}

	if ipv6 != "" {
		if net.ParseIP(ipv6) == nil {
			t.Errorf("invalid IPv6 address: %q", ipv6)
		}
	}
}

func TestGetIPAddresses_NoLoopback(t *testing.T) {
	ipv4, ipv6 := GetIPAddresses()

	if ipv4 != "" {
		ip := net.ParseIP(ipv4)
		if ip != nil && ip.IsLoopback() {
			t.Errorf("expected non-loopback IPv4, got %q", ipv4)
		}
	}

	if ipv6 != "" {
		ip := net.ParseIP(ipv6)
		if ip != nil && ip.IsLoopback() {
			t.Errorf("expected non-loopback IPv6, got %q", ipv6)
		}
	}
}

func TestCollect_ReturnsBasicInfo(t *testing.T) {
	info, err := Collect()
	if err != nil {
		t.Fatalf("Collect() failed: %v", err)
	}

	if info.Hostname == "" {
		t.Error("expected non-empty Hostname")
	}
	if info.OS == "" {
		t.Error("expected non-empty OS")
	}
	if info.KernelArch == "" {
		t.Error("expected non-empty KernelArch")
	}
	if info.CPUPhysicalCount <= 0 {
		t.Errorf("expected CPUPhysicalCount > 0, got %d", info.CPUPhysicalCount)
	}
	if info.CPULogicalCount <= 0 {
		t.Errorf("expected CPULogicalCount > 0, got %d", info.CPULogicalCount)
	}
	if info.CPULogicalCount < info.CPUPhysicalCount {
		t.Errorf("logical count (%d) < physical count (%d)", info.CPULogicalCount, info.CPUPhysicalCount)
	}
	if info.HostID == "" {
		t.Error("expected non-empty HostID")
	}
}

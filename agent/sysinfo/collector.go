package sysinfo

import (
	"context"
	"net"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
)

const sysinfoTimeout = 5 * time.Second

// SystemInfo contains information about the system, aligned with gopsutil naming conventions.
type SystemInfo struct {
	// Network
	IPv4Address string
	IPv6Address string

	// gopsutil host.InfoStat fields
	Hostname             string
	OS                   string // "linux", "darwin", "windows"
	Platform             string // distro name: "fedora", "ubuntu", "macos"
	PlatformFamily       string // distro family: "rhel", "debian"
	PlatformVersion      string // "43", "22.04", "15.6.1"
	KernelVersion        string // "6.17.1-300.fc43.aarch64", "24.6.0"
	KernelArch           string // "aarch64", "x86_64", "arm64"
	VirtualizationSystem string // "kvm", "vmware", "xen" (empty if physical/unknown)
	VirtualizationRole   string // "guest", "host" (empty if physical)
	HostID               string // unique OS-provided UUID

	// gopsutil cpu.InfoStat fields
	CPUModelName    string
	CPUPhysicalCount int
	CPULogicalCount  int
	CPUMhz           float64
}

// Collect gathers system information using gopsutil.
func Collect() (*SystemInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sysinfoTimeout)
	defer cancel()

	info := &SystemInfo{}

	// Host info (hostname, OS, platform, kernel, virtualization, host ID)
	hostInfo, err := host.InfoWithContext(ctx)
	if err != nil {
		return nil, err
	}
	info.Hostname = hostInfo.Hostname
	info.OS = hostInfo.OS
	info.Platform = hostInfo.Platform
	info.PlatformFamily = hostInfo.PlatformFamily
	info.PlatformVersion = hostInfo.PlatformVersion
	info.KernelVersion = hostInfo.KernelVersion
	info.KernelArch = hostInfo.KernelArch
	info.VirtualizationSystem = hostInfo.VirtualizationSystem
	info.VirtualizationRole = hostInfo.VirtualizationRole
	info.HostID = hostInfo.HostID

	// CPU model info — take first entry, fail gracefully
	cpuInfos, err := cpu.InfoWithContext(ctx)
	if err == nil && len(cpuInfos) > 0 {
		info.CPUModelName = cpuInfos[0].ModelName
		info.CPUMhz = cpuInfos[0].Mhz
	}

	physCount, err := cpu.CountsWithContext(ctx, false)
	if err == nil {
		info.CPUPhysicalCount = physCount
	}
	logCount, err := cpu.CountsWithContext(ctx, true)
	if err == nil {
		info.CPULogicalCount = logCount
	}

	// IP addresses (not provided by gopsutil host package)
	info.IPv4Address, info.IPv6Address = GetIPAddresses()

	return info, nil
}

// GetIPAddresses returns the primary non-loopback IPv4 and IPv6 addresses.
func GetIPAddresses() (string, string) {
	var ipv4, ipv6 string

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", ""
	}

	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		if ipnet.IP.To4() != nil {
			if ipv4 == "" {
				ipv4 = ipnet.IP.String()
			}
		} else if ipv6 == "" && !ipnet.IP.IsLinkLocalUnicast() {
			ipv6 = ipnet.IP.String()
		}
	}

	return ipv4, ipv6
}

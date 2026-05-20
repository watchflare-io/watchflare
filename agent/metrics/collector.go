package metrics

import (
	"log/slog"
	"sync"
	"time"
	"watchflare-agent/sysinfo"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

// Initialize is a no-op kept for API compatibility.
// CPU metrics use a manual T1→sleep(1s)→T2 delta per collection, so no pre-warming is needed.
func Initialize() {}

// cpuStaticInfo caches CPU model/count/MHz — these never change at runtime.
var (
	cpuStaticOnce     sync.Once
	cpuStaticModel    string
	cpuStaticMHz      float64
	cpuStaticPhysical int32
	cpuStaticLogical  int32
)

func initCPUStatic() {
	cpuStaticOnce.Do(func() {
		if infos, err := cpu.Info(); err == nil && len(infos) > 0 {
			cpuStaticModel = infos[0].ModelName
			cpuStaticMHz = infos[0].Mhz
		}
		if count, err := cpu.Counts(false); err == nil {
			cpuStaticPhysical = int32(count)
		}
		if count, err := cpu.Counts(true); err == nil {
			cpuStaticLogical = int32(count)
		}
	})
}

// HostInfoSnapshot holds slowly-changing host properties collected alongside metrics.
// It is NOT serialized to the WAL — the sender attaches the current snapshot at send time.
type HostInfoSnapshot struct {
	PlatformVersion  string
	KernelVersion    string
	KernelArch       string
	CPUModelName     string
	CPUPhysicalCount int32
	CPULogicalCount  int32
	CPUMhz           float64
	ContainerRuntime string
}

// SystemMetrics represents collected system metrics
type SystemMetrics struct {
	CPUUsagePercent   float64
	CPUIowaitPercent  float64 // Linux only: waiting for I/O (0 on other platforms)
	CPUStealPercent   float64 // Linux VMs only: CPU stolen by hypervisor (0 on other platforms)
	MemoryTotalBytes     uint64
	MemoryUsedBytes      uint64
	MemoryAvailableBytes uint64
	MemoryBuffersBytes   uint64 // Linux only: kernel buffer cache (0 on other platforms)
	MemoryCachedBytes    uint64 // Linux only: page cache (0 on other platforms)
	SwapTotalBytes       uint64
	SwapUsedBytes        uint64
	LoadAvg1Min          float64
	LoadAvg5Min          float64
	LoadAvg15Min         float64
	DiskTotalBytes       uint64
	DiskUsedBytes        uint64
	UptimeSeconds        uint64
	ProcessesCount       uint64
	Timestamp            int64

	// Disk I/O rates (bytes per second)
	DiskReadBytesPerSec  uint64
	DiskWriteBytesPerSec uint64

	// Network rates (bytes per second)
	NetworkRxBytesPerSec uint64
	NetworkTxBytesPerSec uint64

	// Temperature (physical hosts only)
	CPUTemperatureCelsius float64

	// All sensor readings (temperature sensors, battery, storage, etc.)
	SensorReadings []SensorReading

	// Docker container metrics (only for hosts with containers)
	ContainerMetrics []ContainerMetric

	// HostInfo holds slowly-changing host properties (kernel, CPU model, etc.)
	// Populated on every collection but NOT serialized to WAL.
	HostInfo HostInfoSnapshot
}

// Package-level delta tracker for rate-based metrics (disk I/O, network)
var deltaTracker = NewDeltaTracker()

// containerErrorLogged suppresses repeated container runtime error logs after the first failure.
// Protected by containerErrorMu.
var (
	containerErrorLogged bool
	containerErrorMu     sync.Mutex
)

// Collect gathers system metrics based on environment configuration
// config parameter determines which metrics to collect (e.g., containers don't collect disk)
func Collect(config *sysinfo.MetricsConfig) (*SystemMetrics, error) {
	metrics := &SystemMetrics{
		Timestamp: time.Now().Unix(),
	}

	// CPU usage + iowait + steal: manual T1→sleep(1s)→T2 delta
	if config.CollectCPU {
		t1, err := cpu.Times(false)
		if err != nil {
			slog.Debug("failed to collect CPU metrics (T1)", "error", err)
		} else {
			time.Sleep(time.Second)
			t2, err2 := cpu.Times(false)
			if err2 != nil {
				slog.Debug("failed to collect CPU metrics (T2)", "error", err2)
			} else if len(t1) > 0 && len(t2) > 0 {
				prev, curr := t1[0], t2[0]
				totalDelta := curr.Total() - prev.Total()
				if totalDelta > 0 {
					metrics.CPUUsagePercent = (totalDelta - (curr.Idle - prev.Idle)) / totalDelta * 100
					metrics.CPUIowaitPercent = (curr.Iowait - prev.Iowait) / totalDelta * 100
					metrics.CPUStealPercent = (curr.Steal - prev.Steal) / totalDelta * 100
				}
			}
		}
	}

	// Memory stats
	if config.CollectMemory {
		memStats, err := mem.VirtualMemory()
		if err != nil {
			slog.Debug("failed to collect memory metrics", "error", err)
		} else {
			metrics.MemoryTotalBytes = memStats.Total
			metrics.MemoryAvailableBytes = memStats.Available
			if memStats.Total >= memStats.Available {
				metrics.MemoryUsedBytes = memStats.Total - memStats.Available
			} else {
				slog.Debug("memory available exceeds total, reporting used as 0",
					"total", memStats.Total, "available", memStats.Available)
			}
			metrics.MemoryBuffersBytes = memStats.Buffers
			metrics.MemoryCachedBytes = memStats.Cached
		}
	}

	// Swap stats
	if config.CollectSwap {
		swapStats, err := mem.SwapMemory()
		if err != nil {
			slog.Debug("failed to collect swap metrics", "error", err)
		} else {
			metrics.SwapTotalBytes = swapStats.Total
			metrics.SwapUsedBytes = swapStats.Used
		}
	}

	// Load average
	if config.CollectLoadAvg {
		loadStats, err := load.Avg()
		if err != nil {
			slog.Debug("failed to collect load average", "error", err)
		} else {
			metrics.LoadAvg1Min = loadStats.Load1
			metrics.LoadAvg5Min = loadStats.Load5
			metrics.LoadAvg15Min = loadStats.Load15
		}
	}

	// Disk usage - SKIPPED for containers to avoid double-counting
	// On macOS: uses diskutil for APFS-accurate values (container level)
	// On Linux: uses gopsutil disk.Usage("/")
	if config.CollectDisk {
		total, used, diskErr := getDiskUsage()
		if diskErr != nil {
			slog.Debug("failed to collect disk usage", "error", diskErr)
		} else {
			metrics.DiskTotalBytes = total
			metrics.DiskUsedBytes = used
		}
	}

	// Disk I/O
	if config.CollectDiskIO {
		readBytes, writeBytes, ioErr := getDiskIOCounters()
		if ioErr != nil {
			slog.Debug("failed to collect disk I/O", "error", ioErr)
		} else {
			now := time.Now()
			metrics.DiskReadBytesPerSec, metrics.DiskWriteBytesPerSec = deltaTracker.ComputeDiskIORate(readBytes, writeBytes, now)
		}
	}

	// Network bandwidth
	if config.CollectNetwork {
		rxBytes, txBytes, netErr := getNetworkCounters()
		if netErr != nil {
			slog.Debug("failed to collect network counters", "error", netErr)
		} else {
			now := time.Now()
			metrics.NetworkRxBytesPerSec, metrics.NetworkTxBytesPerSec = deltaTracker.ComputeNetworkRate(rxBytes, txBytes, now)
		}
	}

	// Temperature (physical hosts only) — single syscall for CPU temp + all readings
	if config.CollectTemperature {
		cpuTemp, readings, tempErr := collectTemperatures()
		if tempErr != nil {
			slog.Debug("failed to collect temperature sensors", "error", tempErr)
		} else {
			metrics.CPUTemperatureCelsius = cpuTemp
			metrics.SensorReadings = readings
		}
	}

	// Container runtime metrics (Docker, Podman, etc.)
	if config.CollectRuntimeCPU || config.CollectRuntimeMemory || config.CollectRuntimeNetwork {
		containerMetrics, containerErr := CollectContainerMetrics(deltaTracker)
		func() {
			containerErrorMu.Lock()
			defer containerErrorMu.Unlock()
			if containerErr != nil {
				if !containerErrorLogged {
					slog.Warn("failed to collect container metrics", "error", containerErr)
					containerErrorLogged = true
				}
			} else if containerMetrics != nil {
				if containerErrorLogged {
					slog.Info("container metrics collection recovered after previous error")
					containerErrorLogged = false
				}
				slog.Debug("container metrics collected", "count", len(containerMetrics))
				metrics.ContainerMetrics = containerMetrics
			}
		}()
	}

	// System uptime + process count + kernel/platform info (single host.Info call)
	hostInfo, err := host.Info()
	if err == nil {
		metrics.UptimeSeconds = hostInfo.Uptime
		metrics.ProcessesCount = hostInfo.Procs
		metrics.HostInfo.PlatformVersion = hostInfo.PlatformVersion
		metrics.HostInfo.KernelVersion = hostInfo.KernelVersion
		metrics.HostInfo.KernelArch = hostInfo.KernelArch
	}

	// CPU model + frequency — cached once at startup (never changes at runtime)
	initCPUStatic()
	metrics.HostInfo.CPUModelName = cpuStaticModel
	metrics.HostInfo.CPUMhz = cpuStaticMHz
	metrics.HostInfo.CPUPhysicalCount = cpuStaticPhysical
	metrics.HostInfo.CPULogicalCount = cpuStaticLogical

	metrics.HostInfo.ContainerRuntime = config.ContainerRuntime

	return metrics, nil
}

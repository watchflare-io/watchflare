package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// dockerAPIVersion is the Docker Engine API version used for requests.
// v1.44 is the minimum supported by Docker 23.0+ (released January 2023) and
// required by newer runtimes like Colima. Podman exposes the same API.
const dockerAPIVersion = "v1.44"

// containerSocket pairs a socket path with its runtime name.
type containerSocket struct {
	path    string
	runtime string
}

// knownContainerSockets returns the list of socket paths to probe, in order.
// Docker is tried first, then Podman. On macOS, Docker Desktop user-level paths
// are added as fallbacks since /var/run/docker.sock may be a broken symlink.
func knownContainerSockets() []containerSocket {
	sockets := []containerSocket{
		{"/var/run/docker.sock", "docker"},
		{"/run/podman/podman.sock", "podman"},
	}

	if runtime.GOOS == "darwin" {
		if home, err := os.UserHomeDir(); err == nil {
			// Colima (default profile): most common alternative to Docker Desktop on macOS
			sockets = append(sockets,
				containerSocket{filepath.Join(home, ".colima", "default", "docker.sock"), "docker"},
			)
			// Docker Desktop 4.x+ on macOS: socket in user home
			sockets = append(sockets,
				containerSocket{filepath.Join(home, ".docker", "desktop", "docker.sock"), "docker"},
				containerSocket{filepath.Join(home, ".docker", "run", "docker.sock"), "docker"},
			)
		}
	}

	return sockets
}

// ContainerMetric represents metrics for a single container
type ContainerMetric struct {
	Runtime              string // "docker", "podman"
	ContainerID          string
	ContainerName        string
	Image                string
	CPUPercent           float64
	MemoryUsedBytes      uint64
	MemoryLimitBytes     uint64
	NetworkRxBytesPerSec uint64
	NetworkTxBytesPerSec uint64
}

// dockerStatsResponse matches the Docker Engine API /containers/{id}/stats response.
// Podman uses the same schema.
type dockerStatsResponse struct {
	CPUStats    dockerCPUStats    `json:"cpu_stats"`
	PreCPUStats dockerCPUStats    `json:"precpu_stats"`
	MemoryStats dockerMemoryStats `json:"memory_stats"`
	Networks    map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
}

type dockerCPUStats struct {
	CPUUsage struct {
		TotalUsage uint64 `json:"total_usage"`
	} `json:"cpu_usage"`
	SystemCPUUsage uint64 `json:"system_cpu_usage"`
	OnlineCPUs     uint64 `json:"online_cpus"`
}

type dockerMemoryStats struct {
	Usage uint64 `json:"usage"`
	Limit uint64 `json:"limit"`
	Stats struct {
		InactiveFile uint64 `json:"inactive_file"`
		Cache        uint64 `json:"cache"`
	} `json:"stats"`
}

type dockerContainer struct {
	ID    string   `json:"Id"`
	Names []string `json:"Names"`
	Image string   `json:"Image"`
	State string   `json:"State"`
}

// newSocketClient returns an HTTP client that dials the given Unix socket path.
func newSocketClient(socketPath string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
		Timeout: 10 * time.Second,
	}
}

// CollectContainerMetrics collects metrics from all accessible container runtime sockets.
// It probes each known socket in order. Only one socket per runtime is used to avoid
// collecting the same containers twice across fallback paths.
func CollectContainerMetrics(tracker *DeltaTracker) ([]ContainerMetric, error) {
	var allMetrics []ContainerMetric
	var lastErr error
	collected := make(map[string]bool) // runtime name → already collected

	for _, sock := range knownContainerSockets() {
		if collected[sock.runtime] {
			continue
		}
		slog.Debug("probing container socket", "runtime", sock.runtime, "path", sock.path)
		client := newSocketClient(sock.path)
		m, err := collectFromSocket(client, sock.runtime, tracker)
		if err != nil {
			slog.Debug("container runtime unavailable", "runtime", sock.runtime, "path", sock.path, "error", err)
			lastErr = err
			continue
		}
		slog.Debug("collected container metrics", "runtime", sock.runtime, "count", len(m))
		allMetrics = append(allMetrics, m...)
		collected[sock.runtime] = true
	}

	if len(allMetrics) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return allMetrics, nil
}

// collectFromSocket collects metrics for all running containers on a single socket.
func collectFromSocket(client *http.Client, runtime string, tracker *DeltaTracker) ([]ContainerMetric, error) {
	resp, err := client.Get("http://localhost/" + dockerAPIVersion + "/containers/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s API returned status %d", runtime, resp.StatusCode)
	}

	var containers []dockerContainer
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&containers); err != nil {
		return nil, err
	}

	if len(containers) == 0 {
		return nil, nil
	}

	var result []ContainerMetric

	for _, c := range containers {
		if c.State != "running" {
			continue
		}

		stats, err := getContainerStats(client, c.ID)
		if err != nil {
			slog.Warn("failed to get stats for container", "runtime", runtime, "container_id", truncateID(c.ID), "error", err)
			continue
		}

		// Compute CPU percentage
		cpuPercent := computeCPUPercent(stats)

		// Compute memory (exclude cache for actual usage, guard against underflow)
		memUsed := stats.MemoryStats.Usage
		if stats.MemoryStats.Stats.InactiveFile > 0 && stats.MemoryStats.Stats.InactiveFile < stats.MemoryStats.Usage {
			memUsed -= stats.MemoryStats.Stats.InactiveFile
		}

		// Sum network counters across all interfaces
		var totalRx, totalTx uint64
		for _, netStats := range stats.Networks {
			totalRx += netStats.RxBytes
			totalTx += netStats.TxBytes
		}

		// Compute network rate using delta tracker
		now := time.Now()
		rxRate, txRate := tracker.ComputeContainerNetworkRate(c.ID, totalRx, totalTx, now)

		// Clean container name (remove leading /)
		name := truncateID(c.ID)
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		result = append(result, ContainerMetric{
			Runtime:              runtime,
			ContainerID:          truncateID(c.ID),
			ContainerName:        name,
			Image:                c.Image,
			CPUPercent:           cpuPercent,
			MemoryUsedBytes:      memUsed,
			MemoryLimitBytes:     stats.MemoryStats.Limit,
			NetworkRxBytesPerSec: rxRate,
			NetworkTxBytesPerSec: txRate,
		})
	}

	return result, nil
}

// truncateID safely truncates a container ID to 12 characters
func truncateID(id string) string {
	if len(id) >= 12 {
		return id[:12]
	}
	return id
}

// getContainerStats fetches one-shot stats for a container
func getContainerStats(client *http.Client, containerID string) (*dockerStatsResponse, error) {
	resp, err := client.Get("http://localhost/" + dockerAPIVersion + "/containers/" + containerID + "/stats?stream=false")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d for container %s", resp.StatusCode, truncateID(containerID))
	}

	// Limit response size to prevent excessive memory use from unexpectedly large payloads
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB
	if err != nil {
		return nil, err
	}

	var stats dockerStatsResponse
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

// computeCPUPercent calculates CPU usage percentage from Docker/Podman stats
func computeCPUPercent(stats *dockerStatsResponse) float64 {
	// Guard against uint64 underflow (e.g. container restart resets counters)
	if stats.CPUStats.CPUUsage.TotalUsage < stats.PreCPUStats.CPUUsage.TotalUsage ||
		stats.CPUStats.SystemCPUUsage < stats.PreCPUStats.SystemCPUUsage {
		return 0
	}

	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemCPUUsage - stats.PreCPUStats.SystemCPUUsage)

	if systemDelta <= 0 || cpuDelta <= 0 {
		return 0
	}

	numCPUs := stats.CPUStats.OnlineCPUs
	if numCPUs == 0 {
		numCPUs = 1
	}

	return (cpuDelta / systemDelta) * float64(numCPUs) * 100.0
}

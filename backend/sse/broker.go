package sse

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	EventTypeConnected               = "connected"
	EventTypeHostUpdate              = "host_update"
	EventTypeMetricsUpdate           = "metrics_update"
	EventTypeAggregatedMetricsUpdate = "aggregated_metrics_update"
	EventTypeContainerMetricsUpdate  = "container_metrics_update"
	EventTypePackageInventoryUpdate  = "package_inventory_update"
)

// Event represents a host event.
// HostID is an internal routing field used by Broadcast to filter per-host clients.
// It is never serialized — FormatSSE only marshals Data.
type Event struct {
	Type   string `json:"type"`
	Data   any    `json:"data"`
	HostID string `json:"-"` // empty = send to all clients; set = send only to matching host-filtered clients
}

// HostUpdate represents a host status update
type HostUpdate struct {
	ID               string `json:"id"`
	Status           string `json:"status"`
	IPv4Address      string `json:"ip_address_v4,omitempty"`
	IPv6Address      string `json:"ip_address_v6,omitempty"`
	ConfiguredIP     string `json:"configured_ip,omitempty"`
	IgnoreIPMismatch bool   `json:"ignore_ip_mismatch"`
	LastSeen         string `json:"last_seen"`
	Reactivated      bool   `json:"reactivated,omitempty"`  // true if agent was reactivated (UUID reused)
	Hostname         string `json:"hostname,omitempty"`     // hostname for reactivation notification
	ClockDesync      bool   `json:"clock_desync,omitempty"`  // true if agent's clock is out of sync
	AgentVersion     string `json:"agent_version,omitempty"` // current agent version
}

// MetricsUpdate is the input struct for BroadcastMetricsUpdate.
// It is converted to MetricsUpdateMinified before being sent to clients.
type MetricsUpdate struct {
	HostID                string                  `json:"host_id"`
	Timestamp             string                  `json:"timestamp"`
	CPUUsagePercent       float64                 `json:"cpu_usage_percent"`
	CPUIowaitPercent      float64                 `json:"cpu_iowait_percent"`
	CPUStealPercent       float64                 `json:"cpu_steal_percent"`
	MemoryTotalBytes      uint64                  `json:"memory_total_bytes"`
	MemoryUsedBytes       uint64                  `json:"memory_used_bytes"`
	MemoryAvailableBytes  uint64                  `json:"memory_available_bytes"`
	MemoryBuffersBytes    uint64                  `json:"memory_buffers_bytes"`
	MemoryCachedBytes     uint64                  `json:"memory_cached_bytes"`
	SwapTotalBytes        uint64                  `json:"swap_total_bytes"`
	SwapUsedBytes         uint64                  `json:"swap_used_bytes"`
	LoadAvg1Min           float64                 `json:"load_avg_1min"`
	LoadAvg5Min           float64                 `json:"load_avg_5min"`
	LoadAvg15Min          float64                 `json:"load_avg_15min"`
	DiskTotalBytes        uint64                  `json:"disk_total_bytes"`
	DiskUsedBytes         uint64                  `json:"disk_used_bytes"`
	DiskReadBytesPerSec   uint64                  `json:"disk_read_bytes_per_sec"`
	DiskWriteBytesPerSec  uint64                  `json:"disk_write_bytes_per_sec"`
	NetworkRxBytesPerSec  uint64                  `json:"network_rx_bytes_per_sec"`
	NetworkTxBytesPerSec  uint64                  `json:"network_tx_bytes_per_sec"`
	CPUTemperatureCelsius float64                 `json:"cpu_temperature_celsius"`
	UptimeSeconds         uint64                  `json:"uptime_seconds"`
	ProcessesCount        uint64                  `json:"processes_count"`
	SensorReadings        []SensorReadingMinified `json:"sensor_readings,omitempty"`
}

// SensorReadingMinified represents a minified sensor reading for SSE
type SensorReadingMinified struct {
	K string  `json:"k"` // sensor key
	V float64 `json:"v"` // temperature_celsius
}

// MetricsUpdateMinified is the wire format sent to SSE clients.
// Short field names reduce bandwidth on high-frequency updates.
// Format: {"h":"host1","t":1702741200,"c":22.5,"mu":4294967296,"mt":8589934592,...}
// IMPORTANT: field names must stay in sync with frontend/src/lib/sse/manager.ts
type MetricsUpdateMinified struct {
	HostID          string                  `json:"h"`            // host_id
	Timestamp       int64                   `json:"t"`            // Unix timestamp
	CPU             float64                 `json:"c"`            // cpu_usage_percent
	CPUIowait       float64                 `json:"iw"`           // cpu_iowait_percent (Linux only)
	CPUSteal        float64                 `json:"sl"`           // cpu_steal_percent (Linux VMs only)
	MemoryUsed      uint64                  `json:"mu"`           // memory_used_bytes
	MemoryTotal     uint64                  `json:"mt"`           // memory_total_bytes
	MemoryAvailable uint64                  `json:"ma"`           // memory_available_bytes
	MemoryBuffers   uint64                  `json:"mb"`           // memory_buffers_bytes (Linux only)
	MemoryCached    uint64                  `json:"mc"`           // memory_cached_bytes (Linux only)
	DiskUsed        uint64                  `json:"du"`           // disk_used_bytes
	DiskTotal       uint64                  `json:"dt"`           // disk_total_bytes
	LoadAvg1        float64                 `json:"l1"`           // load_avg_1min
	LoadAvg5        float64                 `json:"l5"`           // load_avg_5min
	LoadAvg15       float64                 `json:"l15"`          // load_avg_15min
	DiskReadRate    uint64                  `json:"dr"`           // disk_read_bytes_per_sec
	DiskWriteRate   uint64                  `json:"dw"`           // disk_write_bytes_per_sec
	NetRxRate       uint64                  `json:"nr"`           // network_rx_bytes_per_sec
	NetTxRate       uint64                  `json:"nt"`           // network_tx_bytes_per_sec
	CPUTemp         float64                 `json:"tmp"`          // cpu_temperature_celsius
	Uptime          uint64                  `json:"u"`            // uptime_seconds
	Processes       uint64                  `json:"pr"`           // processes_count
	SwapTotal       uint64                  `json:"st"`           // swap_total_bytes
	SwapUsed        uint64                  `json:"su"`           // swap_used_bytes
	SensorReadings  []SensorReadingMinified `json:"sr,omitempty"` // all sensor readings
}

// AggregatedMetricsUpdate represents aggregated metrics from all online hosts
type AggregatedMetricsUpdate struct {
	Timestamp            string  `json:"timestamp"`
	CPUUsagePercent      float64 `json:"cpu_usage_percent"`
	MemoryTotalBytes     uint64  `json:"memory_total_bytes"`
	MemoryUsedBytes      uint64  `json:"memory_used_bytes"`
	MemoryAvailableBytes uint64  `json:"memory_available_bytes"`
	DiskTotalBytes       uint64  `json:"disk_total_bytes"`
	DiskUsedBytes        uint64  `json:"disk_used_bytes"`
	LoadAvg1Min          float64 `json:"load_avg_1min"`
	LoadAvg5Min          float64 `json:"load_avg_5min"`
	LoadAvg15Min         float64 `json:"load_avg_15min"`
}

// ContainerMetricMinified represents a minified container metric for SSE
type ContainerMetricMinified struct {
	ID      string  `json:"i"`  // container_id (short hash)
	Name    string  `json:"n"`  // container_name
	CPU     float64 `json:"c"`  // cpu_percent
	MU      uint64  `json:"mu"` // memory_used_bytes
	ML      uint64  `json:"ml"` // memory_limit_bytes
	NR      uint64  `json:"nr"` // network_rx_bytes_per_sec
	NT      uint64  `json:"nt"` // network_tx_bytes_per_sec
	Runtime string  `json:"r"`  // container runtime ("docker", "podman")
}

// ContainerMetricsUpdate represents container metrics for SSE broadcast
type ContainerMetricsUpdate struct {
	HostID    string                    `json:"h"`
	Timestamp int64                     `json:"t"`
	Metrics   []ContainerMetricMinified `json:"m"`
}

// PackageInventoryUpdate notifies the frontend that a new package inventory was received
type PackageInventoryUpdate struct {
	HostID         string `json:"host_id"`
	CollectionType string `json:"collection_type"`
	PackagesCount  int    `json:"packages_count"`
	ChangesCount   int    `json:"changes_count"`
}

// Client represents an SSE client connection.
// HostID is non-empty for per-host detail pages; empty clients receive all events.
type Client struct {
	ID      string
	HostID  string // empty = global; non-empty = only receive events for this host
	Channel chan Event
}

// Broker manages SSE client connections and event broadcasting.
type Broker struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

var (
	broker *Broker
	once   sync.Once
)

// ClientCount returns the number of currently connected SSE clients.
func (b *Broker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// GetBroker returns the global Broker singleton.
func GetBroker() *Broker {
	once.Do(func() {
		broker = &Broker{
			clients: make(map[string]*Client),
		}
	})
	return broker
}

func (b *Broker) AddClient(clientID string) *Client {
	return b.addClient(clientID, "")
}

// AddClientWithHostFilter registers a client that only receives events for the given host.
// aggregated_metrics_update events are never delivered to host-filtered clients.
func (b *Broker) AddClientWithHostFilter(clientID, hostID string) *Client {
	return b.addClient(clientID, hostID)
}

func (b *Broker) addClient(clientID, hostID string) *Client {
	b.mu.Lock()
	defer b.mu.Unlock()

	// If a client with the same ID already exists (e.g. reconnect without prior cleanup),
	// close the old channel so its reader goroutine unblocks and exits.
	if existing, ok := b.clients[clientID]; ok {
		close(existing.Channel)
	}

	client := &Client{
		ID:      clientID,
		HostID:  hostID,
		Channel: make(chan Event, 10),
	}
	b.clients[clientID] = client
	if hostID != "" {
		slog.Info("SSE host-filtered client connected", "client_id", clientID, "host_id", hostID, "total", len(b.clients))
	} else {
		slog.Info("SSE client connected", "client_id", clientID, "total", len(b.clients))
	}

	return client
}

func (b *Broker) RemoveClient(clientID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if client, exists := b.clients[clientID]; exists {
		close(client.Channel)
		delete(b.clients, clientID)
		slog.Info("SSE client disconnected", "client_id", clientID, "total", len(b.clients))
	}
}

func (b *Broker) Broadcast(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, client := range b.clients {
		if client.HostID != "" {
			// Per-host clients: apply filtering rules.
			// Aggregated metrics are not meaningful on a single-host detail page.
			if event.Type == EventTypeAggregatedMetricsUpdate {
				continue
			}
			// Skip events that belong to a different host.
			if event.HostID != "" && event.HostID != client.HostID {
				continue
			}
		} else {
			// Global clients (dashboard): skip per-host raw metrics — only aggregated
			// metrics are consumed there. Per-host metrics_update and container_metrics_update
			// are handled exclusively on the per-host SSE stream.
			if event.Type == EventTypeMetricsUpdate || event.Type == EventTypeContainerMetricsUpdate {
				continue
			}
		}
		select {
		case client.Channel <- event:
		default:
			slog.Warn("SSE client channel full, dropping event", "client_id", client.ID, "event_type", event.Type)
		}
	}
}

func (b *Broker) BroadcastHostUpdate(update HostUpdate) {
	b.Broadcast(Event{Type: EventTypeHostUpdate, Data: update, HostID: update.ID})
}

func (b *Broker) BroadcastMetricsUpdate(update MetricsUpdate) {
	b.Broadcast(Event{Type: EventTypeMetricsUpdate, Data: toMinifiedMetrics(update), HostID: update.HostID})
}

// BroadcastAggregatedMetricsUpdate sends aggregated (cross-host) metrics.
// Per-host filtered clients never receive this event.
func (b *Broker) BroadcastAggregatedMetricsUpdate(update AggregatedMetricsUpdate) {
	b.Broadcast(Event{Type: EventTypeAggregatedMetricsUpdate, Data: update})
}

func (b *Broker) BroadcastContainerMetricsUpdate(update ContainerMetricsUpdate) {
	b.Broadcast(Event{Type: EventTypeContainerMetricsUpdate, Data: update, HostID: update.HostID})
}

func (b *Broker) BroadcastPackageInventoryUpdate(update PackageInventoryUpdate) {
	b.Broadcast(Event{Type: EventTypePackageInventoryUpdate, Data: update, HostID: update.HostID})
}

// toMinifiedMetrics converts a MetricsUpdate to the compact wire format.
// If the timestamp cannot be parsed, it defaults to 0 (callers always provide RFC3339).
func toMinifiedMetrics(update MetricsUpdate) MetricsUpdateMinified {
	var timestamp int64
	if t, err := time.Parse(time.RFC3339, update.Timestamp); err == nil {
		timestamp = t.Unix()
	}

	return MetricsUpdateMinified{
		HostID:          update.HostID,
		Timestamp:       timestamp,
		CPU:             update.CPUUsagePercent,
		CPUIowait:       update.CPUIowaitPercent,
		CPUSteal:        update.CPUStealPercent,
		MemoryUsed:      update.MemoryUsedBytes,
		MemoryTotal:     update.MemoryTotalBytes,
		MemoryAvailable: update.MemoryAvailableBytes,
		MemoryBuffers:   update.MemoryBuffersBytes,
		MemoryCached:    update.MemoryCachedBytes,
		DiskUsed:        update.DiskUsedBytes,
		DiskTotal:       update.DiskTotalBytes,
		LoadAvg1:        update.LoadAvg1Min,
		LoadAvg5:        update.LoadAvg5Min,
		LoadAvg15:       update.LoadAvg15Min,
		DiskReadRate:    update.DiskReadBytesPerSec,
		DiskWriteRate:   update.DiskWriteBytesPerSec,
		NetRxRate:       update.NetworkRxBytesPerSec,
		NetTxRate:       update.NetworkTxBytesPerSec,
		CPUTemp:         update.CPUTemperatureCelsius,
		Uptime:          update.UptimeSeconds,
		Processes:       update.ProcessesCount,
		SwapTotal:       update.SwapTotalBytes,
		SwapUsed:        update.SwapUsedBytes,
		SensorReadings:  update.SensorReadings,
	}
}

// FormatSSE formats an event as an SSE protocol message.
func FormatSSE(event Event) string {
	data, err := json.Marshal(event.Data)
	if err != nil {
		slog.Error("failed to marshal SSE event", "event_type", event.Type, "error", err)
		return ""
	}
	return fmt.Sprintf("event: %s\ndata: %s\n\n", event.Type, string(data))
}

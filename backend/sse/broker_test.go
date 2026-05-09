package sse

import (
	"strings"
	"testing"
	"time"
)

func newTestBroker() *Broker {
	return &Broker{clients: make(map[string]*Client)}
}

// --- GetBroker ---

func TestGetBroker_Singleton(t *testing.T) {
	b1 := GetBroker()
	b2 := GetBroker()
	if b1 != b2 {
		t.Error("expected GetBroker to return the same instance")
	}
}

// --- AddClient / RemoveClient ---

func TestAddClient(t *testing.T) {
	b := newTestBroker()

	client := b.AddClient("c1")

	if client.ID != "c1" {
		t.Errorf("id: got %s, want c1", client.ID)
	}
	if client.Channel == nil {
		t.Error("expected non-nil channel")
	}
	if _, ok := b.clients["c1"]; !ok {
		t.Error("expected client in broker map")
	}
}

func TestAddClient_ClosesExistingChannel(t *testing.T) {
	b := newTestBroker()

	first := b.AddClient("c1")
	b.AddClient("c1") // re-register same ID

	// The old channel must be closed so its reader can unblock.
	select {
	case _, open := <-first.Channel:
		if open {
			t.Error("expected old channel to be closed")
		}
	default:
		t.Error("expected old channel to be closed, but read would block")
	}
}

func TestRemoveClient(t *testing.T) {
	b := newTestBroker()

	client := b.AddClient("c1")
	b.RemoveClient("c1")

	if _, ok := b.clients["c1"]; ok {
		t.Error("expected client to be removed from broker map")
	}

	// Channel must be closed.
	select {
	case _, open := <-client.Channel:
		if open {
			t.Error("expected channel to be closed after RemoveClient")
		}
	default:
		t.Error("expected channel to be closed, but read would block")
	}
}

func TestRemoveClient_NonexistentClient(t *testing.T) {
	b := newTestBroker()
	// Must not panic.
	b.RemoveClient("nonexistent")
}

// --- Broadcast ---

func TestBroadcast_SendsToAllClients(t *testing.T) {
	b := newTestBroker()

	c1 := b.AddClient("c1")
	c2 := b.AddClient("c2")

	event := Event{Type: "test", Data: "hello"}
	b.Broadcast(event)

	for _, ch := range []chan Event{c1.Channel, c2.Channel} {
		select {
		case got := <-ch:
			if got.Type != "test" {
				t.Errorf("event type: got %s, want test", got.Type)
			}
		default:
			t.Error("expected event in channel")
		}
	}
}

func TestBroadcast_DropsOnFullChannel(t *testing.T) {
	b := newTestBroker()
	c := b.AddClient("c1")

	event := Event{Type: "test", Data: "x"}

	// Fill the channel (capacity 10).
	for i := 0; i < 10; i++ {
		b.Broadcast(event)
	}

	// This must not block even though the channel is full.
	done := make(chan struct{})
	go func() {
		b.Broadcast(event)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("Broadcast blocked on a full channel")
	}

	if len(c.Channel) != 10 {
		t.Errorf("expected 10 buffered events, got %d", len(c.Channel))
	}
}

func TestBroadcast_NoClients(t *testing.T) {
	b := newTestBroker()
	// Must not panic.
	b.Broadcast(Event{Type: "test", Data: "x"})
}

// --- BroadcastHostUpdate ---

func TestBroadcastHostUpdate(t *testing.T) {
	b := newTestBroker()
	c := b.AddClient("c1")

	b.BroadcastHostUpdate(HostUpdate{ID: "srv1", Status: "online"})

	got := <-c.Channel
	if got.Type != EventTypeHostUpdate {
		t.Errorf("event type: got %s, want %s", got.Type, EventTypeHostUpdate)
	}
	update, ok := got.Data.(HostUpdate)
	if !ok {
		t.Fatal("expected Data to be HostUpdate")
	}
	if update.ID != "srv1" {
		t.Errorf("host id: got %s, want srv1", update.ID)
	}
}

// --- BroadcastMetricsUpdate ---

func TestBroadcastMetricsUpdate_Minified(t *testing.T) {
	b := newTestBroker()
	// Per-host client receives metrics_update for its host.
	c := b.AddClientWithHostFilter("c1", "srv1")

	b.BroadcastMetricsUpdate(MetricsUpdate{
		HostID:          "srv1",
		Timestamp:       "2024-01-01T00:00:00Z",
		CPUUsagePercent: 42.5,
		MemoryUsedBytes: 1024,
	})

	got := <-c.Channel
	if got.Type != EventTypeMetricsUpdate {
		t.Errorf("event type: got %s, want %s", got.Type, EventTypeMetricsUpdate)
	}
	minified, ok := got.Data.(MetricsUpdateMinified)
	if !ok {
		t.Fatal("expected Data to be MetricsUpdateMinified")
	}
	if minified.HostID != "srv1" {
		t.Errorf("host id: got %s, want srv1", minified.HostID)
	}
	if minified.CPU != 42.5 {
		t.Errorf("cpu: got %f, want 42.5", minified.CPU)
	}
	if minified.MemoryUsed != 1024 {
		t.Errorf("memory used: got %d, want 1024", minified.MemoryUsed)
	}
}

func TestBroadcast_GlobalClient_SkipsPerHostMetrics(t *testing.T) {
	b := newTestBroker()
	global := b.AddClient("global")

	b.BroadcastMetricsUpdate(MetricsUpdate{HostID: "srv1", CPUUsagePercent: 42.5})
	b.BroadcastContainerMetricsUpdate(ContainerMetricsUpdate{HostID: "srv1"})

	if len(global.Channel) != 0 {
		t.Errorf("global client: expected 0 per-host metric events, got %d", len(global.Channel))
	}
}

// --- AddClientWithHostFilter ---

func TestAddClientWithHostFilter_SetsHostID(t *testing.T) {
	b := newTestBroker()

	client := b.AddClientWithHostFilter("c1", "host-abc")

	if client.HostID != "host-abc" {
		t.Errorf("HostID: got %q, want %q", client.HostID, "host-abc")
	}
	if _, ok := b.clients["c1"]; !ok {
		t.Error("expected client in broker map")
	}
}

// --- Per-host filtering ---

func TestBroadcast_HostFilteredClient_ReceivesMatchingHostEvents(t *testing.T) {
	b := newTestBroker()
	c := b.AddClientWithHostFilter("c1", "host-A")

	b.BroadcastHostUpdate(HostUpdate{ID: "host-A", Status: "online"})

	select {
	case got := <-c.Channel:
		if got.Type != EventTypeHostUpdate {
			t.Errorf("expected host_update, got %s", got.Type)
		}
	default:
		t.Error("expected event for matching host, got nothing")
	}
}

func TestBroadcast_HostFilteredClient_SkipsOtherHostEvents(t *testing.T) {
	b := newTestBroker()
	c := b.AddClientWithHostFilter("c1", "host-A")

	b.BroadcastHostUpdate(HostUpdate{ID: "host-B", Status: "online"})
	b.BroadcastMetricsUpdate(MetricsUpdate{HostID: "host-B"})

	// Channel must remain empty — no events for host-B should reach c.
	if len(c.Channel) != 0 {
		t.Errorf("expected empty channel, got %d events", len(c.Channel))
	}
}

func TestBroadcast_HostFilteredClient_SkipsAggregatedMetrics(t *testing.T) {
	b := newTestBroker()
	c := b.AddClientWithHostFilter("c1", "host-A")

	b.BroadcastAggregatedMetricsUpdate(AggregatedMetricsUpdate{CPUUsagePercent: 42.0})

	if len(c.Channel) != 0 {
		t.Errorf("expected aggregated_metrics_update to be filtered, got %d events", len(c.Channel))
	}
}

func TestBroadcast_GlobalClient_ReceivesAllHostEvents(t *testing.T) {
	b := newTestBroker()
	global := b.AddClient("global")

	b.BroadcastHostUpdate(HostUpdate{ID: "host-A", Status: "online"})
	b.BroadcastHostUpdate(HostUpdate{ID: "host-B", Status: "online"})
	b.BroadcastAggregatedMetricsUpdate(AggregatedMetricsUpdate{CPUUsagePercent: 10.0})

	if len(global.Channel) != 3 {
		t.Errorf("global client: expected 3 events, got %d", len(global.Channel))
	}
}

func TestBroadcast_MixedClients_FilteredAndGlobal(t *testing.T) {
	b := newTestBroker()
	global := b.AddClient("global")
	filtered := b.AddClientWithHostFilter("filtered", "host-A")

	b.BroadcastHostUpdate(HostUpdate{ID: "host-A", Status: "online"})
	b.BroadcastHostUpdate(HostUpdate{ID: "host-B", Status: "offline"})
	b.BroadcastAggregatedMetricsUpdate(AggregatedMetricsUpdate{CPUUsagePercent: 5.0})

	// Global receives all 3 events.
	if len(global.Channel) != 3 {
		t.Errorf("global client: expected 3 events, got %d", len(global.Channel))
	}
	// Filtered receives only the host-A event.
	if len(filtered.Channel) != 1 {
		t.Errorf("filtered client: expected 1 event, got %d", len(filtered.Channel))
	}
	got := <-filtered.Channel
	update := got.Data.(HostUpdate)
	if update.ID != "host-A" {
		t.Errorf("filtered client: expected host-A, got %s", update.ID)
	}
}

// --- BroadcastAggregatedMetricsUpdate ---

func TestBroadcastAggregatedMetricsUpdate(t *testing.T) {
	b := newTestBroker()
	c := b.AddClient("c1")

	b.BroadcastAggregatedMetricsUpdate(AggregatedMetricsUpdate{CPUUsagePercent: 10.0})

	got := <-c.Channel
	if got.Type != EventTypeAggregatedMetricsUpdate {
		t.Errorf("event type: got %s, want %s", got.Type, EventTypeAggregatedMetricsUpdate)
	}
}

// --- BroadcastContainerMetricsUpdate ---

func TestBroadcastContainerMetricsUpdate(t *testing.T) {
	b := newTestBroker()
	// Per-host client receives container_metrics_update for its host.
	c := b.AddClientWithHostFilter("c1", "srv1")

	b.BroadcastContainerMetricsUpdate(ContainerMetricsUpdate{HostID: "srv1"})

	got := <-c.Channel
	if got.Type != EventTypeContainerMetricsUpdate {
		t.Errorf("event type: got %s, want %s", got.Type, EventTypeContainerMetricsUpdate)
	}
}

// --- toMinifiedMetrics ---

func TestToMinifiedMetrics(t *testing.T) {
	input := MetricsUpdate{
		HostID:               "srv1",
		Timestamp:            "2024-01-15T12:00:00Z",
		CPUUsagePercent:      55.5,
		CPUIowaitPercent:     3.2,
		CPUStealPercent:      1.1,
		MemoryTotalBytes:     8000,
		MemoryUsedBytes:      4000,
		MemoryAvailableBytes: 4000,
		MemoryBuffersBytes:   512,
		MemoryCachedBytes:    1024,
		SwapTotalBytes:       2000,
		SwapUsedBytes:        500,
		LoadAvg1Min:          1.1,
		LoadAvg5Min:          1.5,
		LoadAvg15Min:         1.9,
		DiskTotalBytes:       500000,
		DiskUsedBytes:        250000,
		DiskReadBytesPerSec:  100,
		DiskWriteBytesPerSec: 200,
		NetworkRxBytesPerSec: 300,
		NetworkTxBytesPerSec: 400,
		CPUTemperatureCelsius: 65.0,
		UptimeSeconds:        3600,
		ProcessesCount:       142,
		SensorReadings:       []SensorReadingMinified{{K: "cpu", V: 65.0}},
	}

	got := toMinifiedMetrics(input)

	if got.HostID != "srv1" {
		t.Errorf("HostID: got %s, want srv1", got.HostID)
	}
	expectedTS, _ := time.Parse(time.RFC3339, "2024-01-15T12:00:00Z")
	if got.Timestamp != expectedTS.Unix() {
		t.Errorf("Timestamp: got %d, want %d", got.Timestamp, expectedTS.Unix())
	}
	if got.CPU != 55.5 {
		t.Errorf("CPU: got %f, want 55.5", got.CPU)
	}
	if got.CPUIowait != 3.2 {
		t.Errorf("CPUIowait: got %f, want 3.2", got.CPUIowait)
	}
	if got.CPUSteal != 1.1 {
		t.Errorf("CPUSteal: got %f, want 1.1", got.CPUSteal)
	}
	if got.MemoryTotal != 8000 {
		t.Errorf("MemoryTotal: got %d, want 8000", got.MemoryTotal)
	}
	if got.MemoryUsed != 4000 {
		t.Errorf("MemoryUsed: got %d, want 4000", got.MemoryUsed)
	}
	if got.MemoryAvailable != 4000 {
		t.Errorf("MemoryAvailable: got %d, want 4000", got.MemoryAvailable)
	}
	if got.MemoryBuffers != 512 {
		t.Errorf("MemoryBuffers: got %d, want 512", got.MemoryBuffers)
	}
	if got.MemoryCached != 1024 {
		t.Errorf("MemoryCached: got %d, want 1024", got.MemoryCached)
	}
	if got.LoadAvg1 != 1.1 {
		t.Errorf("LoadAvg1: got %f, want 1.1", got.LoadAvg1)
	}
	if got.LoadAvg5 != 1.5 {
		t.Errorf("LoadAvg5: got %f, want 1.5", got.LoadAvg5)
	}
	if got.LoadAvg15 != 1.9 {
		t.Errorf("LoadAvg15: got %f, want 1.9", got.LoadAvg15)
	}
	if got.DiskTotal != 500000 {
		t.Errorf("DiskTotal: got %d, want 500000", got.DiskTotal)
	}
	if got.DiskUsed != 250000 {
		t.Errorf("DiskUsed: got %d, want 250000", got.DiskUsed)
	}
	if got.DiskReadRate != 100 {
		t.Errorf("DiskReadRate: got %d, want 100", got.DiskReadRate)
	}
	if got.DiskWriteRate != 200 {
		t.Errorf("DiskWriteRate: got %d, want 200", got.DiskWriteRate)
	}
	if got.NetRxRate != 300 {
		t.Errorf("NetRxRate: got %d, want 300", got.NetRxRate)
	}
	if got.NetTxRate != 400 {
		t.Errorf("NetTxRate: got %d, want 400", got.NetTxRate)
	}
	if got.CPUTemp != 65.0 {
		t.Errorf("CPUTemp: got %f, want 65.0", got.CPUTemp)
	}
	if got.Uptime != 3600 {
		t.Errorf("Uptime: got %d, want 3600", got.Uptime)
	}
	if got.Processes != 142 {
		t.Errorf("Processes: got %d, want 142", got.Processes)
	}
	if got.SwapTotal != 2000 {
		t.Errorf("SwapTotal: got %d, want 2000", got.SwapTotal)
	}
	if got.SwapUsed != 500 {
		t.Errorf("SwapUsed: got %d, want 500", got.SwapUsed)
	}
	if len(got.SensorReadings) != 1 || got.SensorReadings[0].K != "cpu" {
		t.Error("sensor readings mismatch")
	}
}

func TestToMinifiedMetrics_InvalidTimestamp(t *testing.T) {
	got := toMinifiedMetrics(MetricsUpdate{Timestamp: "not-a-timestamp"})
	if got.Timestamp != 0 {
		t.Errorf("expected Timestamp=0 for invalid input, got %d", got.Timestamp)
	}
}

// --- FormatSSE ---

func TestFormatSSE(t *testing.T) {
	event := Event{Type: "host_update", Data: map[string]string{"id": "srv1"}}
	result := FormatSSE(event)

	if !strings.HasPrefix(result, "event: host_update\n") {
		t.Errorf("unexpected format: %q", result)
	}
	if !strings.Contains(result, "data: ") {
		t.Errorf("missing data line: %q", result)
	}
	if !strings.HasSuffix(result, "\n\n") {
		t.Errorf("expected trailing double newline: %q", result)
	}
}

func TestFormatSSE_MarshalError(t *testing.T) {
	// Channels cannot be marshaled to JSON.
	event := Event{Type: "test", Data: make(chan int)}
	if result := FormatSSE(event); result != "" {
		t.Errorf("expected empty string on marshal error, got %q", result)
	}
}

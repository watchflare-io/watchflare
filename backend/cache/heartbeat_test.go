package cache

import (
	"testing"
	"time"

	"watchflare/backend/models"
)

func TestGetCache_Singleton(t *testing.T) {
	c1 := GetCache()
	c2 := GetCache()
	if c1 != c2 {
		t.Error("expected GetCache to return the same instance")
	}
}

func TestUpdate_NewEntry(t *testing.T) {
	c := GetCache()
	c.clear()

	c.Update("agent1", "1.2.3.4", "::1")

	data, ok := c.Get("agent1")
	if !ok {
		t.Fatal("expected entry to exist")
	}
	if data.Status != models.StatusOnline {
		t.Errorf("status: got %s, want models.StatusOnline", data.Status)
	}
	if data.IPv4Address != "1.2.3.4" {
		t.Errorf("ipv4: got %s, want 1.2.3.4", data.IPv4Address)
	}
	if data.IPv6Address != "::1" {
		t.Errorf("ipv6: got %s, want ::1", data.IPv6Address)
	}
	if !data.Updated {
		t.Error("expected Updated=true")
	}
	if data.ClockDesync {
		t.Error("expected ClockDesync=false on new entry")
	}
}

func TestUpdate_ExistingEntry_ClearsClockDesync(t *testing.T) {
	c := GetCache()
	c.clear()

	c.Update("agent1", "1.2.3.4", "")
	c.SetClockDesync("agent1")
	c.MarkSynced("agent1")

	c.Update("agent1", "5.6.7.8", "::2")

	data, _ := c.Get("agent1")
	if data.IPv4Address != "5.6.7.8" {
		t.Errorf("ipv4: got %s, want 5.6.7.8", data.IPv4Address)
	}
	if data.ClockDesync {
		t.Error("expected ClockDesync cleared after successful update")
	}
	if !data.Updated {
		t.Error("expected Updated=true after update")
	}
}

func TestGet_NotFound(t *testing.T) {
	c := GetCache()
	c.clear()

	if _, ok := c.Get("nonexistent"); ok {
		t.Error("expected not found")
	}
}

func TestGet_ReturnsCopy(t *testing.T) {
	c := GetCache()
	c.clear()

	c.Update("agent1", "1.2.3.4", "")

	data, _ := c.Get("agent1")
	data.Status = "offline" // mutate the copy

	data2, _ := c.Get("agent1")
	if data2.Status != models.StatusOnline {
		t.Error("mutating the returned copy must not affect the cache")
	}
}

func TestGetAll_ReturnsCopies(t *testing.T) {
	c := GetCache()
	c.clear()

	c.Update("agent1", "1.2.3.4", "")
	c.Update("agent2", "5.6.7.8", "")

	all := c.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}

	for _, d := range all {
		d.Status = "offline"
	}

	for _, d := range c.GetAll() {
		if d.Status != models.StatusOnline {
			t.Error("mutating GetAll copies must not affect the cache")
		}
	}
}

func TestMarkSynced(t *testing.T) {
	c := GetCache()
	c.clear()

	c.Update("agent1", "", "")
	c.MarkSynced("agent1")

	data, _ := c.Get("agent1")
	if data.Updated {
		t.Error("expected Updated=false after MarkSynced")
	}
}

func TestMarkSynced_NonexistentAgent(t *testing.T) {
	c := GetCache()
	c.clear()

	// Must not panic.
	c.MarkSynced("nonexistent")
}

func TestCheckStale_TransitionsToOffline(t *testing.T) {
	c := GetCache()
	c.clear()

	c.Update("agent1", "", "")
	c.mu.Lock()
	c.cache["agent1"].LastSeen = time.Now().Add(-30 * time.Second)
	c.mu.Unlock()

	stale := c.CheckStale(15 * time.Second)

	if len(stale) != 1 || stale[0] != "agent1" {
		t.Errorf("expected [agent1], got %v", stale)
	}
	data, _ := c.Get("agent1")
	if data.Status != models.StatusOffline {
		t.Errorf("status: got %s, want models.StatusOffline", data.Status)
	}
	if !data.Updated {
		t.Error("expected Updated=true after stale transition")
	}
}

func TestCheckStale_SkipsAlreadyOffline(t *testing.T) {
	c := GetCache()
	c.clear()

	c.Update("agent1", "", "")
	c.mu.Lock()
	c.cache["agent1"].Status = "offline"
	c.cache["agent1"].LastSeen = time.Now().Add(-30 * time.Second)
	c.mu.Unlock()

	if stale := c.CheckStale(15 * time.Second); len(stale) != 0 {
		t.Errorf("expected no stale agents, got %v", stale)
	}
}

func TestCheckStale_SkipsFreshAgents(t *testing.T) {
	c := GetCache()
	c.clear()

	c.Update("agent1", "", "")

	if stale := c.CheckStale(15 * time.Second); len(stale) != 0 {
		t.Errorf("expected no stale agents, got %v", stale)
	}
}

func TestSetClockDesync_ExistingEntry(t *testing.T) {
	c := GetCache()
	c.clear()

	c.Update("agent1", "", "")
	c.MarkSynced("agent1")
	c.SetClockDesync("agent1")

	data, _ := c.Get("agent1")
	if !data.ClockDesync {
		t.Error("expected ClockDesync=true")
	}
	if !data.Updated {
		t.Error("expected Updated=true after SetClockDesync")
	}
}

func TestSetClockDesync_CreatesEntryIfMissing(t *testing.T) {
	c := GetCache()
	c.clear()

	c.SetClockDesync("agent1")

	data, ok := c.Get("agent1")
	if !ok {
		t.Fatal("expected entry to be created")
	}
	if !data.ClockDesync {
		t.Error("expected ClockDesync=true")
	}
	if data.Status != models.StatusOnline {
		t.Errorf("status: got %s, want models.StatusOnline", data.Status)
	}
}

func TestRemove(t *testing.T) {
	c := GetCache()
	c.clear()

	c.Update("agent1", "", "")
	c.Remove("agent1")

	if _, ok := c.Get("agent1"); ok {
		t.Error("expected agent to be removed")
	}
}

func TestRemove_NonexistentAgent(t *testing.T) {
	c := GetCache()
	c.clear()

	// Must not panic.
	c.Remove("nonexistent")
}

func TestPrimeFromHosts(t *testing.T) {
	c := GetCache()
	c.clear()

	lastSeen := time.Now().Add(-1 * time.Minute)
	ipv4 := "10.0.0.5"
	ipv6 := "fe80::1"

	hosts := []models.Host{
		{AgentID: "agent-online", Status: models.StatusOnline, LastSeen: &lastSeen, IPAddressV4: &ipv4, IPAddressV6: &ipv6},
		{AgentID: "agent-offline", Status: models.StatusOffline, LastSeen: &lastSeen, IPAddressV4: &ipv4},
		{AgentID: "agent-paused", Status: models.StatusPaused, LastSeen: &lastSeen},
		{AgentID: "agent-pending", Status: models.StatusPending},
		{AgentID: "agent-expired", Status: models.StatusExpired},
	}
	c.PrimeFromHosts(hosts)

	data, ok := c.Get("agent-online")
	if !ok {
		t.Fatal("expected online host to be primed")
	}
	if data.Status != models.StatusOnline {
		t.Errorf("status: got %s, want online", data.Status)
	}
	if !data.LastSeen.Equal(lastSeen) {
		t.Errorf("last_seen: got %s, want %s", data.LastSeen, lastSeen)
	}
	if data.IPv4Address != ipv4 {
		t.Errorf("ipv4: got %s, want %s", data.IPv4Address, ipv4)
	}
	if data.IPv6Address != ipv6 {
		t.Errorf("ipv6: got %s, want %s", data.IPv6Address, ipv6)
	}
	if data.Updated {
		t.Error("expected Updated=false on primed entry (no DB re-sync needed)")
	}

	for _, agentID := range []string{"agent-offline", "agent-paused", "agent-pending", "agent-expired"} {
		if _, ok := c.Get(agentID); ok {
			t.Errorf("expected %s not to be primed (non-online status)", agentID)
		}
	}
}

package cache

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"watchflare/backend/models"
)

// HeartbeatData represents cached heartbeat information for an agent.
type HeartbeatData struct {
	AgentID      string
	LastSeen     time.Time
	Status       string // models.StatusOnline or models.StatusOffline
	IPv4Address  string
	IPv6Address  string
	Updated      bool // true if changed since last DB sync
	ClockDesync  bool // true if agent's timestamp was rejected (clock out of sync)
}

// PendingCommand represents a command to be dispatched to an agent on next heartbeat.
type PendingCommand struct {
	CommandID string // UUID for deduplication
	Type      string // "collect_packages" | "update_agent"
}

// HeartbeatCache is an in-memory store for agent heartbeat state.
type HeartbeatCache struct {
	mu       sync.RWMutex
	cache    map[string]*HeartbeatData   // key: agent_id
	commands map[string][]PendingCommand // key: agent_id
}

var (
	globalCache *HeartbeatCache
	once        sync.Once
)

// GetCache returns the global HeartbeatCache singleton.
func GetCache() *HeartbeatCache {
	once.Do(func() {
		globalCache = &HeartbeatCache{
			cache:    make(map[string]*HeartbeatData),
			commands: make(map[string][]PendingCommand),
		}
	})
	return globalCache
}

// EnqueueCommand adds a command to the pending queue for the given agent.
// Returns the generated command ID.
func (c *HeartbeatCache) EnqueueCommand(agentID, commandType string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	cmd := PendingCommand{
		CommandID: uuid.New().String(),
		Type:      commandType,
	}
	c.commands[agentID] = append(c.commands[agentID], cmd)
	return cmd.CommandID
}

// ConsumeCommands atomically returns and clears all pending commands for the given agent.
func (c *HeartbeatCache) ConsumeCommands(agentID string) []PendingCommand {
	c.mu.Lock()
	defer c.mu.Unlock()

	cmds := c.commands[agentID]
	if len(cmds) == 0 {
		return nil
	}
	delete(c.commands, agentID)
	return cmds
}

func (c *HeartbeatCache) Update(agentID, ipv4, ipv6 string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	if existing, ok := c.cache[agentID]; ok {
		existing.LastSeen = now
		existing.Status = models.StatusOnline
		existing.IPv4Address = ipv4
		existing.IPv6Address = ipv6
		existing.Updated = true
		existing.ClockDesync = false
	} else {
		c.cache[agentID] = &HeartbeatData{
			AgentID:     agentID,
			LastSeen:    now,
			Status:      models.StatusOnline,
			IPv4Address: ipv4,
			IPv6Address: ipv6,
			Updated:     true,
		}
	}
}

func (c *HeartbeatCache) Get(agentID string) (*HeartbeatData, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, ok := c.cache[agentID]
	if !ok {
		return nil, false
	}

	// Return a copy to prevent the caller from mutating cached state.
	return &HeartbeatData{
		AgentID:     data.AgentID,
		LastSeen:    data.LastSeen,
		Status:      data.Status,
		IPv4Address: data.IPv4Address,
		IPv6Address: data.IPv6Address,
		Updated:     data.Updated,
		ClockDesync: data.ClockDesync,
	}, true
}

func (c *HeartbeatCache) GetAll() []*HeartbeatData {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*HeartbeatData, 0, len(c.cache))
	for _, data := range c.cache {
		result = append(result, &HeartbeatData{
			AgentID:     data.AgentID,
			LastSeen:    data.LastSeen,
			Status:      data.Status,
			IPv4Address: data.IPv4Address,
			IPv6Address: data.IPv6Address,
			Updated:     data.Updated,
			ClockDesync: data.ClockDesync,
		})
	}
	return result
}

func (c *HeartbeatCache) MarkSynced(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if data, ok := c.cache[agentID]; ok {
		data.Updated = false
	}
}

// CheckStale transitions online agents to offline if they haven't sent a heartbeat
// within timeout. Returns the list of agent IDs that were just transitioned.
func (c *HeartbeatCache) CheckStale(timeout time.Duration) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	var staleAgents []string

	for agentID, data := range c.cache {
		if data.Status == models.StatusOnline && now.Sub(data.LastSeen) > timeout {
			data.Status = models.StatusOffline
			data.Updated = true
			staleAgents = append(staleAgents, agentID)
		}
	}

	return staleAgents
}

// SetClockDesync flags an agent as having a clock out of sync.
// If the agent is not yet in the cache (first heartbeat failed), a new entry is created.
func (c *HeartbeatCache) SetClockDesync(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if data, ok := c.cache[agentID]; ok {
		data.ClockDesync = true
		data.Updated = true
	} else {
		c.cache[agentID] = &HeartbeatData{
			AgentID:     agentID,
			LastSeen:    time.Now(),
			Status:      models.StatusOnline,
			ClockDesync: true,
			Updated:     true,
		}
	}
}

// PrimeFromHosts seeds the cache from hosts loaded from the database. Used at
// boot to reconcile in-memory state with the last-known DB state. Only online
// hosts are added; paused/offline/pending/expired hosts don't need a cache
// entry. Updated is set to false so the sync worker doesn't re-write these
// values back to the DB. The stale checker will transition entries to offline
// if no heartbeat arrives within its timeout.
func (c *HeartbeatCache) PrimeFromHosts(hosts []models.Host) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, h := range hosts {
		if h.Status != models.StatusOnline {
			continue
		}
		lastSeen := time.Now()
		if h.LastSeen != nil {
			lastSeen = *h.LastSeen
		}
		ipv4 := ""
		if h.IPAddressV4 != nil {
			ipv4 = *h.IPAddressV4
		}
		ipv6 := ""
		if h.IPAddressV6 != nil {
			ipv6 = *h.IPAddressV6
		}
		c.cache[h.AgentID] = &HeartbeatData{
			AgentID:     h.AgentID,
			LastSeen:    lastSeen,
			Status:      models.StatusOnline,
			IPv4Address: ipv4,
			IPv6Address: ipv6,
			Updated:     false,
		}
	}
}

func (c *HeartbeatCache) Remove(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, agentID)
}

// clear resets the cache. Used in tests only.
func (c *HeartbeatCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*HeartbeatData)
}

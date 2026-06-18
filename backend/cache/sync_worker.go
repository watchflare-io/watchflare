package cache

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"watchflare/backend/database"
	"watchflare/backend/models"
	"watchflare/backend/sse"

	"gorm.io/gorm"
)

// SyncWorker periodically flushes updated heartbeat entries to the database.
type SyncWorker struct {
	cache    *HeartbeatCache
	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewSyncWorker(interval time.Duration) *SyncWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &SyncWorker{
		cache:    GetCache(),
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start runs the sync loop. Call in a goroutine.
func (w *SyncWorker) Start() {
	slog.Info("heartbeat sync worker started", "interval", w.interval)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.syncToDatabase()
		case <-w.ctx.Done():
			slog.Info("heartbeat sync worker stopped")
			return
		}
	}
}

func (w *SyncWorker) Stop() {
	w.cancel()
}

func (w *SyncWorker) syncToDatabase() {
	allData := w.cache.GetAll()
	syncCount := 0

	for _, data := range allData {
		if !data.Updated {
			continue
		}

		var host models.Host
		result := database.DB.Where("agent_id = ?", data.AgentID).First(&host)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				slog.Warn("agent not found in database, skipping sync", "agent_id", data.AgentID)
			} else {
				slog.Error("failed to query agent", "agent_id", data.AgentID, "error", result.Error)
			}
			continue
		}

		updates := map[string]interface{}{
			"last_seen":     data.LastSeen,
			"status":        data.Status,
			"ip_address_v4": data.IPv4Address,
			"ip_address_v6": data.IPv6Address,
		}

		if err := database.DB.Model(&models.Host{}).Where("agent_id = ?", data.AgentID).Updates(updates).Error; err != nil {
			slog.Error("failed to sync heartbeat", "agent_id", data.AgentID, "error", err)
			continue
		}

		w.cache.MarkSynced(data.AgentID)
		syncCount++
	}

	if syncCount > 0 {
		slog.Info("synced heartbeats to database", "count", syncCount)
	}
}

// StaleChecker periodically transitions agents to offline when no heartbeat is received
// within the configured timeout.
type StaleChecker struct {
	cache    *HeartbeatCache
	interval time.Duration
	timeout  time.Duration // duration without heartbeat before an agent is marked offline
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewStaleChecker(interval, timeout time.Duration) *StaleChecker {
	ctx, cancel := context.WithCancel(context.Background())
	return &StaleChecker{
		cache:    GetCache(),
		interval: interval,
		timeout:  timeout,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start runs the stale-check loop. Call in a goroutine.
func (c *StaleChecker) Start() {
	slog.Info("heartbeat stale checker started", "interval", c.interval, "timeout", c.timeout)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.checkStaleAgents()
			c.promoteStalePending()
		case <-c.ctx.Done():
			slog.Info("heartbeat stale checker stopped")
			return
		}
	}
}

func (c *StaleChecker) Stop() {
	c.cancel()
}

func (c *StaleChecker) checkStaleAgents() {
	staleAgents := c.cache.CheckStale(c.timeout)

	if len(staleAgents) == 0 {
		return
	}

	for _, agentID := range staleAgents {
		var host models.Host
		if err := database.DB.Where("agent_id = ?", agentID).First(&host).Error; err != nil {
			slog.Warn("stale agent not found in database", "agent_id", agentID)
			continue
		}

		data, ok := c.cache.Get(agentID)
		if !ok {
			continue
		}

		configuredIP := ""
		if host.ConfiguredIP != nil {
			configuredIP = *host.ConfiguredIP
		}

		sse.GetBroker().BroadcastHostUpdate(sse.HostUpdate{
			ID:               host.ID,
			Status:           models.StatusOffline,
			IPv4Address:      data.IPv4Address,
			IPv6Address:      data.IPv6Address,
			ConfiguredIP:     configuredIP,
			IgnoreIPMismatch: host.IgnoreIPMismatch,
			LastSeen:         data.LastSeen.Format(time.RFC3339),
			ClockDesync:      data.ClockDesync,
		})

		slog.Warn("agent marked as offline", "agent_id", agentID, "stale_after", c.timeout)
	}
}

// promoteStalePending demotes hosts left "pending" after a resume when no
// heartbeat arrives within the stale-check timeout. Newly created hosts (no
// prior last_seen) are left alone so the registration flow is not disturbed.
func (c *StaleChecker) promoteStalePending() {
	cutoff := time.Now().Add(-c.timeout)

	var hosts []models.Host
	if err := database.DB.
		Where("status = ? AND last_seen IS NOT NULL AND last_seen < ?", models.StatusPending, cutoff).
		Find(&hosts).Error; err != nil {
		slog.Error("failed to query stale pending hosts", "error", err)
		return
	}

	for i := range hosts {
		h := &hosts[i]
		if err := database.DB.Model(h).Update("status", models.StatusOffline).Error; err != nil {
			slog.Error("failed to promote pending to offline", "host_id", h.ID, "error", err)
			continue
		}

		configuredIP := ""
		if h.ConfiguredIP != nil {
			configuredIP = *h.ConfiguredIP
		}
		ipv4 := ""
		if h.IPAddressV4 != nil {
			ipv4 = *h.IPAddressV4
		}
		ipv6 := ""
		if h.IPAddressV6 != nil {
			ipv6 = *h.IPAddressV6
		}
		lastSeen := ""
		if h.LastSeen != nil {
			lastSeen = h.LastSeen.Format(time.RFC3339)
		}

		sse.GetBroker().BroadcastHostUpdate(sse.HostUpdate{
			ID:               h.ID,
			Status:           models.StatusOffline,
			IPv4Address:      ipv4,
			IPv6Address:      ipv6,
			ConfiguredIP:     configuredIP,
			IgnoreIPMismatch: h.IgnoreIPMismatch,
			LastSeen:         lastSeen,
		})

		slog.Info("promoted stale pending host to offline", "host_id", h.ID, "stale_after", c.timeout)
	}
}

package services

import (
	"context"
	"log/slog"
	"time"
	"watchflare/backend/database"
	"watchflare/backend/sse"
)

// AggregatedMetricsScheduler periodically calculates and broadcasts aggregated metrics.
type AggregatedMetricsScheduler struct {
	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewAggregatedMetricsScheduler(interval time.Duration) *AggregatedMetricsScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &AggregatedMetricsScheduler{
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start runs the scheduler. Call in a goroutine.
// It waits for the next interval boundary plus a 2s processing delay before each run,
// giving agents time to deliver their metrics (typically 50–500ms).
func (s *AggregatedMetricsScheduler) Start() {
	const processingDelay = 2 * time.Second

	slog.Info("aggregated metrics scheduler starting", "interval", s.interval, "processing_delay", processingDelay)

	for {
		now := time.Now()
		nextTick := now.Truncate(s.interval).Add(s.interval).Add(processingDelay)
		delay := nextTick.Sub(now)

		select {
		case <-time.After(delay):
			s.calculateAndBroadcast()
		case <-s.ctx.Done():
			slog.Info("aggregated metrics scheduler stopped")
			return
		}
	}
}

func (s *AggregatedMetricsScheduler) Stop() {
	s.cancel()
}

func (s *AggregatedMetricsScheduler) calculateAndBroadcast() {
	now := time.Now()
	bucketTime := now.Truncate(s.interval)

	// Look back 3× the interval to tolerate agent timing drift.
	// DISTINCT ON ensures we only take the latest metric per host,
	// so there is no double-counting even with a wider window.
	lookback := now.Add(-3 * s.interval)

	query := `
		SELECT
			COALESCE(AVG(latest.cpu_usage_percent), 0) as cpu_usage_percent,
			COALESCE(SUM(latest.memory_total_bytes), 0) as memory_total_bytes,
			COALESCE(SUM(latest.memory_used_bytes), 0) as memory_used_bytes,
			COALESCE(SUM(CASE WHEN latest.environment_type != 'container' THEN latest.disk_total_bytes ELSE 0 END), 0) as disk_total_bytes,
			COALESCE(SUM(CASE WHEN latest.environment_type != 'container' THEN latest.disk_used_bytes ELSE 0 END), 0) as disk_used_bytes,
			COALESCE(AVG(latest.load_avg1_min), 0) as load_avg_1min,
			COALESCE(AVG(latest.load_avg5_min), 0) as load_avg_5min,
			COALESCE(AVG(latest.load_avg15_min), 0) as load_avg_15min
		FROM (
			SELECT DISTINCT ON (m.host_id)
				m.cpu_usage_percent,
				m.memory_total_bytes,
				m.memory_used_bytes,
				m.disk_total_bytes,
				m.disk_used_bytes,
				m.load_avg1_min,
				m.load_avg5_min,
				m.load_avg15_min,
				s.environment_type
			FROM metrics m
			JOIN hosts s ON m.host_id = s.id
			WHERE s.status = 'online'
			  AND m.timestamp > $1
			ORDER BY m.host_id, m.timestamp DESC
		) latest
	`

	var cpuUsagePercent, loadAvg1Min, loadAvg5Min, loadAvg15Min float64
	var memoryTotalBytes, memoryUsedBytes, diskTotalBytes, diskUsedBytes uint64

	if err := database.DB.Raw(query, lookback).Row().Scan(
		&cpuUsagePercent, &memoryTotalBytes, &memoryUsedBytes, &diskTotalBytes, &diskUsedBytes,
		&loadAvg1Min, &loadAvg5Min, &loadAvg15Min,
	); err != nil {
		slog.Error("failed to calculate aggregated metrics", "error", err)
		return
	}

	// Skip broadcasting when all hosts are paused or offline.
	if memoryTotalBytes == 0 && diskTotalBytes == 0 && cpuUsagePercent == 0 {
		return
	}

	sse.GetBroker().BroadcastAggregatedMetricsUpdate(sse.AggregatedMetricsUpdate{
		Timestamp:            bucketTime.Format(time.RFC3339),
		CPUUsagePercent:      cpuUsagePercent,
		MemoryTotalBytes:     memoryTotalBytes,
		MemoryUsedBytes:      memoryUsedBytes,
		MemoryAvailableBytes: memoryTotalBytes - memoryUsedBytes,
		DiskTotalBytes:       diskTotalBytes,
		DiskUsedBytes:        diskUsedBytes,
		LoadAvg1Min:          loadAvg1Min,
		LoadAvg5Min:          loadAvg5Min,
		LoadAvg15Min:         loadAvg15Min,
	})
}

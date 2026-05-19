package services

import (
	"errors"
	"fmt"
	"log/slog"
	"time"
	"watchflare/backend/database"
)

// ErrInvalidTimeRange is returned when an unknown time_range value is requested.
var ErrInvalidTimeRange = errors.New("invalid time_range")

// DroppedMetricsAlert represents a dropped metrics alert for the dashboard.
type DroppedMetricsAlert struct {
	HostID           string        `json:"host_id"`
	Hostname         string        `json:"hostname"`
	TotalDropped     int           `json:"total_dropped"`
	FirstDroppedAt   time.Time     `json:"first_dropped_at"`
	LastDroppedAt    time.Time     `json:"last_dropped_at"`
	LastReportedAt   time.Time     `json:"last_reported_at"`
	DowntimeDuration time.Duration `json:"downtime_duration"`
}

// AggregatedPoint represents one cross-host aggregated metrics data point.
type AggregatedPoint struct {
	Timestamp            time.Time `json:"timestamp"`
	CPUUsagePercent      float64   `json:"cpu_usage_percent"`
	MemoryTotalBytes     uint64    `json:"memory_total_bytes"`
	MemoryUsedBytes      uint64    `json:"memory_used_bytes"`
	MemoryAvailableBytes uint64    `json:"memory_available_bytes"`
	DiskTotalBytes       uint64    `json:"disk_total_bytes"`
	DiskUsedBytes        uint64    `json:"disk_used_bytes"`
	LoadAvg1Min          float64   `json:"load_avg_1min"`
	LoadAvg5Min          float64   `json:"load_avg_5min"`
	LoadAvg15Min         float64   `json:"load_avg_15min"`
}

// aggregateConfig holds the parameters for a continuous-aggregate time range.
type aggregateConfig struct {
	duration       time.Duration
	bucketDuration time.Duration
	table          string
	bucketInterval string
}

// aggregateConfigs maps time_range strings to their aggregate table parameters.
var aggregateConfigs = map[string]aggregateConfig{
	"12h": {12 * time.Hour, 10 * time.Minute, "metrics_10min", "10 minutes"},
	"24h": {24 * time.Hour, 15 * time.Minute, "metrics_15min", "15 minutes"},
	"7d":  {7 * 24 * time.Hour, 2 * time.Hour, "metrics_2h", "2 hours"},
	"30d": {30 * 24 * time.Hour, 8 * time.Hour, "metrics_8h", "8 hours"},
}

// buildAggregatedQuery builds a cross-host aggregation query that combines
// data from a continuous aggregate with raw metrics for the recent gap period.
// Timestamps are shifted by +bucketInterval so they represent the END of each bucket
// (e.g. label "08:40" = average of data from 08:30 to 08:40).
// Args: $1=adjustedStart, $2=gapStart (CA exclusive), $3=gapStart (raw inclusive), $4=currentBucket (raw exclusive).
// aggregateTable and bucketInterval come from the hardcoded aggregateConfigs map — no SQL injection risk.
func buildAggregatedQuery(aggregateTable, bucketInterval string) string {
	return fmt.Sprintf(`
		WITH per_host_data AS (
			SELECT m.bucket + INTERVAL '%s' as ts, m.host_id, m.cpu_usage_percent,
				   m.memory_total_bytes, m.memory_used_bytes,
				   m.disk_total_bytes, m.disk_used_bytes,
				   m.load_avg1_min, m.load_avg5_min, m.load_avg15_min
			FROM %s m
			WHERE m.bucket > $1 AND m.bucket < $2

			UNION ALL

			SELECT time_bucket('%s', m.timestamp) + INTERVAL '%s' as ts, m.host_id,
				   AVG(m.cpu_usage_percent) as cpu_usage_percent,
				   AVG(m.memory_total_bytes) as memory_total_bytes,
				   AVG(m.memory_used_bytes) as memory_used_bytes,
				   AVG(m.disk_total_bytes) as disk_total_bytes,
				   AVG(m.disk_used_bytes) as disk_used_bytes,
				   AVG(m.load_avg1_min) as load_avg1_min,
				   AVG(m.load_avg5_min) as load_avg5_min,
				   AVG(m.load_avg15_min) as load_avg15_min
			FROM metrics m
			WHERE m.timestamp >= $3 AND m.timestamp < $4
			GROUP BY time_bucket('%s', m.timestamp), m.host_id
		)
		SELECT
			d.ts as timestamp,
			COALESCE(AVG(d.cpu_usage_percent), 0) as cpu_usage_percent,
			COALESCE(SUM(d.memory_total_bytes), 0)::BIGINT as memory_total_bytes,
			COALESCE(SUM(d.memory_used_bytes), 0)::BIGINT as memory_used_bytes,
			COALESCE(SUM(CASE WHEN s.environment_type != 'container' THEN d.disk_total_bytes ELSE 0 END), 0)::BIGINT as disk_total_bytes,
			COALESCE(SUM(CASE WHEN s.environment_type != 'container' THEN d.disk_used_bytes ELSE 0 END), 0)::BIGINT as disk_used_bytes,
			COALESCE(AVG(d.load_avg1_min), 0) as load_avg_1min,
			COALESCE(AVG(d.load_avg5_min), 0) as load_avg_5min,
			COALESCE(AVG(d.load_avg15_min), 0) as load_avg_15min
		FROM per_host_data d
		JOIN hosts s ON d.host_id = s.id
		WHERE s.status NOT IN ('expired', 'pending')
		GROUP BY d.ts
		ORDER BY d.ts ASC
	`, bucketInterval, aggregateTable, bucketInterval, bucketInterval, bucketInterval)
}

// GetDroppedMetrics returns a summary of dropped metrics from the last 24 hours.
func GetDroppedMetrics() ([]DroppedMetricsAlert, error) {
	rows, err := database.DB.Raw(`
		SELECT host_id, hostname, total_dropped,
		       first_dropped_at, last_dropped_at, last_reported_at
		FROM agent_dropped_metrics_summary
		ORDER BY total_dropped DESC
	`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []DroppedMetricsAlert
	for rows.Next() {
		var alert DroppedMetricsAlert
		if err := rows.Scan(
			&alert.HostID,
			&alert.Hostname,
			&alert.TotalDropped,
			&alert.FirstDroppedAt,
			&alert.LastDroppedAt,
			&alert.LastReportedAt,
		); err != nil {
			return nil, err
		}
		alert.DowntimeDuration = alert.LastDroppedAt.Sub(alert.FirstDroppedAt)
		alerts = append(alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if alerts == nil {
		alerts = []DroppedMetricsAlert{}
	}
	return alerts, nil
}

// GetAggregatedMetrics returns cross-host aggregated metrics for the given time range.
// Returns ErrInvalidTimeRange if timeRange is not a recognised value.
func GetAggregatedMetrics(timeRange string) ([]AggregatedPoint, error) {
	var query string
	var queryArgs []interface{}

	if timeRange == "1h" {
		endTime := time.Now()
		startTime := endTime.Add(-time.Hour)
		query = `
			WITH time_buckets AS (
				SELECT time_bucket('30 seconds'::interval, m.timestamp) as bucket,
					   m.host_id,
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
				WHERE s.status NOT IN ('expired', 'pending')
				  AND m.timestamp > $1
				  AND m.timestamp <= $2
			),
			host_aggregates AS (
				SELECT
					bucket,
					host_id,
					COALESCE(AVG(cpu_usage_percent), 0) as cpu_usage_percent,
					COALESCE(AVG(memory_total_bytes), 0) as memory_total_bytes,
					COALESCE(AVG(memory_used_bytes), 0) as memory_used_bytes,
					COALESCE(AVG(disk_total_bytes), 0) as disk_total_bytes,
					COALESCE(AVG(disk_used_bytes), 0) as disk_used_bytes,
					COALESCE(AVG(load_avg1_min), 0) as load_avg1_min,
					COALESCE(AVG(load_avg5_min), 0) as load_avg5_min,
					COALESCE(AVG(load_avg15_min), 0) as load_avg15_min,
					MAX(environment_type) as environment_type
				FROM time_buckets
				GROUP BY bucket, host_id
			)
			SELECT
				bucket as timestamp,
				COALESCE(AVG(cpu_usage_percent), 0) as cpu_usage_percent,
				COALESCE(SUM(memory_total_bytes), 0)::BIGINT as memory_total_bytes,
				COALESCE(SUM(memory_used_bytes), 0)::BIGINT as memory_used_bytes,
				COALESCE(SUM(CASE WHEN environment_type != 'container' THEN disk_total_bytes ELSE 0 END), 0)::BIGINT as disk_total_bytes,
				COALESCE(SUM(CASE WHEN environment_type != 'container' THEN disk_used_bytes ELSE 0 END), 0)::BIGINT as disk_used_bytes,
				COALESCE(AVG(load_avg1_min), 0) as load_avg_1min,
				COALESCE(AVG(load_avg5_min), 0) as load_avg_5min,
				COALESCE(AVG(load_avg15_min), 0) as load_avg_15min
			FROM host_aggregates
			GROUP BY bucket
			ORDER BY bucket ASC
		`
		queryArgs = []interface{}{startTime, endTime}
	} else {
		cfg, ok := aggregateConfigs[timeRange]
		if !ok {
			return nil, ErrInvalidTimeRange
		}
		endTime := time.Now()
		startTime := endTime.Add(-cfg.duration)
		currentBucket := endTime.Truncate(cfg.bucketDuration)
		gapStart := currentBucket.Add(-2 * cfg.bucketDuration)
		adjustedStart := startTime.Add(-cfg.bucketDuration)

		query = buildAggregatedQuery(cfg.table, cfg.bucketInterval)
		queryArgs = []interface{}{adjustedStart, gapStart, gapStart, currentBucket}
	}

	rows, err := database.DB.Raw(query, queryArgs...).Rows()
	if err != nil {
		slog.Error("failed to query aggregated metrics", "error", err)
		return nil, err
	}
	defer rows.Close()

	var points []AggregatedPoint
	for rows.Next() {
		var point AggregatedPoint
		if err := rows.Scan(
			&point.Timestamp,
			&point.CPUUsagePercent,
			&point.MemoryTotalBytes,
			&point.MemoryUsedBytes,
			&point.DiskTotalBytes,
			&point.DiskUsedBytes,
			&point.LoadAvg1Min,
			&point.LoadAvg5Min,
			&point.LoadAvg15Min,
		); err != nil {
			slog.Error("failed to scan aggregated metrics", "error", err)
			return nil, err
		}
		point.MemoryAvailableBytes = point.MemoryTotalBytes - point.MemoryUsedBytes
		points = append(points, point)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if points == nil {
		points = []AggregatedPoint{}
	}
	return points, nil
}

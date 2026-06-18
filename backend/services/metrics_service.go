package services

import (
	"errors"
	"fmt"
	"time"
	"watchflare/backend/database"
	"watchflare/backend/models"

	"gorm.io/gorm"
)

// MetricsQueryParams holds parameters for metrics retrieval.
type MetricsQueryParams struct {
	HostID   string
	Start    time.Time
	End      time.Time
	Interval string // e.g., "1m", "5m", "15m", "1h"
}

// MetricDataPoint represents an aggregated metric data point.
type MetricDataPoint struct {
	Timestamp             time.Time             `json:"timestamp"`
	CPUUsagePercent       float64               `json:"cpu_usage_percent"`
	CPUIowaitPercent      float64               `json:"cpu_iowait_percent"`
	CPUStealPercent       float64               `json:"cpu_steal_percent"`
	MemoryTotalBytes      uint64                `json:"memory_total_bytes"`
	MemoryUsedBytes       uint64                `json:"memory_used_bytes"`
	MemoryAvailableBytes  uint64                `json:"memory_available_bytes"`
	MemoryBuffersBytes    uint64                `json:"memory_buffers_bytes"`
	MemoryCachedBytes     uint64                `json:"memory_cached_bytes"`
	SwapTotalBytes        uint64                `json:"swap_total_bytes"`
	SwapUsedBytes         uint64                `json:"swap_used_bytes"`
	LoadAvg1Min           float64               `json:"load_avg_1min"`
	LoadAvg5Min           float64               `json:"load_avg_5min"`
	LoadAvg15Min          float64               `json:"load_avg_15min"`
	DiskTotalBytes        uint64                `json:"disk_total_bytes"`
	DiskUsedBytes         uint64                `json:"disk_used_bytes"`
	DiskReadBytesPerSec   uint64                `json:"disk_read_bytes_per_sec"`
	DiskWriteBytesPerSec  uint64                `json:"disk_write_bytes_per_sec"`
	NetworkRxBytesPerSec  uint64                `json:"network_rx_bytes_per_sec"`
	NetworkTxBytesPerSec  uint64                `json:"network_tx_bytes_per_sec"`
	CPUTemperatureCelsius float64               `json:"cpu_temperature_celsius"`
	UptimeSeconds         uint64                `json:"uptime_seconds"`
	ProcessesCount        uint64                `json:"processes_count"`
	SensorReadings        models.SensorReadings `json:"sensor_readings,omitempty"`
}

// aggregatedMetricRow scans continuous aggregate query results.
// SensorReadings is intentionally omitted — JSONB columns are absent from aggregate views.
type aggregatedMetricRow struct {
	Timestamp             time.Time `gorm:"column:timestamp"`
	CPUUsagePercent       float64   `gorm:"column:cpu_usage_percent"`
	MemoryTotalBytes      uint64    `gorm:"column:memory_total_bytes"`
	MemoryUsedBytes       uint64    `gorm:"column:memory_used_bytes"`
	MemoryAvailableBytes  uint64    `gorm:"column:memory_available_bytes"`
	LoadAvg1Min           float64   `gorm:"column:load_avg_1min"`
	LoadAvg5Min           float64   `gorm:"column:load_avg_5min"`
	LoadAvg15Min          float64   `gorm:"column:load_avg_15min"`
	DiskTotalBytes        uint64    `gorm:"column:disk_total_bytes"`
	DiskUsedBytes         uint64    `gorm:"column:disk_used_bytes"`
	DiskReadBytesPerSec   uint64    `gorm:"column:disk_read_bytes_per_sec"`
	DiskWriteBytesPerSec  uint64    `gorm:"column:disk_write_bytes_per_sec"`
	NetworkRxBytesPerSec  uint64    `gorm:"column:network_rx_bytes_per_sec"`
	NetworkTxBytesPerSec  uint64    `gorm:"column:network_tx_bytes_per_sec"`
	CPUTemperatureCelsius float64   `gorm:"column:cpu_temperature_celsius"`
	UptimeSeconds         uint64    `gorm:"column:uptime_seconds"`
}

// GetMetrics retrieves metrics for a host. When Interval is empty, raw data is
// returned; otherwise the appropriate continuous aggregate view is used.
func GetMetrics(params MetricsQueryParams) ([]MetricDataPoint, error) {
	var host models.Host
	if err := database.DB.Where("id = ?", params.HostID).First(&host).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHostNotFound
		}
		return nil, err
	}

	if params.Interval == "" {
		var metrics []models.Metric
		if err := database.DB.Where("host_id = ? AND timestamp >= ? AND timestamp <= ?",
			params.HostID, params.Start, params.End).
			Order("timestamp ASC").
			Find(&metrics).Error; err != nil {
			return nil, err
		}

		results := make([]MetricDataPoint, len(metrics))
		for i, m := range metrics {
			results[i] = MetricDataPoint{
				Timestamp:             m.Timestamp,
				CPUUsagePercent:       m.CPUUsagePercent,
				CPUIowaitPercent:      m.CPUIowaitPercent,
				CPUStealPercent:       m.CPUStealPercent,
				MemoryTotalBytes:      m.MemoryTotalBytes,
				MemoryUsedBytes:       m.MemoryUsedBytes,
				MemoryAvailableBytes:  m.MemoryAvailableBytes,
				MemoryBuffersBytes:    m.MemoryBuffersBytes,
				MemoryCachedBytes:     m.MemoryCachedBytes,
				SwapTotalBytes:        m.SwapTotalBytes,
				SwapUsedBytes:         m.SwapUsedBytes,
				LoadAvg1Min:           m.LoadAvg1Min,
				LoadAvg5Min:           m.LoadAvg5Min,
				LoadAvg15Min:          m.LoadAvg15Min,
				DiskTotalBytes:        m.DiskTotalBytes,
				DiskUsedBytes:         m.DiskUsedBytes,
				DiskReadBytesPerSec:   m.DiskReadBytesPerSec,
				DiskWriteBytesPerSec:  m.DiskWriteBytesPerSec,
				NetworkRxBytesPerSec:  m.NetworkRxBytesPerSec,
				NetworkTxBytesPerSec:  m.NetworkTxBytesPerSec,
				CPUTemperatureCelsius: m.CPUTemperatureCelsius,
				UptimeSeconds:         m.UptimeSeconds,
				ProcessesCount:        m.ProcessesCount,
				SensorReadings:        m.SensorReadings,
			}
		}
		return results, nil
	}

	// Use pre-calculated continuous aggregates for better performance.
	tableName, err := getContinuousAggregateTable(params.Interval)
	if err != nil {
		return nil, err
	}

	// tableName comes from a whitelist map — no SQL injection risk.
	query := fmt.Sprintf(`
		SELECT
			bucket AS timestamp,
			cpu_usage_percent,
			CAST(memory_total_bytes AS BIGINT) AS memory_total_bytes,
			CAST(memory_used_bytes AS BIGINT) AS memory_used_bytes,
			CAST(memory_available_bytes AS BIGINT) AS memory_available_bytes,
			load_avg1_min AS load_avg_1min,
			load_avg5_min AS load_avg_5min,
			load_avg15_min AS load_avg_15min,
			CAST(disk_total_bytes AS BIGINT) AS disk_total_bytes,
			CAST(disk_used_bytes AS BIGINT) AS disk_used_bytes,
			CAST(disk_read_bytes_per_sec AS BIGINT) AS disk_read_bytes_per_sec,
			CAST(disk_write_bytes_per_sec AS BIGINT) AS disk_write_bytes_per_sec,
			CAST(network_rx_bytes_per_sec AS BIGINT) AS network_rx_bytes_per_sec,
			CAST(network_tx_bytes_per_sec AS BIGINT) AS network_tx_bytes_per_sec,
			cpu_temperature_celsius,
			CAST(uptime_seconds AS BIGINT) AS uptime_seconds
		FROM %s
		WHERE host_id = $1 AND bucket >= $2 AND bucket <= $3
		ORDER BY bucket ASC
	`, tableName)

	var rows []aggregatedMetricRow
	if err := database.DB.Raw(query, params.HostID, params.Start, params.End).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	results := make([]MetricDataPoint, len(rows))
	for i, r := range rows {
		results[i] = MetricDataPoint{
			Timestamp:             r.Timestamp,
			CPUUsagePercent:       r.CPUUsagePercent,
			MemoryTotalBytes:      r.MemoryTotalBytes,
			MemoryUsedBytes:       r.MemoryUsedBytes,
			MemoryAvailableBytes:  r.MemoryAvailableBytes,
			LoadAvg1Min:           r.LoadAvg1Min,
			LoadAvg5Min:           r.LoadAvg5Min,
			LoadAvg15Min:          r.LoadAvg15Min,
			DiskTotalBytes:        r.DiskTotalBytes,
			DiskUsedBytes:         r.DiskUsedBytes,
			DiskReadBytesPerSec:   r.DiskReadBytesPerSec,
			DiskWriteBytesPerSec:  r.DiskWriteBytesPerSec,
			NetworkRxBytesPerSec:  r.NetworkRxBytesPerSec,
			NetworkTxBytesPerSec:  r.NetworkTxBytesPerSec,
			CPUTemperatureCelsius: r.CPUTemperatureCelsius,
			UptimeSeconds:         r.UptimeSeconds,
		}
	}

	return results, nil
}

// GetContainerMetrics retrieves container metrics for a host within a time range.
func GetContainerMetrics(hostID string, start, end time.Time, interval string) ([]models.ContainerMetric, error) {
	var host models.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHostNotFound
		}
		return nil, err
	}

	if interval == "" {
		var metrics []models.ContainerMetric
		if err := database.DB.Where("host_id = ? AND timestamp >= ? AND timestamp <= ?",
			hostID, start, end).
			Order("timestamp ASC").
			Find(&metrics).Error; err != nil {
			return nil, err
		}
		return metrics, nil
	}

	// Use continuous aggregates for longer time ranges.
	tableName, err := getContainerAggregateTable(interval)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
		SELECT
			bucket AS timestamp,
			host_id,
			container_id,
			container_name,
			cpu_percent,
			CAST(memory_used_bytes AS BIGINT) AS memory_used_bytes,
			CAST(memory_limit_bytes AS BIGINT) AS memory_limit_bytes,
			CAST(network_rx_bytes_per_sec AS BIGINT) AS network_rx_bytes_per_sec,
			CAST(network_tx_bytes_per_sec AS BIGINT) AS network_tx_bytes_per_sec
		FROM %s
		WHERE host_id = $1 AND bucket >= $2 AND bucket <= $3
		ORDER BY bucket ASC, container_name ASC
	`, tableName)

	var metrics []models.ContainerMetric
	if err := database.DB.Raw(query, hostID, start, end).
		Scan(&metrics).Error; err != nil {
		return nil, err
	}

	return metrics, nil
}

// GetLatestMetric returns the most recent metric row for a host, or nil if none exist yet.
func GetLatestMetric(hostID string) (*MetricDataPoint, error) {
	var host models.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHostNotFound
		}
		return nil, err
	}

	var m models.Metric
	if err := database.DB.Where("host_id = ?", hostID).
		Order("timestamp DESC").
		First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &MetricDataPoint{
		Timestamp:             m.Timestamp,
		CPUUsagePercent:       m.CPUUsagePercent,
		CPUIowaitPercent:      m.CPUIowaitPercent,
		CPUStealPercent:       m.CPUStealPercent,
		MemoryTotalBytes:      m.MemoryTotalBytes,
		MemoryUsedBytes:       m.MemoryUsedBytes,
		MemoryAvailableBytes:  m.MemoryAvailableBytes,
		MemoryBuffersBytes:    m.MemoryBuffersBytes,
		MemoryCachedBytes:     m.MemoryCachedBytes,
		SwapTotalBytes:        m.SwapTotalBytes,
		SwapUsedBytes:         m.SwapUsedBytes,
		LoadAvg1Min:           m.LoadAvg1Min,
		LoadAvg5Min:           m.LoadAvg5Min,
		LoadAvg15Min:          m.LoadAvg15Min,
		DiskTotalBytes:        m.DiskTotalBytes,
		DiskUsedBytes:         m.DiskUsedBytes,
		DiskReadBytesPerSec:   m.DiskReadBytesPerSec,
		DiskWriteBytesPerSec:  m.DiskWriteBytesPerSec,
		NetworkRxBytesPerSec:  m.NetworkRxBytesPerSec,
		NetworkTxBytesPerSec:  m.NetworkTxBytesPerSec,
		CPUTemperatureCelsius: m.CPUTemperatureCelsius,
		UptimeSeconds:         m.UptimeSeconds,
		ProcessesCount:        m.ProcessesCount,
		SensorReadings:        m.SensorReadings,
	}, nil
}

func getContinuousAggregateTable(interval string) (string, error) {
	tables := map[string]string{
		"10m": "metrics_10min",
		"15m": "metrics_15min",
		"2h":  "metrics_2h",
		"8h":  "metrics_8h",
	}
	name, ok := tables[interval]
	if !ok {
		return "", fmt.Errorf("invalid interval: %s (valid: 10m, 15m, 2h, 8h)", interval)
	}
	return name, nil
}

func getContainerAggregateTable(interval string) (string, error) {
	tables := map[string]string{
		"10m": "container_metrics_10min",
		"15m": "container_metrics_15min",
		"2h":  "container_metrics_2h",
		"8h":  "container_metrics_8h",
	}
	name, ok := tables[interval]
	if !ok {
		return "", fmt.Errorf("invalid interval: %s (valid: 10m, 15m, 2h, 8h)", interval)
	}
	return name, nil
}

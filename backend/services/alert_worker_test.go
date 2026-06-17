package services

import (
	"strings"
	"testing"
	"time"
	"watchflare/backend/models"

	"github.com/stretchr/testify/assert"
)

func TestEvaluateMetric_HostDown(t *testing.T) {
	// Offline host → breaching
	breaching, value := evaluateMetric(models.MetricTypeHostDown, 0, models.StatusOffline, nil)
	assert.True(t, breaching)
	assert.Equal(t, float64(0), value)

	// Online host → not breaching
	breaching, value = evaluateMetric(models.MetricTypeHostDown, 0, models.StatusOnline, nil)
	assert.False(t, breaching)
	assert.Equal(t, float64(0), value)
}

func TestEvaluateMetric_CPUUsage(t *testing.T) {
	// Above threshold
	m := &models.Metric{CPUUsagePercent: 90.0}
	breaching, value := evaluateMetric(models.MetricTypeCPUUsage, 80.0, models.StatusOnline, m)
	assert.True(t, breaching)
	assert.Equal(t, 90.0, value)

	// Below threshold
	breaching, value = evaluateMetric(models.MetricTypeCPUUsage, 95.0, models.StatusOnline, m)
	assert.False(t, breaching)
	assert.Equal(t, 90.0, value)

	// Nil metric → not breaching
	breaching, value = evaluateMetric(models.MetricTypeCPUUsage, 80.0, models.StatusOnline, nil)
	assert.False(t, breaching)
	assert.Equal(t, float64(0), value)
}

func TestEvaluateMetric_MemoryUsage(t *testing.T) {
	// 8GB used out of 16GB = 50%
	m := &models.Metric{
		MemoryTotalBytes: 16 * 1024 * 1024 * 1024,
		MemoryUsedBytes:  8 * 1024 * 1024 * 1024,
	}

	// Threshold 40% → breaching (50% >= 40%)
	breaching, value := evaluateMetric(models.MetricTypeMemoryUsage, 40.0, models.StatusOnline, m)
	assert.True(t, breaching)
	assert.InDelta(t, 50.0, value, 0.01)

	// Threshold 60% → not breaching (50% < 60%)
	breaching, value = evaluateMetric(models.MetricTypeMemoryUsage, 60.0, models.StatusOnline, m)
	assert.False(t, breaching)
	assert.InDelta(t, 50.0, value, 0.01)

	// Nil metric → not breaching
	breaching, value = evaluateMetric(models.MetricTypeMemoryUsage, 40.0, models.StatusOnline, nil)
	assert.False(t, breaching)
	assert.Equal(t, float64(0), value)

	// Zero total bytes → not breaching (avoid division by zero)
	m2 := &models.Metric{MemoryTotalBytes: 0, MemoryUsedBytes: 1024}
	breaching, value = evaluateMetric(models.MetricTypeMemoryUsage, 40.0, models.StatusOnline, m2)
	assert.False(t, breaching)
	assert.Equal(t, float64(0), value)
}

func TestEvaluateMetric_DiskUsage(t *testing.T) {
	// 90GB used out of 100GB = 90%
	m := &models.Metric{
		DiskTotalBytes: 100 * 1024 * 1024 * 1024,
		DiskUsedBytes:  90 * 1024 * 1024 * 1024,
	}

	// Threshold 85% → breaching
	breaching, value := evaluateMetric(models.MetricTypeDiskUsage, 85.0, models.StatusOnline, m)
	assert.True(t, breaching)
	assert.InDelta(t, 90.0, value, 0.01)

	// Threshold 95% → not breaching
	breaching, value = evaluateMetric(models.MetricTypeDiskUsage, 95.0, models.StatusOnline, m)
	assert.False(t, breaching)
	assert.InDelta(t, 90.0, value, 0.01)

	// Nil metric → not breaching
	breaching, value = evaluateMetric(models.MetricTypeDiskUsage, 85.0, models.StatusOnline, nil)
	assert.False(t, breaching)
	assert.Equal(t, float64(0), value)
}

func TestEvaluateMetric_LoadAvg(t *testing.T) {
	m := &models.Metric{
		LoadAvg1Min:  2.5,
		LoadAvg5Min:  1.8,
		LoadAvg15Min: 1.2,
	}

	// load_avg (1min) — above threshold
	breaching, value := evaluateMetric(models.MetricTypeLoadAvg, 2.0, models.StatusOnline, m)
	assert.True(t, breaching)
	assert.Equal(t, 2.5, value)

	// load_avg (1min) — below threshold
	breaching, value = evaluateMetric(models.MetricTypeLoadAvg, 3.0, models.StatusOnline, m)
	assert.False(t, breaching)
	assert.Equal(t, 2.5, value)

	// load_avg_5 — above threshold
	breaching, value = evaluateMetric(models.MetricTypeLoadAvg5, 1.5, models.StatusOnline, m)
	assert.True(t, breaching)
	assert.Equal(t, 1.8, value)

	// load_avg_15 — below threshold
	breaching, value = evaluateMetric(models.MetricTypeLoadAvg15, 2.0, models.StatusOnline, m)
	assert.False(t, breaching)
	assert.Equal(t, 1.2, value)

	// Nil metric → not breaching
	breaching, value = evaluateMetric(models.MetricTypeLoadAvg, 2.0, models.StatusOnline, nil)
	assert.False(t, breaching)
	assert.Equal(t, float64(0), value)
}

func TestEvaluateMetric_Temperature(t *testing.T) {
	m := &models.Metric{CPUTemperatureCelsius: 75.0}

	// Above threshold
	breaching, value := evaluateMetric(models.MetricTypeTemperature, 70.0, models.StatusOnline, m)
	assert.True(t, breaching)
	assert.Equal(t, 75.0, value)

	// Below threshold
	breaching, value = evaluateMetric(models.MetricTypeTemperature, 80.0, models.StatusOnline, m)
	assert.False(t, breaching)
	assert.Equal(t, 75.0, value)

	// Zero value (sensor unavailable) → not breaching
	m2 := &models.Metric{CPUTemperatureCelsius: 0}
	breaching, value = evaluateMetric(models.MetricTypeTemperature, 70.0, models.StatusOnline, m2)
	assert.False(t, breaching)
	assert.Equal(t, float64(0), value)

	// Nil metric → not breaching
	breaching, value = evaluateMetric(models.MetricTypeTemperature, 70.0, models.StatusOnline, nil)
	assert.False(t, breaching)
	assert.Equal(t, float64(0), value)
}

func TestEvaluateMetric_UnknownType(t *testing.T) {
	breaching, value := evaluateMetric("unknown_metric", 50.0, models.StatusOnline, &models.Metric{CPUUsagePercent: 99})
	assert.False(t, breaching)
	assert.Equal(t, float64(0), value)
}

func TestBuildResolutionEmailContent_HostDown(t *testing.T) {
	startedAt := time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC)
	resolvedAt := time.Date(2026, 4, 8, 10, 5, 30, 0, time.UTC)

	subject, body := buildResolutionEmailContent("web01", models.MetricTypeHostDown, startedAt, resolvedAt)

	assert.Contains(t, subject, "Resolved")
	assert.Contains(t, subject, "web01")
	assert.Contains(t, subject, "back online")
	assert.Contains(t, body, "web01")
	assert.Contains(t, body, "5m30s")
}

func TestBuildResolutionEmailContent_CPUUsage(t *testing.T) {
	startedAt := time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC)
	resolvedAt := time.Date(2026, 4, 8, 10, 10, 0, 0, time.UTC)

	subject, body := buildResolutionEmailContent("web01", models.MetricTypeCPUUsage, startedAt, resolvedAt)

	assert.Contains(t, subject, "Resolved")
	assert.Contains(t, subject, "CPU usage")
	assert.Contains(t, body, "CPU usage")
	assert.True(t, strings.Contains(body, "10m0s"))
}

func TestBuildResolutionEmailContent_UnknownMetric(t *testing.T) {
	startedAt := time.Now().Add(-2 * time.Minute)
	resolvedAt := time.Now()

	subject, body := buildResolutionEmailContent("db01", "custom_metric", startedAt, resolvedAt)

	assert.Contains(t, subject, "custom_metric")
	assert.Contains(t, body, "db01")
}

func TestClearHost_RemovesPendingForHost(t *testing.T) {
	w := NewAlertWorker(30 * time.Second)
	w.pending["host-a:cpu_usage"] = time.Now()
	w.pending["host-a:memory_usage"] = time.Now()
	w.pending["host-b:cpu_usage"] = time.Now()

	w.ClearHost("host-a")

	assert.NotContains(t, w.pending, "host-a:cpu_usage")
	assert.NotContains(t, w.pending, "host-a:memory_usage")
	assert.Contains(t, w.pending, "host-b:cpu_usage")
}

func TestNewAlertWorker_SetsDefault(t *testing.T) {
	prev := DefaultAlertWorker
	t.Cleanup(func() { DefaultAlertWorker = prev })

	w := NewAlertWorker(30 * time.Second)
	assert.Same(t, w, DefaultAlertWorker)
}

package services

import (
	"testing"
	"watchflare/backend/database"
	"watchflare/backend/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func teardownAlertRules() {
	database.DB.Exec("DELETE FROM host_alert_rules")
	database.DB.Exec("DELETE FROM alert_rules")
}

// seedHost creates a parent host so host_alert_rules inserts satisfy the
// host_id foreign key constraint. Cleaned up by teardownTestDB.
func seedHost(t *testing.T, id string) {
	t.Helper()
	require.NoError(t, database.DB.Create(&models.Host{ID: id, DisplayName: id, Status: models.StatusOffline}).Error)
}

func seedGlobalRules(t *testing.T) {
	t.Helper()
	inputs := []AlertRuleInput{
		{MetricType: models.MetricTypeCPUUsage, Enabled: true, Threshold: 80.0, DurationMinutes: 5},
		{MetricType: models.MetricTypeMemoryUsage, Enabled: false, Threshold: 90.0, DurationMinutes: 10},
	}
	require.NoError(t, UpdateAlertRules(inputs))
}

func TestGetAlertRules(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownAlertRules()

	seedGlobalRules(t)

	rules, err := GetAlertRules()
	require.NoError(t, err)

	// Rules must be returned in AllMetricTypes order.
	// cpu_usage comes before memory_usage in AllMetricTypes.
	found := make(map[string]models.AlertRule)
	for _, r := range rules {
		found[r.MetricType] = r
	}

	cpuRule, ok := found[models.MetricTypeCPUUsage]
	require.True(t, ok)
	assert.True(t, cpuRule.Enabled)
	assert.Equal(t, 80.0, cpuRule.Threshold)
	assert.Equal(t, 5, cpuRule.DurationMinutes)

	memRule, ok := found[models.MetricTypeMemoryUsage]
	require.True(t, ok)
	assert.False(t, memRule.Enabled)
	assert.Equal(t, 90.0, memRule.Threshold)
	assert.Equal(t, 10, memRule.DurationMinutes)
}

func TestGetAlertRules_OrderedByAllMetricTypes(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownAlertRules()

	// Seed all metric types in reverse order to verify ordering is canonical.
	for i := len(models.AllMetricTypes) - 1; i >= 0; i-- {
		require.NoError(t, UpdateAlertRules([]AlertRuleInput{
			{MetricType: models.AllMetricTypes[i], Enabled: false, Threshold: 0, DurationMinutes: 1},
		}))
	}

	rules, err := GetAlertRules()
	require.NoError(t, err)
	require.Len(t, rules, len(models.AllMetricTypes))

	for i, r := range rules {
		assert.Equal(t, models.AllMetricTypes[i], r.MetricType)
	}
}

func TestUpdateAlertRules(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownAlertRules()

	// Create initial rule.
	require.NoError(t, UpdateAlertRules([]AlertRuleInput{
		{MetricType: models.MetricTypeCPUUsage, Enabled: false, Threshold: 50.0, DurationMinutes: 3},
	}))

	// Update it.
	require.NoError(t, UpdateAlertRules([]AlertRuleInput{
		{MetricType: models.MetricTypeCPUUsage, Enabled: true, Threshold: 85.0, DurationMinutes: 10},
	}))

	rules, err := GetAlertRules()
	require.NoError(t, err)

	var cpuRule *models.AlertRule
	for _, r := range rules {
		r := r
		if r.MetricType == models.MetricTypeCPUUsage {
			cpuRule = &r
			break
		}
	}
	require.NotNil(t, cpuRule)
	assert.True(t, cpuRule.Enabled)
	assert.Equal(t, 85.0, cpuRule.Threshold)
	assert.Equal(t, 10, cpuRule.DurationMinutes)
}

func TestGetHostAlertRules_GlobalFallback(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownAlertRules()

	seedGlobalRules(t)

	rules, err := GetHostAlertRules("host-no-overrides")
	require.NoError(t, err)

	found := make(map[string]EffectiveAlertRule)
	for _, r := range rules {
		found[r.MetricType] = r
	}

	cpu, ok := found[models.MetricTypeCPUUsage]
	require.True(t, ok)
	assert.Equal(t, 80.0, cpu.Threshold)
	assert.False(t, cpu.IsOverride)

	mem, ok := found[models.MetricTypeMemoryUsage]
	require.True(t, ok)
	assert.Equal(t, 90.0, mem.Threshold)
	assert.False(t, mem.IsOverride)
}

func TestGetHostAlertRules_HostOverrideMerged(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownAlertRules()

	seedGlobalRules(t)

	hostID := "host-with-override"
	seedHost(t, hostID)
	require.NoError(t, UpsertHostAlertRule(hostID, models.MetricTypeCPUUsage, AlertRuleInput{
		MetricType:      models.MetricTypeCPUUsage,
		Enabled:         true,
		Threshold:       95.0,
		DurationMinutes: 2,
	}))

	rules, err := GetHostAlertRules(hostID)
	require.NoError(t, err)

	found := make(map[string]EffectiveAlertRule)
	for _, r := range rules {
		found[r.MetricType] = r
	}

	// CPU must come from override.
	cpu, ok := found[models.MetricTypeCPUUsage]
	require.True(t, ok)
	assert.Equal(t, 95.0, cpu.Threshold)
	assert.Equal(t, 2, cpu.DurationMinutes)
	assert.True(t, cpu.IsOverride)

	// Memory must fall back to global.
	mem, ok := found[models.MetricTypeMemoryUsage]
	require.True(t, ok)
	assert.Equal(t, 90.0, mem.Threshold)
	assert.False(t, mem.IsOverride)
}

func TestUpsertHostAlertRule_CreateAndUpdate(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownAlertRules()

	seedGlobalRules(t)

	hostID := "host-upsert"
	seedHost(t, hostID)

	// Create override.
	require.NoError(t, UpsertHostAlertRule(hostID, models.MetricTypeCPUUsage, AlertRuleInput{
		MetricType:      models.MetricTypeCPUUsage,
		Enabled:         true,
		Threshold:       70.0,
		DurationMinutes: 3,
	}))

	var rule models.HostAlertRule
	require.NoError(t, database.DB.Where("host_id = ? AND metric_type = ?", hostID, models.MetricTypeCPUUsage).First(&rule).Error)
	assert.Equal(t, 70.0, rule.Threshold)
	assert.Equal(t, 3, rule.DurationMinutes)

	// Update override.
	require.NoError(t, UpsertHostAlertRule(hostID, models.MetricTypeCPUUsage, AlertRuleInput{
		MetricType:      models.MetricTypeCPUUsage,
		Enabled:         false,
		Threshold:       99.0,
		DurationMinutes: 15,
	}))

	require.NoError(t, database.DB.Where("host_id = ? AND metric_type = ?", hostID, models.MetricTypeCPUUsage).First(&rule).Error)
	assert.Equal(t, 99.0, rule.Threshold)
	assert.Equal(t, 15, rule.DurationMinutes)
	assert.False(t, rule.Enabled)
}

func TestDeleteHostAlertRule(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownAlertRules()

	seedGlobalRules(t)

	hostID := "host-delete"
	seedHost(t, hostID)

	// Create override.
	require.NoError(t, UpsertHostAlertRule(hostID, models.MetricTypeCPUUsage, AlertRuleInput{
		MetricType:      models.MetricTypeCPUUsage,
		Enabled:         true,
		Threshold:       70.0,
		DurationMinutes: 3,
	}))

	// Delete it.
	require.NoError(t, DeleteHostAlertRule(hostID, models.MetricTypeCPUUsage))

	var count int64
	database.DB.Model(&models.HostAlertRule{}).
		Where("host_id = ? AND metric_type = ?", hostID, models.MetricTypeCPUUsage).
		Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestDeleteHostAlertRule_NoOp(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer teardownAlertRules()

	// Deleting a non-existent override must not return an error.
	err := DeleteHostAlertRule("host-nonexistent", models.MetricTypeCPUUsage)
	assert.NoError(t, err)
}

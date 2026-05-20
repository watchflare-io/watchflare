package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"
	"watchflare/backend/database"
	"watchflare/backend/models"
	"watchflare/backend/services"

	"github.com/gin-gonic/gin"
)

// GetAlertRules returns the global alert rules.
func GetAlertRules(c *gin.Context) {
	rules, err := services.GetAlertRules()
	if err != nil {
		slog.Error("failed to get alert rules", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get alert rules"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

// UpdateAlertRulesRequest is the body for PUT /settings/alerts.
type UpdateAlertRulesRequest struct {
	Rules []UpdateAlertRuleItem `json:"rules"`
}

// UpdateAlertRuleItem is one entry in an UpdateAlertRulesRequest.
type UpdateAlertRuleItem struct {
	MetricType      string  `json:"metric_type"`
	Enabled         bool    `json:"enabled"`
	Threshold       float64 `json:"threshold"`
	DurationMinutes int     `json:"duration_minutes"`
}

// UpdateAlertRules replaces all global alert rules.
func UpdateAlertRules(c *gin.Context) {
	var req UpdateAlertRulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, item := range req.Rules {
		if !isValidMetricType(item.MetricType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid metric_type: " + item.MetricType})
			return
		}
		if item.DurationMinutes < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "duration_minutes must be at least 1"})
			return
		}
		if !isValidThreshold(item.MetricType, item.Threshold) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "threshold value out of valid range for metric type " + item.MetricType})
			return
		}
	}

	inputs := make([]services.AlertRuleInput, len(req.Rules))
	for i, r := range req.Rules {
		inputs[i] = services.AlertRuleInput{
			MetricType:      r.MetricType,
			Enabled:         r.Enabled,
			Threshold:       r.Threshold,
			DurationMinutes: r.DurationMinutes,
		}
	}

	if err := services.UpdateAlertRules(inputs); err != nil {
		slog.Error("failed to update alert rules", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update alert rules"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "alert rules updated"})
}

// GetHostAlertRules returns the effective alert rules for a specific host.
func GetHostAlertRules(c *gin.Context) {
	hostID := c.Param("id")
	rules, err := services.GetHostAlertRules(hostID)
	if err != nil {
		slog.Error("failed to get host alert rules", "host_id", hostID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get host alert rules"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

// UpsertHostAlertRuleRequest is the body for PUT /hosts/:id/alerts/:metric_type.
type UpsertHostAlertRuleRequest struct {
	Enabled         bool    `json:"enabled"`
	Threshold       float64 `json:"threshold"`
	DurationMinutes int     `json:"duration_minutes"`
}

// UpsertHostAlertRule creates or updates a per-host alert rule override.
func UpsertHostAlertRule(c *gin.Context) {
	hostID := c.Param("id")
	metricType := c.Param("metric_type")

	if !isValidMetricType(metricType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid metric_type: " + metricType})
		return
	}

	var req UpsertHostAlertRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.DurationMinutes < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "duration_minutes must be at least 1"})
		return
	}
	if !isValidThreshold(metricType, req.Threshold) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "threshold value out of valid range for metric type " + metricType})
		return
	}

	if err := services.UpsertHostAlertRule(hostID, metricType, services.AlertRuleInput{
		MetricType:      metricType,
		Enabled:         req.Enabled,
		Threshold:       req.Threshold,
		DurationMinutes: req.DurationMinutes,
	}); err != nil {
		slog.Error("failed to upsert host alert rule", "host_id", hostID, "metric_type", metricType, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save host alert rule"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "host alert rule saved"})
}

// DeleteHostAlertRule removes a per-host override, reverting to the global default.
func DeleteHostAlertRule(c *gin.Context) {
	hostID := c.Param("id")
	metricType := c.Param("metric_type")

	if !isValidMetricType(metricType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid metric_type: " + metricType})
		return
	}

	if err := services.DeleteHostAlertRule(hostID, metricType); err != nil {
		slog.Error("failed to delete host alert rule", "host_id", hostID, "metric_type", metricType, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete host alert rule"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "host alert rule deleted"})
}

// ActiveIncidentItem is the response shape for GET /alerts/active.
type ActiveIncidentItem struct {
	ID             string    `json:"id"`
	HostID         string    `json:"host_id"`
	HostName       string    `json:"host_name"`
	MetricType     string    `json:"metric_type"`
	StartedAt      time.Time `json:"started_at"`
	ThresholdValue float64   `json:"threshold_value"`
	CurrentValue   float64   `json:"current_value"`
}

// GetActiveIncidents returns all unresolved alert incidents with their host name.
func GetActiveIncidents(c *gin.Context) {
	var items []ActiveIncidentItem
	err := database.DB.Table("alert_incidents").
		Select("alert_incidents.id, alert_incidents.host_id, hosts.display_name AS host_name, alert_incidents.metric_type, alert_incidents.started_at, alert_incidents.threshold_value, alert_incidents.current_value").
		Joins("JOIN hosts ON hosts.id = alert_incidents.host_id").
		Where("alert_incidents.resolved_at IS NULL").
		Order("alert_incidents.started_at DESC").
		Scan(&items).Error
	if err != nil {
		slog.Error("failed to get active incidents", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get active incidents"})
		return
	}
	if items == nil {
		items = []ActiveIncidentItem{}
	}
	c.JSON(http.StatusOK, gin.H{"incidents": items})
}

// GlobalIncidentItem is the response shape for GET /settings/alerts/incidents.
type GlobalIncidentItem struct {
	ID             string     `json:"id"`
	HostID         string     `json:"host_id"`
	HostName       string     `json:"host_name"`
	MetricType     string     `json:"metric_type"`
	StartedAt      time.Time  `json:"started_at"`
	ResolvedAt     *time.Time `json:"resolved_at"`
	ThresholdValue float64    `json:"threshold_value"`
	CurrentValue   float64    `json:"current_value"`
}

// GetAllIncidents returns all alert incidents across all hosts (paginated).
// Query params: status=all|active|resolved (default: all), limit (default: 20, max: 100), offset (default: 0).
func GetAllIncidents(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
	statusFilter := c.DefaultQuery("status", "all")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	query := database.DB.Table("alert_incidents").
		Joins("JOIN hosts ON hosts.id = alert_incidents.host_id")
	switch statusFilter {
	case "active":
		query = query.Where("alert_incidents.resolved_at IS NULL")
	case "resolved":
		query = query.Where("alert_incidents.resolved_at IS NOT NULL")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		slog.Error("failed to count incidents", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get incidents"})
		return
	}

	var items []GlobalIncidentItem
	err := query.
		Select("alert_incidents.id, alert_incidents.host_id, hosts.display_name AS host_name, alert_incidents.metric_type, alert_incidents.started_at, alert_incidents.resolved_at, alert_incidents.threshold_value, alert_incidents.current_value").
		Order("alert_incidents.started_at DESC").
		Limit(limit).Offset(offset).
		Scan(&items).Error
	if err != nil {
		slog.Error("failed to get incidents", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get incidents"})
		return
	}
	if items == nil {
		items = []GlobalIncidentItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"incidents":   items,
		"total_count": total,
		"limit":       limit,
		"offset":      offset,
	})
}

// HostIncidentItem is the response shape for GET /hosts/:id/incidents.
type HostIncidentItem struct {
	ID             string     `json:"id"`
	MetricType     string     `json:"metric_type"`
	StartedAt      time.Time  `json:"started_at"`
	ResolvedAt     *time.Time `json:"resolved_at"`
	ThresholdValue float64    `json:"threshold_value"`
	CurrentValue   float64    `json:"current_value"`
}

// GetHostIncidents returns the incident history for a specific host (paginated).
// Query params: status=all|active|resolved (default: all), limit (default: 20, max: 100), offset (default: 0).
func GetHostIncidents(c *gin.Context) {
	hostID := c.Param("id")

	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
	statusFilter := c.DefaultQuery("status", "all")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	query := database.DB.Model(&models.AlertIncident{}).Where("host_id = ?", hostID)
	switch statusFilter {
	case "active":
		query = query.Where("resolved_at IS NULL")
	case "resolved":
		query = query.Where("resolved_at IS NOT NULL")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		slog.Error("failed to count host incidents", "host_id", hostID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get incidents"})
		return
	}

	var incidents []models.AlertIncident
	if err := query.Order("started_at DESC").Limit(limit).Offset(offset).Find(&incidents).Error; err != nil {
		slog.Error("failed to get host incidents", "host_id", hostID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get incidents"})
		return
	}

	items := make([]HostIncidentItem, len(incidents))
	for i, inc := range incidents {
		items[i] = HostIncidentItem{
			ID:             inc.ID,
			MetricType:     inc.MetricType,
			StartedAt:      inc.StartedAt,
			ResolvedAt:     inc.ResolvedAt,
			ThresholdValue: inc.ThresholdValue,
			CurrentValue:   inc.CurrentValue,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"incidents":   items,
		"total_count": total,
		"limit":       limit,
		"offset":      offset,
	})
}

func isValidMetricType(mt string) bool {
	for _, valid := range models.AllMetricTypes {
		if mt == valid {
			return true
		}
	}
	return false
}

// isValidThreshold checks that a threshold value is within a sensible range for the given metric type.
// host_down has no threshold (ignored). Percentages must be 0-100. Temperature 0-150. Load avg > 0.
func isValidThreshold(metricType string, threshold float64) bool {
	switch metricType {
	case models.MetricTypeHostDown:
		return true
	case models.MetricTypeCPUUsage, models.MetricTypeMemoryUsage, models.MetricTypeDiskUsage:
		return threshold >= 0 && threshold <= 100
	case models.MetricTypeTemperature:
		return threshold >= 0 && threshold <= 150
	case models.MetricTypeLoadAvg, models.MetricTypeLoadAvg5, models.MetricTypeLoadAvg15:
		return threshold >= 0
	}
	return true
}

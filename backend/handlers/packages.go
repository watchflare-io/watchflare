package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"watchflare/backend/cache"
	"watchflare/backend/database"
	"watchflare/backend/models"

	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
)

// paginationResult is the pagination envelope returned by package list endpoints.
type paginationResult struct {
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
	Pages int   `json:"pages"`
}

func buildPagination(total int64, limit, offset int) paginationResult {
	if limit <= 0 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}
	pages := int((total + int64(limit) - 1) / int64(limit))
	if pages < 1 {
		pages = 1
	}
	return paginationResult{
		Page:  offset/limit + 1,
		Limit: limit,
		Total: total,
		Pages: pages,
	}
}

// GetHostPackages returns current packages for a host with server-side filtering, sorting, and pagination.
// GET /api/v1/hosts/:id/packages
func GetHostPackages(c *gin.Context) {
	hostID := c.Param("id")

	// Pagination
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "25"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}

	// Filters
	q := c.Query("q")
	managerFilters := c.QueryArray("manager")
	statusFilters := c.QueryArray("status")
	sortBy := c.DefaultQuery("sort_by", "name")
	sortOrder := c.DefaultQuery("sort_order", "asc")
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "asc"
	}

	// Build base query
	query := database.DB.Where("host_id = ?", hostID)

	if q != "" {
		query = query.Where("name ILIKE ?", "%"+q+"%")
	}
	if len(managerFilters) > 0 {
		query = query.Where("package_manager IN ?", managerFilters)
	}
	if len(statusFilters) > 0 {
		var parts []string
		for _, s := range statusFilters {
			switch s {
			case "security":
				parts = append(parts, "has_security_update = true")
			case "outdated":
				parts = append(parts, "(COALESCE(available_version, '') != '' AND has_security_update = false)")
			case "up_to_date":
				parts = append(parts, "(COALESCE(available_version, '') = '' AND update_checked = true AND has_security_update = false)")
			case "not_checked":
				parts = append(parts, "(update_checked = false AND COALESCE(available_version, '') = '' AND has_security_update = false)")
			}
		}
		if len(parts) > 0 {
			query = query.Where("(" + strings.Join(parts, " OR ") + ")")
		}
	}

	// Sort order
	switch sortBy {
	case "version":
		query = query.Order("version " + sortOrder + ", name ASC")
	case "manager":
		query = query.Order("package_manager " + sortOrder + ", name ASC")
	case "status":
		query = query.Order(fmt.Sprintf(
			"CASE WHEN has_security_update THEN 0 WHEN COALESCE(available_version,'') != '' THEN 1 WHEN update_checked THEN 2 ELSE 3 END %s, name ASC",
			sortOrder,
		))
	case "latest_version":
		query = query.Order("COALESCE(available_version, '') " + sortOrder + ", name ASC")
	default:
		query = query.Order("name " + sortOrder)
	}

	// Count
	var totalCount int64
	if err := query.Model(&models.Package{}).Count(&totalCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count packages"})
		return
	}

	// Fetch page
	var packages []models.Package
	if err := query.Limit(limit).Offset(offset).Find(&packages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch packages"})
		return
	}
	if packages == nil {
		packages = []models.Package{}
	}

	c.JSON(http.StatusOK, gin.H{
		"packages":   packages,
		"pagination": buildPagination(totalCount, limit, offset),
	})
}

// GetHostPackageHistory returns package history for a host
// GET /api/v1/hosts/:id/packages/history
func GetHostPackageHistory(c *gin.Context) {
	hostID := c.Param("id")

	// Query parameters
	changeType := c.Query("change_type") // 'added', 'removed', 'updated', 'initial'
	excludeInitial := c.Query("exclude_initial") == "true"
	packageName := c.Query("package")
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")
	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	// Validate change_type if provided
	if changeType != "" &&
		changeType != models.ChangeTypeAdded &&
		changeType != models.ChangeTypeRemoved &&
		changeType != models.ChangeTypeUpdated &&
		changeType != models.ChangeTypeInitial {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid change_type, valid values: added, removed, updated, initial"})
		return
	}

	// Build query
	query := database.DB.Where("host_id = ?", hostID)

	if changeType != "" {
		query = query.Where("change_type = ?", changeType)
	} else if excludeInitial {
		query = query.Where("change_type != ?", models.ChangeTypeInitial)
	}

	if packageName != "" {
		query = query.Where("name ILIKE ?", "%"+packageName+"%")
	}

	if startTimeStr != "" {
		if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			query = query.Where("timestamp >= ?", startTime)
		}
	}

	if endTimeStr != "" {
		if endTime, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			query = query.Where("timestamp <= ?", endTime)
		}
	}

	// Get total count
	var totalCount int64
	if err := query.Model(&models.PackageHistory{}).Count(&totalCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count history records"})
		return
	}

	// Get history
	var history []models.PackageHistory
	if err := query.Order("timestamp DESC").Limit(limit).Offset(offset).Find(&history).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"history":    history,
		"pagination": buildPagination(totalCount, limit, offset),
	})
}

// GetHostPackageCollections returns package collection metadata
// GET /api/v1/hosts/:id/packages/collections
func GetHostPackageCollections(c *gin.Context) {
	hostID := c.Param("id")

	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	offset, _ := strconv.Atoi(offsetStr)
	if offset < 0 {
		offset = 0
	}

	// Get total count
	var totalCount int64
	if err := database.DB.Model(&models.PackageCollection{}).
		Where("host_id = ?", hostID).
		Count(&totalCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count collections"})
		return
	}

	// Get collections
	var collections []models.PackageCollection
	if err := database.DB.Where("host_id = ?", hostID).
		Order("timestamp DESC").
		Limit(limit).
		Offset(offset).
		Find(&collections).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch collections"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"collections": collections,
		"pagination":  buildPagination(totalCount, limit, offset),
	})
}

// TriggerPackageCollect enqueues a "collect_packages" command for the agent.
// The command is delivered on the agent's next heartbeat (within ~5s).
// POST /api/v1/hosts/:id/packages/collect
func TriggerPackageCollect(c *gin.Context) {
	hostID := c.Param("id")

	var host models.Host
	if err := database.DB.Select("id, agent_id, status").Where("id = ?", hostID).First(&host).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "host not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch host"})
		}
		return
	}

	heartbeatCache := cache.GetCache()
	cacheEntry, inCache := heartbeatCache.Get(host.AgentID)
	isOnline := inCache && cacheEntry.Status == models.StatusOnline
	if !isOnline {
		c.JSON(http.StatusConflict, gin.H{"error": "host is not online"})
		return
	}

	cmdID := heartbeatCache.EnqueueCommand(host.AgentID, models.CommandCollectPackages)
	slog.Info("package collect command enqueued", "host_id", hostID, "command_id", cmdID)

	c.JSON(http.StatusAccepted, gin.H{"message": "collection requested", "command_id": cmdID})
}

// globalPackage is the response shape for ListAllPackages — one row per (name, package_manager).
type globalPackage struct {
	Name              string `json:"name"`
	PackageManager    string `json:"package_manager"`
	HostCount         int64  `json:"host_count"`
	AvailableVersion  string `json:"available_version"`
	CurrentVersion    string `json:"current_version"`
	HasSecurityUpdate bool   `json:"has_security_update"`
	UpdateChecked     bool   `json:"update_checked"`
}

// ListAllPackages returns a deduplicated view of all packages across all hosts,
// grouped by (name, package_manager) with aggregated status.
// GET /api/v1/packages
func ListAllPackages(c *gin.Context) {
	q := c.Query("q")
	statusFilters := c.QueryArray("status")
	managerFilters := c.QueryArray("manager")

	limitStr := c.DefaultQuery("limit", "25")
	offsetStr := c.DefaultQuery("offset", "0")
	sortBy := c.DefaultQuery("sort_by", "name")
	sortOrder := c.DefaultQuery("sort_order", "asc")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	if limit <= 0 || limit > 200 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "asc"
	}

	// ORDER BY — references aliases from the SELECT, safe allowlist
	var orderClause string
	switch sortBy {
	case "host_count":
		orderClause = "host_count " + sortOrder
	case "manager":
		orderClause = "package_manager " + sortOrder + ", name ASC"
	case "available_version":
		orderClause = "available_version " + sortOrder + ", name ASC"
	case "status":
		orderClause = fmt.Sprintf(
			"CASE WHEN BOOL_OR(has_security_update) THEN 0 WHEN MAX(available_version) != '' THEN 1 WHEN BOOL_AND(update_checked) THEN 2 ELSE 3 END %s, name ASC",
			sortOrder,
		)
	default:
		orderClause = "name " + sortOrder
	}

	// WHERE conditions — user values always via ? placeholders
	whereClauses := []string{"1=1"}
	var whereArgs []interface{}
	if q != "" {
		whereClauses = append(whereClauses, "name ILIKE ?")
		whereArgs = append(whereArgs, "%"+q+"%")
	}
	if len(managerFilters) > 0 {
		whereClauses = append(whereClauses, "package_manager IN ?")
		whereArgs = append(whereArgs, managerFilters)
	}
	whereSQL := strings.Join(whereClauses, " AND ")

	// HAVING conditions for status filters — OR semantics, multiple statuses allowed
	var havingParts []string
	for _, s := range statusFilters {
		switch s {
		case "security":
			havingParts = append(havingParts, "BOOL_OR(has_security_update) = true")
		case "outdated":
			havingParts = append(havingParts, "(MAX(available_version) != '' AND NOT BOOL_OR(has_security_update))")
		case "up_to_date":
			havingParts = append(havingParts, "(MAX(available_version) = '' AND BOOL_AND(update_checked) = true)")
		case "not_checked":
			havingParts = append(havingParts, "(MAX(available_version) = '' AND BOOL_AND(update_checked) = false)")
		}
	}
	var havingSQL string
	if len(havingParts) > 0 {
		havingSQL = "HAVING " + strings.Join(havingParts, " OR ")
	}

	// Global stats — always unfiltered, used for the stats cards at the top of the page
	type statsRow struct {
		TotalPackages int64
		OutdatedCount int64
		SecurityCount int64
	}
	var stats statsRow
	statsSQL := `
		SELECT
			COUNT(*) AS total_packages,
			COUNT(*) FILTER (WHERE available_version != '') AS outdated_count,
			COUNT(*) FILTER (WHERE has_security_update) AS security_count
		FROM (
			SELECT
				MAX(available_version) AS available_version,
				BOOL_OR(has_security_update) AS has_security_update
			FROM packages
			GROUP BY name, package_manager
		) AS sub`
	if err := database.DB.Raw(statsSQL).Scan(&stats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch package stats"})
		return
	}

	// Hosts with at least one outdated package — unfiltered
	var outdatedHostsCount int64
	if err := database.DB.Model(&models.Package{}).
		Distinct("host_id").
		Where("available_version != '' OR has_security_update = true").
		Count(&outdatedHostsCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count outdated hosts"})
		return
	}

	// Hosts with at least one security update — unfiltered
	var securityHostsCount int64
	if err := database.DB.Model(&models.Package{}).
		Distinct("host_id").
		Where("has_security_update = true").
		Count(&securityHostsCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count security hosts"})
		return
	}

	// Available managers — unfiltered, used to populate the manager filter dropdown
	var availableManagers []string
	if err := database.DB.Model(&models.Package{}).
		Distinct("package_manager").
		Order("package_manager").
		Pluck("package_manager", &availableManagers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch managers"})
		return
	}
	if availableManagers == nil {
		availableManagers = []string{}
	}

	// Filtered count for pagination
	countSQL := fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT name
			FROM packages
			WHERE %s
			GROUP BY name, package_manager
			%s
		) AS sub`, whereSQL, havingSQL)
	var totalCount int64
	if err := database.DB.Raw(countSQL, whereArgs...).Scan(&totalCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count packages"})
		return
	}

	// Paginated results
	mainSQL := fmt.Sprintf(`
		SELECT
			name,
			package_manager,
			COUNT(DISTINCT host_id) AS host_count,
			MAX(available_version) AS available_version,
			MAX(CASE WHEN update_checked AND available_version = '' THEN version END) AS current_version,
			BOOL_OR(has_security_update) AS has_security_update,
			BOOL_AND(update_checked) AS update_checked
		FROM packages
		WHERE %s
		GROUP BY name, package_manager
		%s
		ORDER BY %s
		LIMIT ? OFFSET ?`, whereSQL, havingSQL, orderClause)

	mainArgs := append(whereArgs, limit, offset)

	packages := []globalPackage{}
	if err := database.DB.Raw(mainSQL, mainArgs...).Scan(&packages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch packages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"packages":             packages,
		"pagination":           buildPagination(totalCount, limit, offset),
		"total_packages":       stats.TotalPackages,
		"outdated_count":       stats.OutdatedCount,
		"security_count":       stats.SecurityCount,
		"outdated_hosts_count": outdatedHostsCount,
		"security_hosts_count": securityHostsCount,
		"available_managers":   availableManagers,
	})
}

// GetPackageStats returns aggregated package statistics
// GET /api/v1/hosts/:id/packages/stats
func GetPackageStats(c *gin.Context) {
	hostID := c.Param("id")

	// Package count by package manager
	var managerStats []struct {
		PackageManager string `json:"package_manager"`
		Count          int64  `json:"count"`
	}

	if err := database.DB.Model(&models.Package{}).
		Select("package_manager, COUNT(*) as count").
		Where("host_id = ?", hostID).
		Group("package_manager").
		Scan(&managerStats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch stats"})
		return
	}

	// Total packages
	var totalPackages int64
	if err := database.DB.Model(&models.Package{}).Where("host_id = ?", hostID).Count(&totalPackages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count packages"})
		return
	}

	// Recent changes (last 30 days)
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	var recentChanges int64
	if err := database.DB.Model(&models.PackageHistory{}).
		Where("host_id = ? AND timestamp >= ? AND change_type != ?", hostID, thirtyDaysAgo, models.ChangeTypeInitial).
		Count(&recentChanges).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count recent changes"})
		return
	}

	// Outdated packages count
	var outdatedCount int64
	if err := database.DB.Model(&models.Package{}).
		Where("host_id = ? AND available_version IS NOT NULL AND available_version != ''", hostID).
		Count(&outdatedCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count outdated packages"})
		return
	}

	// Security updates count
	var securityUpdatesCount int64
	if err := database.DB.Model(&models.Package{}).
		Where("host_id = ? AND has_security_update = true", hostID).
		Count(&securityUpdatesCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count security updates"})
		return
	}

	// Last collection (zero value is fine if none exists yet)
	var lastCollection models.PackageCollection
	if err := database.DB.Where("host_id = ?", hostID).Order("timestamp DESC").First(&lastCollection).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Warn("failed to fetch last collection", "host_id", hostID, "error", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"total_packages":        totalPackages,
		"by_package_manager":    managerStats,
		"recent_changes":        recentChanges,
		"outdated_count":        outdatedCount,
		"security_updates_count": securityUpdatesCount,
		"last_collection":       lastCollection,
	})
}

package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
	"watchflare/backend/cache"
	"watchflare/backend/database"
	"watchflare/backend/models"
	"watchflare/backend/sse"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const registrationTokenTTL = 24 * time.Hour

// ErrHostNotFound is returned when a host lookup finds no matching record.
var ErrHostNotFound = errors.New("host not found")

// CreateAgent creates a new host with status "pending" and returns the host,
// plaintext registration token, and plaintext agent key.
func CreateAgent(name, configuredIP string, allowAnyIP bool) (*models.Host, string, string, error) {
	agentID := uuid.New().String()

	token, hashedToken, err := generateRegistrationToken()
	if err != nil {
		return nil, "", "", err
	}

	// 32-byte key for HMAC-SHA256
	agentKeyBytes := make([]byte, 32)
	if _, err := rand.Read(agentKeyBytes); err != nil {
		return nil, "", "", err
	}
	agentKey := hex.EncodeToString(agentKeyBytes)

	expiresAt := time.Now().Add(registrationTokenTTL)

	var configuredIPPtr *string
	if configuredIP != "" {
		configuredIPPtr = &configuredIP
	}

	host := &models.Host{
		ID:                     uuid.New().String(),
		AgentID:                agentID,
		AgentKey:               agentKey,
		DisplayName:            name,
		ConfiguredIP:           configuredIPPtr,
		AllowAnyIPRegistration: allowAnyIP,
		RegistrationToken:      &hashedToken,
		ExpiresAt:              &expiresAt,
		Status:                 models.StatusPending,
	}

	if err := database.DB.Create(host).Error; err != nil {
		return nil, "", "", err
	}

	return host, token, agentKey, nil
}

// HostListParams holds parameters for listing hosts with sort/filter.
type HostListParams struct {
	Page        int
	PerPage     int
	Sort        string
	Order       string
	Status      string
	Search      string
	Environment string
}

// allowedSortColumns is a whitelist preventing SQL injection in ORDER BY.
var allowedSortColumns = map[string]string{
	"name":       "display_name",
	"status":     "status",
	"ip":         "ip_address_v4",
	"last_seen":  "last_seen",
	"created_at": "created_at",
}

// ListHosts returns hosts with sort, filter and pagination.
// Status filtering happens after cache merge because the cache holds real-time status.
func ListHosts(params HostListParams) ([]models.Host, int64, error) {
	query := database.DB.Model(&models.Host{})

	if params.Search != "" {
		search := "%" + params.Search + "%"
		query = query.Where("display_name ILIKE ? OR hostname ILIKE ?", search, search)
	}

	if params.Environment != "" {
		query = query.Where("environment_type = ?", params.Environment)
	}

	sortColumn := "created_at"
	if col, ok := allowedSortColumns[params.Sort]; ok {
		sortColumn = col
	}
	sortOrder := "DESC"
	if params.Order == "asc" {
		sortOrder = "ASC"
	}

	var allHosts []models.Host
	if err := query.Order(sortColumn + " " + sortOrder).Find(&allHosts).Error; err != nil {
		return nil, 0, err
	}

	mergeCache(allHosts)

	if params.Status != "" {
		var filtered []models.Host
		for _, h := range allHosts {
			if h.Status == params.Status {
				filtered = append(filtered, h)
			}
		}
		allHosts = filtered
	}

	total := int64(len(allHosts))

	// Pagination applied in memory after status filter.
	if params.PerPage > 0 {
		page := params.Page
		if page < 1 {
			page = 1
		}
		start := (page - 1) * params.PerPage
		if start >= int(total) {
			return []models.Host{}, total, nil
		}
		end := start + params.PerPage
		if end > int(total) {
			end = int(total)
		}
		allHosts = allHosts[start:end]
	}

	return allHosts, total, nil
}

// ListAllHosts returns all hosts without pagination (used for dashboard/SSE).
func ListAllHosts() ([]models.Host, error) {
	var hosts []models.Host
	if err := database.DB.Find(&hosts).Error; err != nil {
		return nil, err
	}
	mergeCache(hosts)
	return hosts, nil
}

// GetHost returns a single host by ID with real-time status from cache.
func GetHost(hostID string) (*models.Host, error) {
	var host models.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHostNotFound
		}
		return nil, err
	}

	if cachedData, ok := cache.GetCache().Get(host.AgentID); ok {
		host.Status = cachedData.Status
		host.LastSeen = &cachedData.LastSeen
		if cachedData.IPv4Address != "" {
			host.IPAddressV4 = &cachedData.IPv4Address
		}
		if cachedData.IPv6Address != "" {
			host.IPAddressV6 = &cachedData.IPv6Address
		}
	}

	return &host, nil
}

// ValidateIP confirms a selected IP for a host and clears the configured_ip.
func ValidateIP(hostID string, selectedIP string) error {
	var host models.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrHostNotFound
		}
		return err
	}

	host.IPAddressV4 = &selectedIP
	host.ConfiguredIP = nil

	return database.DB.Save(&host).Error
}

// RenameHost changes the display name of a host.
func RenameHost(hostID string, newName string) error {
	if len(newName) < 2 || len(newName) > 64 {
		return errors.New("name must be between 2 and 64 characters")
	}

	var host models.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrHostNotFound
		}
		return err
	}

	host.DisplayName = newName
	return database.DB.Save(&host).Error
}

// UpdateConfiguredIP changes the configured IP for a host and resets the ignore flag.
func UpdateConfiguredIP(hostID string, newIP string) error {
	var host models.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrHostNotFound
		}
		return err
	}

	if host.ConfiguredIP != nil && *host.ConfiguredIP != "" {
		host.PreviousConfiguredIP = host.ConfiguredIP
	}
	host.ConfiguredIP = &newIP
	host.IgnoreIPMismatch = false

	return database.DB.Save(&host).Error
}

// IgnoreIPMismatch marks the IP mismatch warning as dismissed by the user.
func IgnoreIPMismatch(hostID string) error {
	var host models.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrHostNotFound
		}
		return err
	}

	host.IgnoreIPMismatch = true
	return database.DB.Save(&host).Error
}

// DismissReactivation clears the reactivation badge for a host.
func DismissReactivation(hostID string) error {
	var host models.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrHostNotFound
		}
		return err
	}

	host.ReactivatedAt = nil
	return database.DB.Save(&host).Error
}

// RegenerateToken issues a new registration token and sets the host back to "pending".
func RegenerateToken(hostID string) (string, error) {
	var host models.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrHostNotFound
		}
		return "", err
	}

	token, hashedToken, err := generateRegistrationToken()
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().Add(registrationTokenTTL)
	host.RegistrationToken = &hashedToken
	host.ExpiresAt = &expiresAt
	host.Status = models.StatusPending

	if err := database.DB.Save(&host).Error; err != nil {
		return "", err
	}

	return token, nil
}

// PauseHost sets a host's status to "paused", suspends any open alert incidents,
// and removes it from the heartbeat cache and the alert worker's pending state.
func PauseHost(hostID string) error {
	var host models.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrHostNotFound
		}
		return err
	}

	if host.Status == models.StatusPending {
		return errors.New("cannot pause a pending host")
	}
	if host.Status == models.StatusPaused {
		return errors.New("host is already paused")
	}

	host.Status = models.StatusPaused
	if err := database.DB.Save(&host).Error; err != nil {
		return err
	}

	now := time.Now()
	result := database.DB.Model(&models.AlertIncident{}).
		Where("host_id = ? AND resolved_at IS NULL AND paused_at IS NULL", host.ID).
		Update("paused_at", now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		sse.GetBroker().BroadcastIncidentsChanged()
	}

	// Remove from heartbeat cache so the stale checker ignores it.
	cache.GetCache().Remove(host.AgentID)

	// Drop any pending firstSeen entries so a stale value cannot survive a pause/resume.
	if DefaultAlertWorker != nil {
		DefaultAlertWorker.ClearHost(host.ID)
	}

	return nil
}

// ResumeHost sets a paused host to "pending" and clears the paused state on any
// of its open incidents. We set pending (not online or offline) because we have
// no signal that the agent is alive yet: the heartbeat cache was wiped on pause.
// last_seen is reset to now so the stale-pending promotion timer starts here.
// The next incoming heartbeat will flip the host to online via the gRPC handler;
// if no heartbeat arrives within the stale-checker timeout, the host is promoted
// to offline and the alert worker reopens the host_down incident.
func ResumeHost(hostID string) error {
	var host models.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrHostNotFound
		}
		return err
	}

	if host.Status != models.StatusPaused {
		return errors.New("host is not paused")
	}

	now := time.Now()
	host.Status = models.StatusPending
	host.LastSeen = &now
	if err := database.DB.Save(&host).Error; err != nil {
		return err
	}

	result := database.DB.Model(&models.AlertIncident{}).
		Where("host_id = ? AND paused_at IS NOT NULL AND resolved_at IS NULL", host.ID).
		Update("paused_at", nil)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		sse.GetBroker().BroadcastIncidentsChanged()
	}
	return nil
}

// DeleteHost permanently removes a host and its associated data.
func DeleteHost(hostID string) error {
	var host models.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrHostNotFound
		}
		return err
	}

	return database.DB.Delete(&host).Error
}

// generateRegistrationToken generates a new wf_reg_* token and returns the
// plaintext token and its SHA-256 hash for storage.
func generateRegistrationToken() (token, hashedToken string, err error) {
	tokenBytes := make([]byte, 16)
	if _, err = rand.Read(tokenBytes); err != nil {
		return "", "", err
	}
	token = fmt.Sprintf("wf_reg_%s", hex.EncodeToString(tokenBytes))
	hashedToken = hashToken(token)
	return token, hashedToken, nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// mergeCache overlays real-time heartbeat data onto a slice of hosts.
func mergeCache(hosts []models.Host) {
	heartbeatCache := cache.GetCache()
	for i := range hosts {
		if cachedData, ok := heartbeatCache.Get(hosts[i].AgentID); ok {
			hosts[i].Status = cachedData.Status
			hosts[i].LastSeen = &cachedData.LastSeen
			if cachedData.IPv4Address != "" {
				hosts[i].IPAddressV4 = &cachedData.IPv4Address
			}
			if cachedData.IPv6Address != "" {
				hosts[i].IPAddressV6 = &cachedData.IPv6Address
			}
		}
	}
}

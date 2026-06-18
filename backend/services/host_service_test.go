package services

import (
	"os"
	"strings"
	"testing"
	"time"
	"watchflare/backend/cache"
	"watchflare/backend/config"
	"watchflare/backend/database"
	"watchflare/backend/models"

	"github.com/stretchr/testify/assert"
)

func testDSN() string {
	get := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}
	return "host=" + get("POSTGRES_HOST", "localhost") +
		" port=" + get("POSTGRES_PORT", "5432") +
		" user=" + get("POSTGRES_USER", "watchflare") +
		" password=" + get("POSTGRES_PASSWORD", "watchflare_dev") +
		" dbname=" + get("POSTGRES_TEST_DB", "watchflare_test") +
		" sslmode=" + get("POSTGRES_SSLMODE", "disable")
}

func setupTestDB(t *testing.T) {
	t.Helper()
	config.AppConfig = &config.Config{
		JWTSecret: "test-secret-key-must-be-32-chars!!",
	}
	if err := database.Connect(testDSN()); err != nil {
		t.Skipf("skipping test: database unavailable: %v", err)
	}
}

func teardownTestDB() {
	database.DB.Exec("DELETE FROM hosts")
}

func TestCreateAgent(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	host, token, agentKey, err := CreateAgent("host01", "192.168.1.100", false)

	assert.NoError(t, err)
	assert.NotNil(t, host)
	assert.NotEmpty(t, token)
	assert.NotEmpty(t, agentKey)

	// Verify host fields
	assert.Equal(t, "host01", host.DisplayName)
	assert.Equal(t, "192.168.1.100", *host.ConfiguredIP)
	assert.False(t, host.AllowAnyIPRegistration)
	assert.Equal(t, models.StatusPending, host.Status)

	// Verify agent ID is UUID
	assert.Len(t, host.AgentID, 36) // UUID length

	// Verify token format (wf_reg_{32_chars})
	assert.True(t, strings.HasPrefix(token, "wf_reg_"))
	assert.Len(t, token, 39) // "wf_reg_" + 32 chars

	// Verify agent key is AES-256 (64 hex chars)
	assert.Len(t, agentKey, 64)

	// Verify registration token is hashed in DB
	assert.NotNil(t, host.RegistrationToken)
	assert.NotEqual(t, token, *host.RegistrationToken) // Should be hashed

	// Verify expiration is ~24 hours from now
	assert.NotNil(t, host.ExpiresAt)
	expectedExpiry := time.Now().Add(time.Hour * 24)
	assert.WithinDuration(t, expectedExpiry, *host.ExpiresAt, time.Minute)

	// Verify host is saved in DB
	var dbHost models.Host
	database.DB.Where("id = ?", host.ID).First(&dbHost)
	assert.Equal(t, host.ID, dbHost.ID)
	assert.Equal(t, models.StatusPending, dbHost.Status)
}

func TestListHosts(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	// Create test hosts
	CreateAgent("host01", "192.168.1.100", false)
	CreateAgent("host02", "192.168.1.101", true)
	CreateAgent("host03", "192.168.1.102", false)

	hosts, _, err := ListHosts(HostListParams{})

	assert.NoError(t, err)
	assert.Len(t, hosts, 3)

	// Verify hosts are returned
	names := []string{hosts[0].DisplayName, hosts[1].DisplayName, hosts[2].DisplayName}
	assert.Contains(t, names, "host01")
	assert.Contains(t, names, "host02")
	assert.Contains(t, names, "host03")
}

func TestListHosts_Empty(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	hosts, _, err := ListHosts(HostListParams{})

	assert.NoError(t, err)
	assert.Empty(t, hosts)
}

func TestGetHost(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	// Create test host
	createdHost, _, _, _ := CreateAgent("host01", "192.168.1.100", false)

	// Get host
	host, err := GetHost(createdHost.ID)

	assert.NoError(t, err)
	assert.NotNil(t, host)
	assert.Equal(t, createdHost.ID, host.ID)
	assert.Equal(t, "host01", host.DisplayName)
}

func TestGetHost_NotFound(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	host, err := GetHost("00000000-0000-0000-0000-000000000000")

	assert.Error(t, err)
	assert.Nil(t, host)
	assert.Contains(t, err.Error(), "not found")
}

func TestValidateIP(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	// Create test host
	host, _, _, _ := CreateAgent("host01", "192.168.1.100", false)

	// Validate IP
	err := ValidateIP(host.ID, "192.168.1.150")

	assert.NoError(t, err)

	// Verify IP was updated and configured_ip cleared
	updatedHost, _ := GetHost(host.ID)
	assert.Equal(t, "192.168.1.150", *updatedHost.IPAddressV4)
	assert.Nil(t, updatedHost.ConfiguredIP)
}

func TestUpdateConfiguredIP(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	// Create test host
	host, _, _, _ := CreateAgent("host01", "192.168.1.100", false)

	// Update configured IP
	err := UpdateConfiguredIP(host.ID, "192.168.1.200")

	assert.NoError(t, err)

	// Verify IP was updated
	updatedHost, _ := GetHost(host.ID)
	assert.Equal(t, "192.168.1.200", *updatedHost.ConfiguredIP)
}

func TestRegenerateToken(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	// Create test host
	host, originalToken, _, _ := CreateAgent("host01", "192.168.1.100", false)

	// Regenerate token
	newToken, err := RegenerateToken(host.ID)

	assert.NoError(t, err)
	assert.NotEmpty(t, newToken)
	assert.NotEqual(t, originalToken, newToken)

	// Verify token format
	assert.True(t, strings.HasPrefix(newToken, "wf_reg_"))

	// Verify expiration was updated
	updatedHost, _ := GetHost(host.ID)
	expectedExpiry := time.Now().Add(time.Hour * 24)
	assert.WithinDuration(t, expectedExpiry, *updatedHost.ExpiresAt, time.Minute)
	assert.Equal(t, models.StatusPending, updatedHost.Status)
}

func TestRegenerateToken_OnlineHost(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	host, _, _, _ := CreateAgent("host01", "192.168.1.100", false)
	database.DB.Model(&host).Update("status", models.StatusOnline)

	// Regenerating token on an online host is allowed — agent will re-register.
	newToken, err := RegenerateToken(host.ID)

	assert.NoError(t, err)
	assert.NotEmpty(t, newToken)

	updatedHost, _ := GetHost(host.ID)
	assert.Equal(t, models.StatusPending, updatedHost.Status)
}

func TestDeleteHost(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	// Create test host
	host, _, _, _ := CreateAgent("host01", "192.168.1.100", false)

	// Delete host
	err := DeleteHost(host.ID)

	assert.NoError(t, err)

	// Verify host was deleted
	_, err = GetHost(host.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDeleteHost_OnlineHost(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	// Create host and set to online
	host, _, _, _ := CreateAgent("host01", "192.168.1.100", false)
	database.DB.Model(&host).Update("status", models.StatusOnline)

	// Delete host (should succeed)
	err := DeleteHost(host.ID)

	assert.NoError(t, err)

	// Verify host was deleted
	var deletedHost models.Host
	err = database.DB.Where("id = ?", host.ID).First(&deletedHost).Error
	assert.Error(t, err) // Should not find the host
}

func TestDeleteHost_NotFound(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	err := DeleteHost("00000000-0000-0000-0000-000000000000")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestListAllHosts(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	CreateAgent("host01", "192.168.1.1", false)
	CreateAgent("host02", "192.168.1.2", false)

	hosts, err := ListAllHosts()

	assert.NoError(t, err)
	assert.Len(t, hosts, 2)
}

func TestListHosts_Pagination(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	CreateAgent("host01", "192.168.1.1", false)
	CreateAgent("host02", "192.168.1.2", false)
	CreateAgent("host03", "192.168.1.3", false)

	hosts, total, err := ListHosts(HostListParams{Page: 1, PerPage: 2})

	assert.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, hosts, 2)
}

func TestListHosts_PaginationBeyondEnd(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	CreateAgent("host01", "192.168.1.1", false)

	hosts, total, err := ListHosts(HostListParams{Page: 99, PerPage: 10})

	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Empty(t, hosts)
}

func TestListHosts_SearchFilter(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	CreateAgent("web-host", "192.168.1.1", false)
	CreateAgent("db-host", "192.168.1.2", false)

	hosts, total, err := ListHosts(HostListParams{Search: "web"})

	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "web-host", hosts[0].DisplayName)
}

func TestRenameHost(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	host, _, _, _ := CreateAgent("old-name", "192.168.1.1", false)

	err := RenameHost(host.ID, "new-name")

	assert.NoError(t, err)
	updated, _ := GetHost(host.ID)
	assert.Equal(t, "new-name", updated.DisplayName)
}

func TestRenameHost_TooShort(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	host, _, _, _ := CreateAgent("old-name", "192.168.1.1", false)

	err := RenameHost(host.ID, "x")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "between 2 and 64")
}

func TestRenameHost_TooLong(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	host, _, _, _ := CreateAgent("old-name", "192.168.1.1", false)

	err := RenameHost(host.ID, strings.Repeat("a", 65))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "between 2 and 64")
}

func TestIgnoreIPMismatch(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	host, _, _, _ := CreateAgent("host01", "192.168.1.1", false)

	err := IgnoreIPMismatch(host.ID)

	assert.NoError(t, err)
	updated, _ := GetHost(host.ID)
	assert.True(t, updated.IgnoreIPMismatch)
}

func TestDismissReactivation(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	host, _, _, _ := CreateAgent("host01", "192.168.1.1", false)
	now := time.Now()
	database.DB.Model(&host).Update("reactivated_at", now)

	err := DismissReactivation(host.ID)

	assert.NoError(t, err)
	updated, _ := GetHost(host.ID)
	assert.Nil(t, updated.ReactivatedAt)
}

func TestPauseHost(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	host, _, _, _ := CreateAgent("host01", "192.168.1.1", false)
	database.DB.Model(&host).Update("status", models.StatusOnline)

	err := PauseHost(host.ID)

	assert.NoError(t, err)
	updated, _ := GetHost(host.ID)
	assert.Equal(t, models.StatusPaused, updated.Status)
}

func TestPauseHost_AlreadyPaused(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	host, _, _, _ := CreateAgent("host01", "192.168.1.1", false)
	database.DB.Model(&host).Update("status", models.StatusPaused)

	err := PauseHost(host.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already paused")
}

func TestPauseHost_Pending(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	host, _, _, _ := CreateAgent("host01", "192.168.1.1", false)

	err := PauseHost(host.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot pause a pending")
}

func TestResumeHost(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	host, _, _, _ := CreateAgent("host01", "192.168.1.1", false)
	database.DB.Model(&host).Update("status", models.StatusPaused)

	err := ResumeHost(host.ID)

	assert.NoError(t, err)
	updated, _ := GetHost(host.ID)
	assert.Equal(t, models.StatusOnline, updated.Status)
}

func TestResumeHost_NotPaused(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	host, _, _, _ := CreateAgent("host01", "192.168.1.1", false)
	database.DB.Model(&host).Update("status", models.StatusOnline)

	err := ResumeHost(host.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not paused")
}

func TestGenerateRegistrationToken(t *testing.T) {
	token1, hash1, err1 := generateRegistrationToken()
	token2, hash2, err2 := generateRegistrationToken()

	assert.NoError(t, err1)
	assert.NoError(t, err2)

	assert.True(t, strings.HasPrefix(token1, "wf_reg_"))
	assert.Len(t, token1, 39) // "wf_reg_" + 32 hex chars

	assert.NotEqual(t, token1, token2)
	assert.NotEqual(t, hash1, hash2)

	// Hash must differ from plaintext token.
	assert.NotEqual(t, token1, hash1)
	assert.Len(t, hash1, 64) // SHA-256 hex
}

func TestMergeCache(t *testing.T) {
	c := cache.GetCache()
	c.Remove("agent-abc")
	c.Remove("agent-xyz")
	defer c.Remove("agent-abc")

	c.Update("agent-abc", "10.0.0.1", "::1")

	hosts := []models.Host{
		{AgentID: "agent-abc", Status: models.StatusPending},
		{AgentID: "agent-xyz", Status: models.StatusPending}, // not in cache
	}
	mergeCache(hosts)

	// Agent in cache: status and IPs must be overridden.
	if hosts[0].Status != models.StatusOnline {
		t.Errorf("status: got %s, want online", hosts[0].Status)
	}
	if hosts[0].IPAddressV4 == nil || *hosts[0].IPAddressV4 != "10.0.0.1" {
		t.Errorf("ipv4: got %v, want 10.0.0.1", hosts[0].IPAddressV4)
	}
	if hosts[0].IPAddressV6 == nil || *hosts[0].IPAddressV6 != "::1" {
		t.Errorf("ipv6: got %v, want ::1", hosts[0].IPAddressV6)
	}

	// Agent not in cache: status must be unchanged.
	if hosts[1].Status != models.StatusPending {
		t.Errorf("status: got %s, want pending", hosts[1].Status)
	}
}

func TestPauseHost_MarksOpenIncidentsAsPaused(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	host, _, _, err := CreateAgent("host-paused-1", "10.0.0.1", false)
	assert.NoError(t, err)
	// Move the host out of pending so PauseHost is allowed.
	host.Status = models.StatusOnline
	assert.NoError(t, database.DB.Save(host).Error)

	open := models.AlertIncident{
		HostID:     host.ID,
		MetricType: models.MetricTypeCPUUsage,
		StartedAt:  time.Now().Add(-10 * time.Minute),
	}
	assert.NoError(t, database.DB.Create(&open).Error)

	resolved := models.AlertIncident{
		HostID:     host.ID,
		MetricType: models.MetricTypeMemoryUsage,
		StartedAt:  time.Now().Add(-1 * time.Hour),
		ResolvedAt: ptrTime(time.Now().Add(-30 * time.Minute)),
	}
	assert.NoError(t, database.DB.Create(&resolved).Error)

	assert.NoError(t, PauseHost(host.ID))

	var got models.AlertIncident
	assert.NoError(t, database.DB.First(&got, "id = ?", open.ID).Error)
	assert.NotNil(t, got.PausedAt, "open incident should be paused")
	assert.Nil(t, got.ResolvedAt, "open incident should not be resolved")

	var stillResolved models.AlertIncident
	assert.NoError(t, database.DB.First(&stillResolved, "id = ?", resolved.ID).Error)
	assert.Nil(t, stillResolved.PausedAt, "resolved incident should remain untouched")
	assert.NotNil(t, stillResolved.ResolvedAt)
}

func TestResumeHost_ClearsPausedAt(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	host, _, _, err := CreateAgent("host-resume-1", "10.0.0.2", false)
	assert.NoError(t, err)
	host.Status = models.StatusOnline
	assert.NoError(t, database.DB.Save(host).Error)

	open := models.AlertIncident{
		HostID:     host.ID,
		MetricType: models.MetricTypeCPUUsage,
		StartedAt:  time.Now().Add(-10 * time.Minute),
	}
	assert.NoError(t, database.DB.Create(&open).Error)

	assert.NoError(t, PauseHost(host.ID))
	assert.NoError(t, ResumeHost(host.ID))

	var got models.AlertIncident
	assert.NoError(t, database.DB.First(&got, "id = ?", open.ID).Error)
	assert.Nil(t, got.PausedAt, "resume should clear paused_at")
	assert.Nil(t, got.ResolvedAt, "resume should not resolve the incident")
}

func TestResumeHost_SetsStatusPending(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	host, _, _, err := CreateAgent("host-resume-pending", "10.0.0.3", false)
	assert.NoError(t, err)
	host.Status = models.StatusOnline
	assert.NoError(t, database.DB.Save(host).Error)

	assert.NoError(t, PauseHost(host.ID))
	assert.NoError(t, ResumeHost(host.ID))

	var got models.Host
	assert.NoError(t, database.DB.First(&got, "id = ?", host.ID).Error)
	assert.Equal(t, models.StatusPending, got.Status, "resume should leave the host pending until a heartbeat arrives or the stale checker promotes it")
	assert.NotNil(t, got.LastSeen, "resume should reset last_seen so the stale-pending timer starts now")
	assert.WithinDuration(t, time.Now(), *got.LastSeen, 5*time.Second)
}

func TestListHosts_PageZeroTreatedAsOne(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	CreateAgent("host01", "192.168.1.1", false)
	CreateAgent("host02", "192.168.1.2", false)

	// Page=0 must not panic and must behave like Page=1.
	hosts, total, err := ListHosts(HostListParams{Page: 0, PerPage: 1})

	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, hosts, 1)
}

func TestCreateAgent_EmptyConfiguredIP(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	host, _, _, err := CreateAgent("host01", "", false)

	assert.NoError(t, err)
	assert.Nil(t, host.ConfiguredIP) // empty string must not be stored as non-nil pointer
}

func TestHashToken(t *testing.T) {
	token1 := "test_token_123"
	token2 := "test_token_456"

	hash1 := hashToken(token1)
	hash2 := hashToken(token2)

	// Verify hashes are different
	assert.NotEqual(t, hash1, hash2)

	// Verify same token produces same hash
	hash1Again := hashToken(token1)
	assert.Equal(t, hash1, hash1Again)

	// Verify hash is SHA-256 (64 hex chars)
	assert.Len(t, hash1, 64)
}

func ptrTime(t time.Time) *time.Time { return &t }

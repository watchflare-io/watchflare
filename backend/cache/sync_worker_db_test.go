package cache

import (
	"testing"
	"time"

	"watchflare/backend/database"
	"watchflare/backend/models"
	"watchflare/backend/testutil"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedHost inserts a host with the given status and last_seen, satisfying the
// unique/not-null constraints on agent_id, agent_key and display_name.
func seedHost(t *testing.T, status string, lastSeen *time.Time) string {
	t.Helper()
	id := uuid.New().String()
	host := models.Host{
		ID:          id,
		AgentID:     uuid.New().String(),
		AgentKey:    "test-key",
		DisplayName: "test-host",
		Status:      status,
		LastSeen:    lastSeen,
	}
	require.NoError(t, database.DB.Create(&host).Error)
	return id
}

func statusOf(t *testing.T, id string) string {
	t.Helper()
	var host models.Host
	require.NoError(t, database.DB.First(&host, "id = ?", id).Error)
	return host.Status
}

func TestStaleChecker_PromoteStalePending(t *testing.T) {
	testutil.SetupTestDB(t)
	defer database.DB.Exec("DELETE FROM hosts")

	old := time.Now().Add(-1 * time.Hour)
	recent := time.Now()

	// Pending + stale last_seen: must be promoted to offline.
	stale := seedHost(t, models.StatusPending, &old)
	// Pending + recent last_seen: must stay pending.
	fresh := seedHost(t, models.StatusPending, &recent)
	// Pending + no last_seen (fresh registration): must stay pending.
	newReg := seedHost(t, models.StatusPending, nil)
	// Online + stale last_seen: must stay online (status filter).
	online := seedHost(t, models.StatusOnline, &old)

	c := NewStaleChecker(24*time.Hour, 15*time.Second)
	c.promoteStalePending()

	assert.Equal(t, models.StatusOffline, statusOf(t, stale), "stale pending host should be promoted to offline")
	assert.Equal(t, models.StatusPending, statusOf(t, fresh), "recently seen pending host should stay pending")
	assert.Equal(t, models.StatusPending, statusOf(t, newReg), "never-seen pending host should stay pending")
	assert.Equal(t, models.StatusOnline, statusOf(t, online), "online host should be untouched")
}

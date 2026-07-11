package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"watchflare/backend/database"
	"watchflare/backend/models"
	"watchflare/backend/testutil"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestListAllContainers(t *testing.T) {
	testutil.SetupTestDB(t)

	host := &models.Host{
		ID:          uuid.New().String(),
		AgentID:     uuid.New().String(),
		DisplayName: "list-containers-host",
		Status:      models.StatusOnline,
		AgentKey:    "list-containers-key",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() {
		database.DB.Where("host_id = ?", host.ID).Delete(&models.ContainerState{})
		database.DB.Unscoped().Delete(host)
	})

	require.NoError(t, database.DB.Create(&models.ContainerState{
		HostID:        host.ID,
		ContainerID:   "abc",
		ContainerName: "web",
		Image:         "nginx",
		Runtime:       "docker",
		Status:        "Up 2 hours",
		Health:        "healthy",
		UpdatedAt:     time.Now(),
	}).Error)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/containers", ListAllContainers)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/containers", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Containers []struct {
			HostID        string `json:"host_id"`
			ContainerID   string `json:"container_id"`
			ContainerName string `json:"container_name"`
			HostName      string `json:"host_name"`
			HostStatus    string `json:"host_status"`
		} `json:"containers"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	var found bool
	for _, c := range body.Containers {
		if c.ContainerID == "abc" && c.HostID == host.ID {
			found = true
			require.Equal(t, "web", c.ContainerName)
			require.Equal(t, "list-containers-host", c.HostName)
			require.Equal(t, "online", c.HostStatus)
		}
	}
	require.True(t, found, "expected container abc in response")
}

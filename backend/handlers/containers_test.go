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
		database.DB.Where("host_id = ?", host.ID).Delete(&models.ContainerImageUpdate{})
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
	require.NoError(t, database.DB.Create(&models.ContainerState{
		HostID:        host.ID,
		ContainerID:   "def",
		ContainerName: "unchecked",
		Image:         "redis",
		UpdatedAt:     time.Now(),
	}).Error)
	require.NoError(t, database.DB.Create(&models.ContainerImageUpdate{
		HostID:        host.ID,
		ContainerID:   "abc",
		Image:         "nginx",
		LocalDigest:   "sha256:old",
		RemoteDigest:  "sha256:new",
		Status:        "update_available",
		CheckedAt:     time.Now(),
		LocalVersion:  "1.27.0",
		RemoteVersion: "1.27.3",
		CommitsBehind: 0,
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

			ImageUpdateStatus  string     `json:"image_update_status"`
			ImageRemoteDigest  string     `json:"image_remote_digest"`
			ImageUpdateChecked *time.Time `json:"image_update_checked_at"`
			ImageLocalVersion  string     `json:"image_local_version"`
			ImageRemoteVersion string     `json:"image_remote_version"`
			ImageCommitsBehind int        `json:"image_commits_behind"`
		} `json:"containers"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	var found, foundUnchecked bool
	for _, c := range body.Containers {
		if c.HostID != host.ID {
			continue
		}
		switch c.ContainerID {
		case "abc":
			found = true
			require.Equal(t, "web", c.ContainerName)
			require.Equal(t, "list-containers-host", c.HostName)
			require.Equal(t, "online", c.HostStatus)
			require.Equal(t, "update_available", c.ImageUpdateStatus)
			require.Equal(t, "sha256:new", c.ImageRemoteDigest)
			require.NotNil(t, c.ImageUpdateChecked)
			require.Equal(t, "1.27.0", c.ImageLocalVersion)
			require.Equal(t, "1.27.3", c.ImageRemoteVersion)
			require.Zero(t, c.ImageCommitsBehind)
		case "def":
			foundUnchecked = true
			require.Empty(t, c.ImageUpdateStatus)
			require.Nil(t, c.ImageUpdateChecked)
		}
	}
	require.True(t, found, "expected container abc in response")
	require.True(t, foundUnchecked, "expected unchecked container def in response")
}

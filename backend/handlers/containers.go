package handlers

import (
	"net/http"
	"time"

	"watchflare/backend/database"
	"watchflare/backend/models"

	"github.com/gin-gonic/gin"
)

// ListAllContainers returns the current container state across all hosts,
// joined with host name and status so the UI can show ownership and Live/Stale,
// and with the latest image update check (empty status if never checked).
func ListAllContainers(c *gin.Context) {
	type globalContainer struct {
		models.ContainerState
		HostName           string     `json:"host_name"`
		HostStatus         string     `json:"host_status"`
		ImageUpdateStatus  string     `json:"image_update_status"`
		ImageLocalDigest   string     `json:"image_local_digest"`
		ImageRemoteDigest  string     `json:"image_remote_digest"`
		ImageUpdateError   string     `json:"image_update_error"`
		ImageUpdateChecked *time.Time `json:"image_update_checked_at"`
		ImageLocalVersion  string     `json:"image_local_version"`
		ImageRemoteVersion string     `json:"image_remote_version"`
		ImageCommitsBehind int        `json:"image_commits_behind"`
	}

	containers := []globalContainer{}
	err := database.DB.
		Table("container_states AS cs").
		Select(`cs.*, h.display_name AS host_name, h.status AS host_status,
			COALESCE(iu.status, '') AS image_update_status,
			COALESCE(iu.local_digest, '') AS image_local_digest,
			COALESCE(iu.remote_digest, '') AS image_remote_digest,
			COALESCE(iu.error, '') AS image_update_error,
			iu.checked_at AS image_update_checked,
			COALESCE(iu.local_version, '') AS image_local_version,
			COALESCE(iu.remote_version, '') AS image_remote_version,
			COALESCE(iu.commits_behind, 0) AS image_commits_behind`).
		Joins("JOIN hosts h ON h.id = cs.host_id").
		Joins("LEFT JOIN container_image_updates iu ON iu.host_id = cs.host_id AND iu.container_id = cs.container_id").
		Order("h.display_name ASC, cs.container_name ASC").
		Scan(&containers).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch containers"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"containers": containers})
}

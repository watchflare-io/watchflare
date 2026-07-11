package handlers

import (
	"net/http"

	"watchflare/backend/database"
	"watchflare/backend/models"

	"github.com/gin-gonic/gin"
)

// ListAllContainers returns the current container state across all hosts,
// joined with host name and status so the UI can show ownership and Live/Stale.
func ListAllContainers(c *gin.Context) {
	type globalContainer struct {
		models.ContainerState
		HostName   string `json:"host_name"`
		HostStatus string `json:"host_status"`
	}

	containers := []globalContainer{}
	err := database.DB.
		Table("container_states AS cs").
		Select("cs.*, h.display_name AS host_name, h.status AS host_status").
		Joins("JOIN hosts h ON h.id = cs.host_id").
		Order("h.display_name ASC, cs.container_name ASC").
		Scan(&containers).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch containers"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"containers": containers})
}

package handlers

import (
	"net/http"

	"watchflare/backend/database"
	"watchflare/backend/models"

	"github.com/gin-gonic/gin"
)

func GetHostServices(c *gin.Context) {
	hostID := c.Param("id")

	var services []models.Service
	if err := database.DB.Where("host_id = ?", hostID).
		Order("(active_state = 'failed') DESC, name ASC").
		Find(&services).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch services"})
		return
	}

	failed := 0
	for _, s := range services {
		if s.ActiveState == "failed" {
			failed++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"services": services,
		"summary":  gin.H{"total": len(services), "failed": failed},
	})
}

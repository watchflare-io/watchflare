package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"watchflare/backend/database"
	"watchflare/backend/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGetHostServices_FailedFirstAndSummary(t *testing.T) {
	setupTestDB(t)
	defer database.DB.Exec("DELETE FROM services")
	defer teardownTestDB()

	hostID := uuid.New().String()
	seedHost(t, hostID)

	require.NoError(t, database.DB.Create(&[]models.Service{
		{HostID: hostID, Name: "z.service", ActiveState: "active"},
		{HostID: hostID, Name: "a.service", ActiveState: "failed"},
	}).Error)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.GET("/hosts/:id/services", GetHostServices)

	req := httptest.NewRequest("GET", "/hosts/"+hostID+"/services", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Services []models.Service `json:"services"`
		Summary  struct {
			Total  int `json:"total"`
			Failed int `json:"failed"`
		} `json:"summary"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	if body.Summary.Total != 2 || body.Summary.Failed != 1 {
		t.Fatalf("summary wrong: %+v", body.Summary)
	}
	if body.Services[0].Name != "a.service" {
		t.Fatalf("failed service should sort first, got %s", body.Services[0].Name)
	}
}

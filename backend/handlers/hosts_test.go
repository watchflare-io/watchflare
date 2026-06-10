package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"watchflare/backend/database"
	"watchflare/backend/middleware"
	"watchflare/backend/models"
	"watchflare/backend/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// setupHostRouter creates a test router with host routes
func setupHostRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	hostGroup := router.Group("/hosts")
	hostGroup.Use(middleware.AuthMiddleware())
	{
		hostGroup.POST("", CreateAgent)
		hostGroup.GET("", ListHosts)
		hostGroup.GET("/:id", GetHost)
		hostGroup.PUT("/:id/validate-ip", ValidateIP)
		hostGroup.PUT("/:id/rename", RenameHost)
		hostGroup.PUT("/:id/change-ip", UpdateConfiguredIP)
		hostGroup.PUT("/:id/ignore-ip-mismatch", IgnoreIPMismatch)
		hostGroup.PUT("/:id/dismiss-reactivation", DismissReactivation)
		hostGroup.PUT("/:id/pause", PauseHost)
		hostGroup.PUT("/:id/resume", ResumeHost)
		hostGroup.POST("/:id/regenerate-token", RegenerateToken)
		hostGroup.DELETE("/:id", DeleteHost)
	}

	return router
}

// createTestUser creates a test user and returns JWT cookie
func createTestUser(t *testing.T) *http.Cookie {
	testUser := &models.User{
		Email: "test@test.com",
	}
	testUser.HashPassword("password123")
	database.DB.Create(testUser)

	// Generate JWT
	result, _ := services.Login("test@test.com", "password123")

	return &http.Cookie{
		Name:  "jwt_token",
		Value: result.Token,
	}
}

func TestCreateAgent(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupHostRouter()
	cookie := createTestUser(t)

	tests := []struct {
		name           string
		payload        map[string]interface{}
		withCookie     bool
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "Success - Create pending host",
			payload: map[string]interface{}{
				"display_name":  "host01",
				"configured_ip": "192.168.1.100",
				"allow_any_ip":  false,
			},
			withCookie:     true,
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Host created successfully", resp["message"])
				assert.NotNil(t, resp["host"])
				assert.NotNil(t, resp["token"])
				assert.NotNil(t, resp["agent_key"])

				host := resp["host"].(map[string]interface{})
				assert.Equal(t, "host01", host["display_name"])
				assert.Equal(t, models.StatusPending, host["status"])
			},
		},
		{
			name: "Success - AllowAnyIP without configured_ip",
			payload: map[string]interface{}{
				"display_name": "host02",
				"allow_any_ip": true,
			},
			withCookie:     true,
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.NotNil(t, resp["host"])
			},
		},
		{
			name: "Fail - Missing configured_ip when allow_any_ip is false",
			payload: map[string]interface{}{
				"display_name": "host03",
			},
			withCookie:     true,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.NotNil(t, resp["error"])
			},
		},
		{
			name: "Fail - No authentication",
			payload: map[string]interface{}{
				"display_name":  "host04",
				"configured_ip": "192.168.1.102",
			},
			withCookie:     false,
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.NotNil(t, resp["error"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req, _ := http.NewRequest("POST", "/hosts", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			if tt.withCookie {
				req.AddCookie(cookie)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)
			tt.checkResponse(t, response)
		})
	}
}

func TestListHosts(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupHostRouter()
	cookie := createTestUser(t)

	// Create test hosts
	host1, _, _, _ := services.CreateAgent("host01", "192.168.1.100", false)
	host2, _, _, _ := services.CreateAgent("host02", "192.168.1.101", true)

	req, _ := http.NewRequest("GET", "/hosts", nil)
	req.AddCookie(cookie)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	hosts := response["hosts"].([]interface{})
	assert.Len(t, hosts, 2)

	// Collect IDs and names — order is not guaranteed (sorted by created_at)
	ids := []string{}
	names := []string{}
	for _, h := range hosts {
		hst := h.(map[string]interface{})
		ids = append(ids, hst["id"].(string))
		names = append(names, hst["display_name"].(string))
		assert.Equal(t, models.StatusPending, hst["status"])
	}
	assert.Contains(t, ids, host1.ID)
	assert.Contains(t, ids, host2.ID)
	assert.Contains(t, names, "host01")
	assert.Contains(t, names, "host02")
}

func TestGetHost(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupHostRouter()
	cookie := createTestUser(t)

	// Create test host
	host, _, _, _ := services.CreateAgent("host01", "192.168.1.100", false)

	tests := []struct {
		name           string
		hostID         string
		withCookie     bool
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:           "Success - Get existing host (no metrics yet)",
			hostID:         host.ID,
			withCookie:     true,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				hostData := resp["host"].(map[string]interface{})
				assert.Equal(t, "host01", hostData["display_name"])
				assert.Equal(t, models.StatusPending, hostData["status"])
				assert.Nil(t, resp["latest_metrics"])
			},
		},
		{
			name:           "Fail - Host not found",
			hostID:         "00000000-0000-0000-0000-000000000000",
			withCookie:     true,
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "not found")
			},
		},
		{
			name:           "Fail - Invalid host ID",
			hostID:         "invalid",
			withCookie:     true,
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/hosts/"+tt.hostID, nil)

			if tt.withCookie {
				req.AddCookie(cookie)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)
			tt.checkResponse(t, response)
		})
	}
}

func TestGetHost_WithLatestMetrics(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	defer database.DB.Exec("DELETE FROM metrics")

	router := setupHostRouter()
	cookie := createTestUser(t)

	host, _, _, _ := services.CreateAgent("host01", "192.168.1.100", false)

	// Insert a recent metric for the host
	metric := models.Metric{
		HostID:          host.ID,
		Timestamp:       time.Now(),
		CPUUsagePercent: 42.5,
		MemoryTotalBytes: 8 * 1024 * 1024 * 1024,
		MemoryUsedBytes:  4 * 1024 * 1024 * 1024,
		UptimeSeconds:   3600,
	}
	database.DB.Create(&metric)

	req, _ := http.NewRequest("GET", "/hosts/"+host.ID, nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.NotNil(t, response["latest_metrics"])
	m := response["latest_metrics"].(map[string]interface{})
	assert.InDelta(t, 42.5, m["cpu_usage_percent"], 0.01)
	assert.InDelta(t, float64(8*1024*1024*1024), m["memory_total_bytes"], 1)
	assert.InDelta(t, float64(3600), m["uptime_seconds"], 1)
}

func TestRegenerateToken(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupHostRouter()
	cookie := createTestUser(t)

	// Create test host
	host, _, _, _ := services.CreateAgent("host01", "192.168.1.100", false)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/hosts/%s/regenerate-token", host.ID), nil)
	req.AddCookie(cookie)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.Equal(t, "Token regenerated successfully", response["message"])
	assert.NotNil(t, response["token"])

	// Verify token format
	token := response["token"].(string)
	assert.Contains(t, token, "wf_reg_")
}

func TestDeleteHost(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupHostRouter()
	cookie := createTestUser(t)

	tests := []struct {
		name           string
		setupHost      func() string
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "Success - Delete pending host",
			setupHost: func() string {
				host, _, _, _ := services.CreateAgent("host01", "192.168.1.100", false)
				return host.ID
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Host deleted successfully", resp["message"])
			},
		},
		{
			name: "Fail - Delete non-existent host",
			setupHost: func() string {
				return "00000000-0000-0000-0000-000000000000"
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hostID := tt.setupHost()

			req, _ := http.NewRequest("DELETE", fmt.Sprintf("/hosts/%s", hostID), nil)
			req.AddCookie(cookie)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)
			tt.checkResponse(t, response)
		})
	}
}

func TestValidateIP(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupHostRouter()
	cookie := createTestUser(t)

	// Create test host
	host, _, _, _ := services.CreateAgent("host01", "192.168.1.100", false)

	payload := map[string]string{
		"selected_ip": "192.168.1.100",
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/hosts/%s/validate-ip", host.ID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "IP validated successfully", response["message"])
}

func TestUpdateConfiguredIP(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupHostRouter()
	cookie := createTestUser(t)

	// Create test host
	host, _, _, _ := services.CreateAgent("host01", "192.168.1.100", false)

	payload := map[string]string{
		"new_ip": "192.168.1.200",
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/hosts/%s/change-ip", host.ID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Configured IP updated successfully", response["message"])

	// Verify IP was updated
	updatedHost, _ := services.GetHost(host.ID)
	assert.Equal(t, "192.168.1.200", *updatedHost.ConfiguredIP)
}

func TestRenameHost(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupHostRouter()
	cookie := createTestUser(t)

	host, _, _, _ := services.CreateAgent("host01", "192.168.1.100", false)

	tests := []struct {
		name           string
		hostID         string
		payload        map[string]string
		expectedStatus int
	}{
		{
			name:           "Success",
			hostID:         host.ID,
			payload:        map[string]string{"new_name": "renamed-host"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Fail - Name too short",
			hostID:         host.ID,
			payload:        map[string]string{"new_name": "x"},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Fail - Host not found",
			hostID:         "00000000-0000-0000-0000-000000000000",
			payload:        map[string]string{"new_name": "renamed"},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req, _ := http.NewRequest("PUT", fmt.Sprintf("/hosts/%s/rename", tt.hostID), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(cookie)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestPauseResumeHost(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupHostRouter()
	cookie := createTestUser(t)

	host, _, _, _ := services.CreateAgent("host01", "192.168.1.100", false)

	// Cannot pause a pending host.
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/hosts/%s/pause", host.ID), nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Set to online so pause works.
	database.DB.Model(&models.Host{}).Where("id = ?", host.ID).Update("status", models.StatusOnline)

	req, _ = http.NewRequest("PUT", fmt.Sprintf("/hosts/%s/pause", host.ID), nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Already paused.
	req, _ = http.NewRequest("PUT", fmt.Sprintf("/hosts/%s/pause", host.ID), nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Resume.
	req, _ = http.NewRequest("PUT", fmt.Sprintf("/hosts/%s/resume", host.ID), nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Cannot resume a non-paused host.
	req, _ = http.NewRequest("PUT", fmt.Sprintf("/hosts/%s/resume", host.ID), nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Pause/resume host not found → 404.
	req, _ = http.NewRequest("PUT", "/hosts/00000000-0000-0000-0000-000000000000/pause", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestIgnoreIPMismatch(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupHostRouter()
	cookie := createTestUser(t)

	host, _, _, _ := services.CreateAgent("host01", "192.168.1.100", false)

	req, _ := http.NewRequest("PUT", fmt.Sprintf("/hosts/%s/ignore-ip-mismatch", host.ID), nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Not found.
	req, _ = http.NewRequest("PUT", "/hosts/00000000-0000-0000-0000-000000000000/ignore-ip-mismatch", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDismissReactivation(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupHostRouter()
	cookie := createTestUser(t)

	host, _, _, _ := services.CreateAgent("host01", "192.168.1.100", false)

	req, _ := http.NewRequest("PUT", fmt.Sprintf("/hosts/%s/dismiss-reactivation", host.ID), nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Not found.
	req, _ = http.NewRequest("PUT", "/hosts/00000000-0000-0000-0000-000000000000/dismiss-reactivation", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

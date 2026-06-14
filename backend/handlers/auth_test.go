package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"watchflare/backend/config"
	"watchflare/backend/database"
	"watchflare/backend/middleware"
	"watchflare/backend/models"

	"github.com/gin-gonic/gin"
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
		JWTSecret:                 "test-secret-key-must-be-32-chars!!",
		NotificationEncryptionKey: "test-encryption-key-32-chars-long!",
	}
	if err := database.Connect(testDSN()); err != nil {
		t.Skipf("skipping test: database unavailable: %v", err)
	}
}

// teardownTestDB cleans up the test database
func teardownTestDB() {
	database.DB.Exec("DELETE FROM hosts")
	database.DB.Exec("DELETE FROM users")
}

// setupRouter creates a test router with routes
func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Auth routes (public)
	authGroup := router.Group("/auth")
	{
		authGroup.GET("/setup-required", SetupRequired)
		authGroup.POST("/register", Register)
		authGroup.POST("/login", Login)
		authGroup.POST("/logout", Logout)
	}

	// Protected routes
	protectedGroup := router.Group("/auth")
	protectedGroup.Use(middleware.AuthMiddleware())
	{
		protectedGroup.GET("/user", GetCurrentUser)
		protectedGroup.PUT("/change-password", ChangePassword)
		protectedGroup.PUT("/change-email", ChangeEmail)
		protectedGroup.PUT("/change-username", ChangeUsername)
		protectedGroup.PUT("/preferences", UpdatePreferences)
	}

	return router
}

// loginAndGetCookie registers a user (or reuses existing), logs in, and returns the jwt_token cookie.
func loginAndGetCookie(t *testing.T, router *gin.Engine, email, password string) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	for _, c := range w.Result().Cookies() {
		if c.Name == "jwt_token" {
			return c
		}
	}
	t.Fatal("jwt_token cookie not found after login")
	return nil
}

// TestRegister tests user registration
func TestRegister(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupRouter()

	tests := []struct {
		name           string
		payload        map[string]string
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "Success - First admin registration",
			payload: map[string]string{
				"email":    "admin@test.com",
				"password": "password123",
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "User registered successfully", resp["message"])
				assert.NotNil(t, resp["user"])
			},
		},
		{
			name: "Fail - Second admin registration",
			payload: map[string]string{
				"email":    "admin2@test.com",
				"password": "password456",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "admin user already exists")
			},
		},
		{
			name: "Fail - Invalid email",
			payload: map[string]string{
				"email":    "invalid-email",
				"password": "password123",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.NotNil(t, resp["error"])
			},
		},
		{
			name: "Fail - Password too short",
			payload: map[string]string{
				"email":    "test@test.com",
				"password": "short",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.NotNil(t, resp["error"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)
			tt.checkResponse(t, response)
		})
	}
}

// TestLogin tests user login
func TestLogin(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupRouter()

	// Create a test user
	testUser := &models.User{
		Email: "test@test.com",
	}
	testUser.HashPassword("password123")
	database.DB.Create(testUser)

	tests := []struct {
		name           string
		payload        map[string]string
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder, map[string]interface{})
	}{
		{
			name: "Success - Valid credentials",
			payload: map[string]string{
				"email":    "test@test.com",
				"password": "password123",
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp map[string]interface{}) {
				assert.Equal(t, "Login successful", resp["message"])
				// Check cookie
				cookies := w.Result().Cookies()
				assert.NotEmpty(t, cookies)
				var jwtCookie *http.Cookie
				for _, cookie := range cookies {
					if cookie.Name == "jwt_token" {
						jwtCookie = cookie
						break
					}
				}
				assert.NotNil(t, jwtCookie)
				assert.NotEmpty(t, jwtCookie.Value)
				assert.True(t, jwtCookie.HttpOnly)
				assert.False(t, jwtCookie.Secure) // plain HTTP in tests: no TLS, no trusted proxy
			},
		},
		{
			name: "Fail - Wrong password",
			payload: map[string]string{
				"email":    "test@test.com",
				"password": "wrongpassword",
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "invalid credentials")
			},
		},
		{
			name: "Fail - User not found",
			payload: map[string]string{
				"email":    "notfound@test.com",
				"password": "password123",
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "invalid credentials")
			},
		},
		{
			name: "Fail - Invalid email format",
			payload: map[string]string{
				"email":    "invalid-email",
				"password": "password123",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder, resp map[string]interface{}) {
				assert.NotNil(t, resp["error"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)
			tt.checkResponse(t, w, response)
		})
	}
}

// TestLogout tests user logout
func TestLogout(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupRouter()

	req, _ := http.NewRequest("POST", "/auth/logout", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Logout successful", response["message"])

	// Check cookie is cleared
	cookies := w.Result().Cookies()
	var jwtCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "jwt_token" {
			jwtCookie = cookie
			break
		}
	}
	assert.NotNil(t, jwtCookie)
	assert.Equal(t, "", jwtCookie.Value)
	assert.Equal(t, -1, jwtCookie.MaxAge)
}

// TestChangePassword tests password change
func TestChangePassword(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupRouter()

	// Create a test user and login
	testUser := &models.User{
		Email: "test@test.com",
	}
	testUser.HashPassword("oldpassword123")
	database.DB.Create(testUser)

	// Login to get JWT cookie
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "test@test.com",
		"password": "oldpassword123",
	})
	loginReq, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	router.ServeHTTP(loginW, loginReq)

	// Extract JWT cookie
	var jwtCookie *http.Cookie
	for _, cookie := range loginW.Result().Cookies() {
		if cookie.Name == "jwt_token" {
			jwtCookie = cookie
			break
		}
	}

	tests := []struct {
		name           string
		payload        map[string]string
		withCookie     bool
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "Success - Valid password change",
			payload: map[string]string{
				"current_password": "oldpassword123",
				"new_password":     "newpassword456",
			},
			withCookie:     true,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Password changed successfully", resp["message"])
			},
		},
		{
			name: "Fail - Wrong current password",
			payload: map[string]string{
				"current_password": "wrongpassword",
				"new_password":     "newpassword456",
			},
			withCookie:     true,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "current password is incorrect")
			},
		},
		{
			name: "Fail - New password too short",
			payload: map[string]string{
				"current_password": "oldpassword123",
				"new_password":     "short",
			},
			withCookie:     true,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.NotNil(t, resp["error"])
			},
		},
		{
			name: "Fail - No authentication",
			payload: map[string]string{
				"current_password": "oldpassword123",
				"new_password":     "newpassword456",
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
			req, _ := http.NewRequest("PUT", "/auth/change-password", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			if tt.withCookie {
				req.AddCookie(jwtCookie)
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

func TestSetupRequired(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupRouter()

	// No users → setup required.
	req, _ := http.NewRequest("GET", "/auth/setup-required", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["setup_required"])

	// Create a user → setup no longer required.
	u := &models.User{Email: "admin@test.com"}
	u.HashPassword("password123")
	database.DB.Create(u)

	req, _ = http.NewRequest("GET", "/auth/setup-required", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["setup_required"])
}

func TestGetCurrentUser(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupRouter()

	u := &models.User{Email: "me@test.com"}
	u.HashPassword("password123")
	database.DB.Create(u)
	cookie := loginAndGetCookie(t, router, "me@test.com", "password123")

	// Authenticated → returns user.
	req, _ := http.NewRequest("GET", "/auth/user", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotNil(t, resp["user"])

	// Unauthenticated → 401.
	req, _ = http.NewRequest("GET", "/auth/user", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestChangeEmail(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupRouter()

	u := &models.User{Email: "original@test.com"}
	u.HashPassword("password123")
	database.DB.Create(u)
	cookie := loginAndGetCookie(t, router, "original@test.com", "password123")

	// Success.
	body, _ := json.Marshal(map[string]string{"new_email": "updated@test.com"})
	req, _ := http.NewRequest("PUT", "/auth/change-email", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Duplicate email: create a second user then try to steal its email.
	u2 := &models.User{Email: "taken@test.com"}
	u2.HashPassword("password123")
	database.DB.Create(u2)

	body, _ = json.Marshal(map[string]string{"new_email": "taken@test.com"})
	req, _ = http.NewRequest("PUT", "/auth/change-email", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusConflict, w.Code)

	// Unauthenticated → 401.
	body, _ = json.Marshal(map[string]string{"new_email": "x@test.com"})
	req, _ = http.NewRequest("PUT", "/auth/change-email", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestChangeUsername(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupRouter()

	u := &models.User{Email: "user@test.com"}
	u.HashPassword("password123")
	database.DB.Create(u)
	cookie := loginAndGetCookie(t, router, "user@test.com", "password123")

	// Success.
	body, _ := json.Marshal(map[string]string{"username": "newname"})
	req, _ := http.NewRequest("PUT", "/auth/change-username", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotNil(t, resp["user"])

	// Unauthenticated → 401.
	req, _ = http.NewRequest("PUT", "/auth/change-username", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUpdatePreferences(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	router := setupRouter()

	u := &models.User{Email: "prefs@test.com"}
	u.HashPassword("password123")
	database.DB.Create(u)
	cookie := loginAndGetCookie(t, router, "prefs@test.com", "password123")

	tests := []struct {
		name           string
		payload        map[string]interface{}
		withCookie     bool
		expectedStatus int
	}{
		{
			name:           "Success - Valid preferences",
			payload:        map[string]interface{}{"default_time_range": "7d", "theme": "dark"},
			withCookie:     true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Fail - Invalid time range",
			payload:        map[string]interface{}{"default_time_range": "999y"},
			withCookie:     true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Fail - Invalid theme",
			payload:        map[string]interface{}{"theme": "rainbow"},
			withCookie:     true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Fail - Invalid temperature unit",
			payload:        map[string]interface{}{"temperature_unit": "kelvin"},
			withCookie:     true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Fail - Gauge warning out of range",
			payload:        map[string]interface{}{"gauge_warning_threshold": 0},
			withCookie:     true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Fail - Unauthenticated",
			payload:        map[string]interface{}{"theme": "dark"},
			withCookie:     false,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req, _ := http.NewRequest("PUT", "/auth/preferences", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			if tt.withCookie {
				req.AddCookie(cookie)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestCookieSecureFlag_Login verifies that the Secure flag on the JWT cookie
// follows per-request HTTPS detection logic.
func TestCookieSecureFlag_Login(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	user := &models.User{Email: "secure@test.com"}
	user.HashPassword("password123")
	database.DB.Create(user)

	loginBody, _ := json.Marshal(map[string]string{
		"email":    "secure@test.com",
		"password": "password123",
	})

	getJWTCookie := func(w *httptest.ResponseRecorder) *http.Cookie {
		for _, c := range w.Result().Cookies() {
			if c.Name == "jwt_token" {
				return c
			}
		}
		return nil
	}

	t.Run("plain HTTP: Secure=false", func(t *testing.T) {
		config.AppConfig.CookieSecureOverride = nil
		config.AppConfig.TrustedProxies = []string{"127.0.0.1", "::1"}

		router := setupRouter()
		req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(loginBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		cookie := getJWTCookie(w)
		assert.NotNil(t, cookie)
		assert.False(t, cookie.Secure)
	})

	t.Run("trusted proxy with X-Forwarded-Proto https: Secure=true", func(t *testing.T) {
		config.AppConfig.CookieSecureOverride = nil
		config.AppConfig.TrustedProxies = []string{"127.0.0.1", "::1"}

		router := setupRouter()
		req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(loginBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.RemoteAddr = "127.0.0.1:54321"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		cookie := getJWTCookie(w)
		assert.NotNil(t, cookie)
		assert.True(t, cookie.Secure)
	})

	t.Run("untrusted source with X-Forwarded-Proto https: Secure=false", func(t *testing.T) {
		config.AppConfig.CookieSecureOverride = nil
		config.AppConfig.TrustedProxies = []string{"127.0.0.1", "::1"}

		router := setupRouter()
		req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(loginBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.RemoteAddr = "10.0.0.99:54321"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		cookie := getJWTCookie(w)
		assert.NotNil(t, cookie)
		assert.False(t, cookie.Secure)
	})

	t.Run("COOKIE_SECURE override true: Secure=true regardless", func(t *testing.T) {
		override := true
		config.AppConfig.CookieSecureOverride = &override
		config.AppConfig.TrustedProxies = []string{}
		defer func() { config.AppConfig.CookieSecureOverride = nil }()

		router := setupRouter()
		req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(loginBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		cookie := getJWTCookie(w)
		assert.NotNil(t, cookie)
		assert.True(t, cookie.Secure)
	})
}

package handlers

import (
	"errors"
	"net/http"
	"watchflare/backend/config"
	"watchflare/backend/services"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

// setJWTCookie sets the JWT token as an HttpOnly cookie.
func setJWTCookie(c *gin.Context, token string) {
	domain := config.AppConfig.CookieDomain
	secure := config.CookieSecure(c.Request.TLS != nil, c.Request.RemoteAddr, c.GetHeader("X-Forwarded-Proto"))
	c.SetCookie("jwt_token", token, 60*60*24*7, "/", domain, secure, true)
}

// getUserID extracts the authenticated user ID from the Gin context
func getUserID(c *gin.Context) (string, bool) {
	val, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return "", false
	}
	id, ok := val.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id type"})
		return "", false
	}
	return id, true
}

// RegisterRequest represents the registration request body
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Username string `json:"username" binding:"max=50"`
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// ChangePasswordRequest represents the change password request body
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

// ChangeEmailRequest represents the change email request body
type ChangeEmailRequest struct {
	NewEmail string `json:"new_email" binding:"required,email"`
}

// ChangeUsernameRequest represents the change username request body
type ChangeUsernameRequest struct {
	Username string `json:"username" binding:"min=1,max=50"`
}

// UpdatePreferencesRequest represents the update preferences request body
type UpdatePreferencesRequest struct {
	DefaultTimeRange       string `json:"default_time_range"`
	Theme                  string `json:"theme"`
	TimeFormat             string `json:"time_format"`
	TemperatureUnit        string `json:"temperature_unit"`
	NetworkUnit            string `json:"network_unit"`
	DiskUnit               string `json:"disk_unit"`
	GaugeWarningThreshold  *int   `json:"gauge_warning_threshold"`
	GaugeCriticalThreshold *int   `json:"gauge_critical_threshold"`
}

// Register creates the first admin user and automatically logs them in
func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, token, err := services.Register(req.Email, req.Password, req.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	setJWTCookie(c, token)

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user":    user,
	})
}

// Login authenticates a user and sets JWT token in HttpOnly cookie
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := services.Login(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, services.ErrServiceUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service unavailable, please try again later"})
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		}
		return
	}

	setJWTCookie(c, token)

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
	})
}

// Logout clears the JWT cookie
func Logout(c *gin.Context) {
	domain := config.AppConfig.CookieDomain
	secure := config.CookieSecure(c.Request.TLS != nil, c.Request.RemoteAddr, c.GetHeader("X-Forwarded-Proto"))
	c.SetCookie("jwt_token", "", -1, "/", domain, secure, true)
	c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
}

// ChangePassword updates the authenticated user's password
func ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, ok := getUserID(c)
	if !ok {
		return
	}

	err := services.ChangePassword(userID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Password changed successfully",
	})
}

// ChangeEmail updates the authenticated user's email
func ChangeEmail(c *gin.Context) {
	var req ChangeEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, ok := getUserID(c)
	if !ok {
		return
	}

	if err := services.ChangeEmail(userID, req.NewEmail); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{"error": "email already in use"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update email"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Email updated successfully",
	})
}

// ChangeUsername updates the authenticated user's username
func ChangeUsername(c *gin.Context) {
	var req ChangeUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, ok := getUserID(c)
	if !ok {
		return
	}

	user, err := services.ChangeUsername(userID, req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update username"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Username updated successfully",
		"user":    user,
	})
}

// SetupRequired checks if initial setup is required (no users exist)
func SetupRequired(c *gin.Context) {
	required, err := services.IsSetupRequired()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check setup status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"setup_required": required,
	})
}

// GetCurrentUser returns the authenticated user's information including preferences
func GetCurrentUser(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}

	user, err := services.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}

// UpdatePreferences updates the authenticated user's preferences
func UpdatePreferences(c *gin.Context) {
	var req UpdatePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, ok := getUserID(c)
	if !ok {
		return
	}

	// Validate preferences
	if req.DefaultTimeRange != "" && req.DefaultTimeRange != "1h" && req.DefaultTimeRange != "12h" && req.DefaultTimeRange != "24h" && req.DefaultTimeRange != "7d" && req.DefaultTimeRange != "30d" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid default_time_range, valid values: 1h, 12h, 24h, 7d, 30d"})
		return
	}

	if req.Theme != "" && req.Theme != "light" && req.Theme != "dark" && req.Theme != "system" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid theme, valid values: light, dark, system"})
		return
	}

	// Validate time_format
	if req.TimeFormat != "" && req.TimeFormat != "24h" && req.TimeFormat != "12h" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time_format, valid values: 24h, 12h"})
		return
	}

	// Validate temperature_unit
	if req.TemperatureUnit != "" && req.TemperatureUnit != "celsius" && req.TemperatureUnit != "fahrenheit" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid temperature_unit, valid values: celsius, fahrenheit"})
		return
	}

	// Validate network_unit
	if req.NetworkUnit != "" && req.NetworkUnit != "bytes" && req.NetworkUnit != "bits" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid network_unit, valid values: bytes, bits"})
		return
	}

	// Validate disk_unit
	if req.DiskUnit != "" && req.DiskUnit != "bytes" && req.DiskUnit != "bits" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid disk_unit, valid values: bytes, bits"})
		return
	}

	// Validate gauge thresholds
	if req.GaugeWarningThreshold != nil && (*req.GaugeWarningThreshold < 1 || *req.GaugeWarningThreshold > 99) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid gauge_warning_threshold, must be between 1 and 99"})
		return
	}
	if req.GaugeCriticalThreshold != nil && (*req.GaugeCriticalThreshold < 1 || *req.GaugeCriticalThreshold > 100) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid gauge_critical_threshold, must be between 1 and 100"})
		return
	}
	if req.GaugeWarningThreshold != nil && req.GaugeCriticalThreshold != nil &&
		*req.GaugeWarningThreshold >= *req.GaugeCriticalThreshold {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gauge_warning_threshold must be less than gauge_critical_threshold"})
		return
	}

	// Build updates map
	updates := make(map[string]interface{})
	if req.DefaultTimeRange != "" {
		updates["default_time_range"] = req.DefaultTimeRange
	}
	if req.Theme != "" {
		updates["theme"] = req.Theme
	}
	if req.TimeFormat != "" {
		updates["time_format"] = req.TimeFormat
	}
	if req.TemperatureUnit != "" {
		updates["temperature_unit"] = req.TemperatureUnit
	}
	if req.NetworkUnit != "" {
		updates["network_unit"] = req.NetworkUnit
	}
	if req.DiskUnit != "" {
		updates["disk_unit"] = req.DiskUnit
	}
	if req.GaugeWarningThreshold != nil {
		updates["gauge_warning_threshold"] = *req.GaugeWarningThreshold
	}
	if req.GaugeCriticalThreshold != nil {
		updates["gauge_critical_threshold"] = *req.GaugeCriticalThreshold
	}

	user, err := services.UpdatePreferences(userID, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update preferences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Preferences updated successfully",
		"user":    user,
	})
}

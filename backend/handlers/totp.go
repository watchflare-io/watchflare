package handlers

import (
	"net/http"
	"watchflare/backend/config"
	"watchflare/backend/services"

	"github.com/gin-gonic/gin"
)

func setPreAuthCookie(c *gin.Context, userID string) error {
	token, err := services.CreatePreAuthToken(userID)
	if err != nil {
		return err
	}
	domain := config.AppConfig.CookieDomain
	secure := config.CookieSecure(c.Request.TLS != nil, c.Request.RemoteAddr, c.GetHeader("X-Forwarded-Proto"))
	c.SetCookie("pre_auth", token, 300, "/", domain, secure, true)
	return nil
}

func clearPreAuthCookie(c *gin.Context) {
	domain := config.AppConfig.CookieDomain
	secure := config.CookieSecure(c.Request.TLS != nil, c.Request.RemoteAddr, c.GetHeader("X-Forwarded-Proto"))
	c.SetCookie("pre_auth", "", -1, "/", domain, secure, true)
}

func VerifyTOTP(c *gin.Context) {
	preAuthToken, err := c.Cookie("pre_auth")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	userID, err := services.ParsePreAuthToken(preAuthToken)
	if err != nil {
		clearPreAuthCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "session expired, please log in again"})
		return
	}

	var req struct {
		TOTPCode   string `json:"totp_code"`
		BackupCode string `json:"backup_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.TOTPCode == "" && req.BackupCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "totp_code or backup_code required"})
		return
	}

	if err := services.VerifyTOTPForLogin(userID, req.TOTPCode, req.BackupCode); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid code"})
		return
	}

	clearPreAuthCookie(c)

	token, err := services.GenerateJWTForUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	setJWTCookie(c, token)
	if user, err := services.GetUser(userID); err == nil {
		notifyAccountEvent(services.AccountEventLogin, []string{user.Email}, services.AccountEventMeta{IP: c.ClientIP()})
	}
	c.JSON(http.StatusOK, gin.H{"message": "login successful"})
}

func SetupTOTP(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	user, err := services.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	otpauthURL, secret, err := services.GenerateTOTPSecret(userID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate 2fa secret"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"otpauth_url": otpauthURL,
		"secret":      secret,
	})
}

func EnableTOTPHandler(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	backupCodes, err := services.EnableTOTP(userID, req.Code)
	if err != nil {
		if err.Error() == "invalid code" {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid code"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	if user, err := services.GetUser(userID); err == nil {
		notifyAccountEvent(services.AccountEventTOTPEnabled, []string{user.Email}, services.AccountEventMeta{})
	}
	c.JSON(http.StatusOK, gin.H{
		"message":      "2fa enabled",
		"backup_codes": backupCodes,
	})
}

func DisableTOTPHandler(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		TOTPCode   string `json:"totp_code"`
		BackupCode string `json:"backup_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := services.DisableTOTP(userID, req.TOTPCode, req.BackupCode); err != nil {
		switch err.Error() {
		case "invalid code":
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case "totp_code or backup_code required":
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to disable 2fa"})
		}
		return
	}
	if user, err := services.GetUser(userID); err == nil {
		notifyAccountEvent(services.AccountEventTOTPDisabled, []string{user.Email}, services.AccountEventMeta{})
	}
	c.JSON(http.StatusOK, gin.H{"message": "2fa disabled"})
}

func RegenerateBackupCodesHandler(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	codes, err := services.RegenerateBackupCodes(userID, req.Code)
	if err != nil {
		if err.Error() == "invalid code" {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid code"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to regenerate backup codes"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":      "backup codes regenerated",
		"backup_codes": codes,
	})
}

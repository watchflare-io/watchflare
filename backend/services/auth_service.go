package services

import (
	"errors"
	"log/slog"
	"strings"
	"time"
	"watchflare/backend/config"
	"watchflare/backend/database"
	"watchflare/backend/models"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Register creates the first admin user. Returns an error if a user already exists.
func Register(email, password, username string) (*models.User, string, error) {
	var count int64
	if err := database.DB.Model(&models.User{}).Count(&count).Error; err != nil {
		return nil, "", err
	}
	if count > 0 {
		return nil, "", errors.New("registration is closed - admin user already exists")
	}

	// Derive username from email prefix if not provided.
	if username == "" {
		if idx := strings.Index(email, "@"); idx > 0 {
			username = email[:idx]
		}
	}

	user := &models.User{
		Email:    email,
		Username: username,
	}
	if err := user.HashPassword(password); err != nil {
		return nil, "", err
	}
	if err := database.DB.Create(user).Error; err != nil {
		return nil, "", err
	}

	token, err := generateJWT(user.ID)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

// ErrServiceUnavailable is returned when the database is unreachable during login.
var ErrServiceUnavailable = errors.New("service unavailable")

// Login authenticates a user and returns a JWT token.
func Login(email, password string) (string, error) {
	var user models.User
	if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errors.New("invalid credentials")
		}
		slog.Error("database error during login", "error", err)
		return "", ErrServiceUnavailable
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}

	return generateJWT(user.ID)
}

// ChangePassword updates a user's password after verifying the current one.
func ChangePassword(userID string, currentPassword, newPassword string) error {
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentPassword)); err != nil {
		return errors.New("current password is incorrect")
	}

	if err := user.HashPassword(newPassword); err != nil {
		return err
	}

	return database.DB.Save(&user).Error
}

// ChangeEmail updates a user's email address.
func ChangeEmail(userID, newEmail string) error {
	return database.DB.Model(&models.User{}).Where("id = ?", userID).Update("email", newEmail).Error
}

// ChangeUsername updates a user's username and returns the updated user.
func ChangeUsername(userID, username string) (*models.User, error) {
	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Update("username", username).Error; err != nil {
		return nil, err
	}
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// IsSetupRequired returns true if no users exist yet.
func IsSetupRequired() (bool, error) {
	var count int64
	if err := database.DB.Model(&models.User{}).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

// GetUser returns the user with the given ID.
func GetUser(userID string) (*models.User, error) {
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

// UpdatePreferences applies a partial update to the user's preferences and returns the updated user.
func UpdatePreferences(userID string, updates map[string]interface{}) (*models.User, error) {
	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetUser(userID)
}

func generateJWT(userID string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.JWTSecret))
}

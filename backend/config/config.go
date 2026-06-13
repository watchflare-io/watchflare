package config

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	GRPCPort    string
	DatabaseURL string
	JWTSecret         string
	SMTPEncryptionKey string
	CORSOrigins       []string
	Environment  string
	CookieDomain string // Domain for JWT cookie (empty = localhost, set in production)

	// Cookie security: nil means auto-detect per request (recommended).
	// Set via COOKIE_SECURE env var only to force-override auto-detection.
	CookieSecureOverride *bool

	// TrustedProxies is the list of IP addresses allowed to set X-Forwarded-Proto.
	// Defaults to loopback only. Add your reverse proxy IP if it runs on a separate host.
	TrustedProxies []string

	// TLS Configuration
	TLSMode   string // "auto" or "custom"
	TLSPKIDir string // For auto mode

	// Custom TLS (user-provided certificates)
	TLSCertFile string
	TLSKeyFile  string
	TLSCAFile   string

	// gRPC Security
	GRPCTimestampWindow int // HMAC is always required
}

var AppConfig *Config

// Load reads configuration from environment variables
func Load() {
	// Load .env file if it exists (ignore error in production)
	_ = godotenv.Load()

	AppConfig = &Config{
		GRPCPort: getEnv("GRPC_PORT", "50051"),
		DatabaseURL: fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			getEnv("POSTGRES_HOST", "localhost"),
			getEnv("POSTGRES_PORT", "5432"),
			getEnv("POSTGRES_USER", "watchflare"),
			getEnv("POSTGRES_PASSWORD", "watchflare_dev"),
			getEnv("POSTGRES_DB", "watchflare"),
			getEnv("POSTGRES_SSLMODE", "disable"),
		),
		JWTSecret:         getEnv("JWT_SECRET", ""),
		SMTPEncryptionKey: getEnv("SMTP_ENCRYPTION_KEY", ""),
		CORSOrigins:          parseOrigins(getEnv("CORS_ORIGINS", "http://localhost:5173")),
		Environment:          getEnv("ENV", "development"),
		CookieDomain:         getEnv("COOKIE_DOMAIN", ""),
		CookieSecureOverride: getOptionalBoolEnv("COOKIE_SECURE"),
		TrustedProxies:       parseProxies(getEnv("TRUSTED_PROXIES", "127.0.0.1,::1")),

		// TLS Configuration
		TLSMode:   getEnv("TLS_MODE", "auto"),
		TLSPKIDir: getEnv("TLS_PKI_DIR", "/var/lib/watchflare/pki"),

		// Custom TLS
		TLSCertFile: getEnv("TLS_CERT_FILE", ""),
		TLSKeyFile:  getEnv("TLS_KEY_FILE", ""),
		TLSCAFile:   getEnv("TLS_CA_FILE", ""),

		// gRPC Security (HMAC always required)
		GRPCTimestampWindow: getIntEnv("GRPC_TIMESTAMP_WINDOW", 300),
	}

	// Validate required fields
	if AppConfig.JWTSecret == "" {
		slog.Error("JWT_SECRET is required in environment variables")
		os.Exit(1)
	}

	// Validate JWT secret strength (minimum 32 characters for 256-bit security)
	if len(AppConfig.JWTSecret) < 32 {
		slog.Error("JWT_SECRET too short",
			"current_length", len(AppConfig.JWTSecret),
			"required", 32,
			"hint", "Generate a secure secret: openssl rand -base64 32",
		)
		os.Exit(1)
	}

	// Validate SMTP encryption key: required if SMTP is configured, warn otherwise
	if AppConfig.SMTPEncryptionKey == "" {
		slog.Warn("SMTP_ENCRYPTION_KEY is not set, SMTP password storage will be unavailable",
			"hint", "Generate a secure key: openssl rand -base64 32",
		)
	} else if len(AppConfig.SMTPEncryptionKey) < 32 {
		slog.Error("SMTP_ENCRYPTION_KEY too short",
			"current_length", len(AppConfig.SMTPEncryptionKey),
			"required", 32,
			"hint", "Generate a secure key: openssl rand -base64 32",
		)
		os.Exit(1)
	}

	// Warn if JWT secret looks weak (common patterns)
	weakSecrets := []string{"secret", "password", "admin", "test", "dev", "change", "please"}
	secretLower := strings.ToLower(AppConfig.JWTSecret)
	for _, weak := range weakSecrets {
		if strings.Contains(secretLower, weak) {
			slog.Warn("JWT_SECRET contains common word, use a cryptographically random string", "word", weak)
			break
		}
	}

	cookieSecureMode := "auto (HTTPS detected per request)"
	if AppConfig.CookieSecureOverride != nil {
		if *AppConfig.CookieSecureOverride {
			cookieSecureMode = "true (forced via COOKIE_SECURE)"
		} else {
			cookieSecureMode = "false (forced via COOKIE_SECURE)"
			slog.Warn("COOKIE_SECURE=false is set, cookies will never be marked Secure regardless of HTTPS")
		}
	}

	slog.Info("configuration loaded",
		"grpc_port", AppConfig.GRPCPort,
		"environment", AppConfig.Environment,
		"cookie_secure", cookieSecureMode,
		"trusted_proxies", AppConfig.TrustedProxies,
	)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseOrigins(originsStr string) []string {
	if originsStr == "" {
		return []string{}
	}
	origins := strings.Split(originsStr, ",")
	for i, origin := range origins {
		origins[i] = strings.TrimSpace(origin)
	}
	return origins
}

func getOptionalBoolEnv(key string) *bool {
	val := os.Getenv(key)
	if val == "" {
		return nil
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		slog.Warn("invalid boolean env var, ignoring", "key", key)
		return nil
	}
	return &b
}

func parseProxies(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// CookieSecure returns whether the Secure flag should be set on the JWT cookie
// for the current request.
//
// Priority:
//  1. COOKIE_SECURE env var (explicit override): forces true or false unconditionally
//  2. Direct TLS connection (Request.TLS != nil): always secure
//  3. X-Forwarded-Proto: https from a trusted proxy IP: secure behind reverse proxy
//  4. Default: false (plain HTTP)
func CookieSecure(tls bool, remoteAddr, xForwardedProto string) bool {
	if AppConfig.CookieSecureOverride != nil {
		return *AppConfig.CookieSecureOverride
	}
	if tls {
		return true
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	for _, trusted := range AppConfig.TrustedProxies {
		if host == trusted {
			return xForwardedProto == "https"
		}
	}
	return false
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		intVal, err := strconv.Atoi(value)
		if err != nil {
			slog.Warn("invalid integer env var, using default", "key", key, "default", defaultValue)
			return defaultValue
		}
		return intVal
	}
	return defaultValue
}

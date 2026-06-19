// Package testutil provides shared helpers for tests that need a real
// PostgreSQL/TimescaleDB connection. The test database is shared across
// packages, so `go test ./...` must run package-serially (`-p 1`) to avoid
// cross-package interference on shared tables.
package testutil

import (
	"os"
	"testing"

	"watchflare/backend/config"
	"watchflare/backend/database"
)

// DSN builds the connection string for the test database from the standard
// POSTGRES_* environment variables, falling back to local dev defaults.
func DSN() string {
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

// SetupTestDB sets a minimal app config and connects to the test database.
// The test is skipped (not failed) when the database is unreachable, so the
// suite stays green on machines without a local PostgreSQL.
func SetupTestDB(t *testing.T) {
	t.Helper()
	config.AppConfig = &config.Config{
		JWTSecret: "test-secret-key-must-be-32-chars!!",
	}
	if err := database.Connect(DSN()); err != nil {
		t.Skipf("skipping test: database unavailable: %v", err)
	}
}

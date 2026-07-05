package grpc

import (
	"testing"

	"watchflare/backend/database"
)

func TestContainerStatesTableExists(t *testing.T) {
	setupGRPCTestDB(t)
	if !database.DB.Migrator().HasTable("container_states") {
		t.Fatal("container_states table was not created by migration")
	}
}

package handlers

import (
	"testing"

	"watchflare/backend/database"
	"watchflare/backend/models"
)

func TestSmtpSettingsHasCategoriesColumn(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()
	if !database.DB.Migrator().HasColumn(&models.SmtpSettings{}, "categories") {
		t.Fatal("smtp_settings.categories column was not created by migration")
	}
}

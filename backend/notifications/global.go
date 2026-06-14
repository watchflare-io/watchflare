package notifications

import "gorm.io/gorm"

// Default is the application-wide notifications service.
// Initialized by Init at startup; nil before Init is called.
var Default *Service

// Init wires the default service with the given database and encryption key.
// Called once from main.go after the database is connected and configuration is loaded.
func Init(db *gorm.DB, encryptionKey string) {
	Default = NewService(NewRepository(db), NewShoutrrrNotifier(), encryptionKey)
}

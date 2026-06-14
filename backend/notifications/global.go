package notifications

import "gorm.io/gorm"

// Default is the application-wide notifications service. Nil until Init runs.
var Default *Service

// Init wires the default service with the given database and encryption key.
func Init(db *gorm.DB, encryptionKey string) {
	Default = NewService(NewRepository(db), NewShoutrrrNotifier(), encryptionKey)
}

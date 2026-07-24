package models

import "time"

// ChangeType constants for PackageHistory.
const (
	ChangeTypeInitial = "initial"
	ChangeTypeAdded   = "added"
	ChangeTypeRemoved = "removed"
	ChangeTypeUpdated = "updated"
)

// CollectionType constants for PackageCollection.
const (
	CollectionTypeFull    = "full"
	CollectionTypeDelta   = "delta"
	CollectionTypeInitial = "initial"
)

// PackageCollectionStatus constants for PackageCollection.
const (
	PackageCollectionStatusSuccess = "success"
	PackageCollectionStatusFailed  = "failed"
	PackageCollectionStatusPartial = "partial"
)

// CheckablePackageManagers is the static list of package managers that have
// update checkers on the agent side. After any inventory, the Hub bulk-marks
// all packages belonging to these managers as update_checked = true.
var CheckablePackageManagers = []string{
	"dpkg",         // apt (Debian/Ubuntu)
	"rpm",          // dnf/yum (Fedora/RHEL/CentOS)
	"apk",          // apk (Alpine)
	"pacman",       // pacman/checkupdates (Arch)
	"brew-formula", // brew formulae (macOS)
	"brew-cask",    // brew casks (macOS)
	"npm",          // npm global packages
	"pip",          // pip (Python)
	"gem",          // gem (Ruby)
	"composer",     // composer (PHP)
	"pnpm-global",  // pnpm global packages
}

// Package represents current package state on a host
type Package struct {
	ID                int64      `gorm:"primaryKey" json:"id"`
	HostID            string     `gorm:"type:char(36);not null;index:idx_packages_host_id" json:"host_id"`
	Name              string     `gorm:"type:varchar(255);not null" json:"name"`
	Version           string     `gorm:"type:varchar(100);not null" json:"version"`
	Architecture      string     `gorm:"type:varchar(50)" json:"architecture"`
	PackageManager    string     `gorm:"type:varchar(20);not null" json:"package_manager"`
	Source            string     `gorm:"type:varchar(255)" json:"source"`
	InstalledAt       *time.Time `json:"installed_at"`
	PackageSize       int64      `json:"package_size"`
	Description       string     `gorm:"type:varchar(100)" json:"description"`
	AvailableVersion  string     `gorm:"type:varchar(100)" json:"available_version"` // Empty if up to date
	HasSecurityUpdate bool       `gorm:"not null;default:false" json:"has_security_update"`
	UpdateChecked     bool       `gorm:"not null;default:false" json:"update_checked"` // True if an update checker covers this package manager
	FirstSeen         time.Time  `gorm:"not null;default:now()" json:"first_seen"`
	LastSeen          time.Time  `gorm:"not null;default:now()" json:"last_seen"`
}

// PackageHistory stores temporal snapshots of packages (TimescaleDB hypertable)
type PackageHistory struct {
	ID             int64     `gorm:"primaryKey" json:"id"`
	Timestamp      time.Time `gorm:"primaryKey;not null" json:"timestamp"`
	HostID         string    `gorm:"type:char(36);not null;index:idx_package_history_host_id" json:"host_id"`
	Name           string    `gorm:"type:varchar(255);not null" json:"name"`
	Version        string    `gorm:"type:varchar(100);not null" json:"version"`
	Architecture   string    `gorm:"type:varchar(50)" json:"architecture"`
	PackageManager string    `gorm:"type:varchar(20);not null" json:"package_manager"`
	Source         string    `gorm:"type:varchar(255)" json:"source"`
	PackageSize    int64     `json:"package_size"`
	Description    string    `gorm:"type:varchar(100)" json:"description"`
	ChangeType     string    `gorm:"type:varchar(20);not null" json:"change_type"` // 'added', 'removed', 'updated', 'initial'
}

// PackageCollection tracks metadata about package collection jobs
type PackageCollection struct {
	ID             int64     `gorm:"primaryKey" json:"id"`
	HostID         string    `gorm:"type:char(36);not null;index:idx_package_collections_host_id" json:"host_id"`
	Timestamp      time.Time `gorm:"not null;default:now()" json:"timestamp"`
	CollectionType string    `gorm:"type:varchar(20);not null" json:"collection_type"` // 'full', 'delta', 'initial'
	PackageCount   int       `gorm:"not null" json:"package_count"`
	ChangesCount   int       `gorm:"default:0" json:"changes_count"`
	DurationMs     int       `json:"duration_ms"`
	Status         string    `gorm:"type:varchar(20);not null;default:'success'" json:"status"` // 'success', 'failed', 'partial'
	ErrorMessage   string    `gorm:"type:text" json:"error_message"`
}

// TableName overrides the default "package_histories" pluralization
func (PackageHistory) TableName() string {
	return "package_history"
}

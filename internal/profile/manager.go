package profile

import (
	"github.com/metaviz/internal/storage"
)

// Manager is a minimal stub — uploaded config files are managed
// directly via the filesystem in api/server.go.
// This package exists to satisfy the import in core/manager.go.
type Manager struct {
	db *storage.DB
}

func NewManager(db *storage.DB) *Manager { return &Manager{db: db} }

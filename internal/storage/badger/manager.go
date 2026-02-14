package badger

import (
	"log/slog"

	"github.com/bobmcallan/vire-portal/internal/config"
	"github.com/bobmcallan/vire-portal/internal/interfaces"
)

// Manager implements the StorageManager interface for Badger.
type Manager struct {
	db     *BadgerDB
	kv     interfaces.KeyValueStorage
	logger *slog.Logger
}

// NewManager creates a new Badger storage manager.
func NewManager(logger *slog.Logger, cfg *config.BadgerConfig) (interfaces.StorageManager, error) {
	db, err := NewBadgerDB(logger, cfg)
	if err != nil {
		return nil, err
	}

	manager := &Manager{
		db:     db,
		kv:     NewKVStorage(db, logger),
		logger: logger,
	}

	logger.Debug("Badger storage manager initialized")

	return manager, nil
}

// KeyValueStorage returns the KeyValue storage interface.
func (m *Manager) KeyValueStorage() interfaces.KeyValueStorage {
	return m.kv
}

// DB returns the underlying database connection.
func (m *Manager) DB() interface{} {
	if m.db != nil {
		return m.db.Store()
	}
	return nil
}

// Close closes the database connection.
func (m *Manager) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

package badger

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
	"github.com/timshannon/badgerhold/v4"
)

// BadgerDB manages the Badger database connection.
type BadgerDB struct {
	store  *badgerhold.Store
	logger *common.Logger
	config *config.BadgerConfig
}

// NewBadgerDB creates a new Badger database connection.
func NewBadgerDB(logger *common.Logger, cfg *config.BadgerConfig) (*BadgerDB, error) {
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	logger.Debug().Str("path", cfg.Path).Msg("opening Badger database")

	options := badgerhold.DefaultOptions
	options.Dir = cfg.Path
	options.ValueDir = cfg.Path
	options.Logger = nil // Disable default badger logger

	store, err := badgerhold.Open(options)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger database: %w", err)
	}

	logger.Debug().Str("path", cfg.Path).Msg("Badger database initialized")

	return &BadgerDB{
		store:  store,
		logger: logger,
		config: cfg,
	}, nil
}

// Store returns the underlying badgerhold store.
func (b *BadgerDB) Store() *badgerhold.Store {
	return b.store
}

// Close closes the database connection.
func (b *BadgerDB) Close() error {
	if b.store != nil {
		return b.store.Close()
	}
	return nil
}

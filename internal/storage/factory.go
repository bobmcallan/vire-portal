package storage

import (
	"log/slog"

	"github.com/bobmcallan/vire-portal/internal/config"
	"github.com/bobmcallan/vire-portal/internal/interfaces"
	"github.com/bobmcallan/vire-portal/internal/storage/badger"
)

// NewStorageManager creates a new storage manager based on config.
func NewStorageManager(logger *slog.Logger, cfg *config.Config) (interfaces.StorageManager, error) {
	return badger.NewManager(logger, &cfg.Storage.Badger)
}

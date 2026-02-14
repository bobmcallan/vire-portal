package storage

import (
	"github.com/bobmcallan/vire-portal/internal/config"
	"github.com/bobmcallan/vire-portal/internal/interfaces"
	"github.com/bobmcallan/vire-portal/internal/storage/badger"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// NewStorageManager creates a new storage manager based on config.
func NewStorageManager(logger *common.Logger, cfg *config.Config) (interfaces.StorageManager, error) {
	return badger.NewManager(logger, &cfg.Storage.Badger)
}

package badger

import (
	"context"
	"fmt"

	common "github.com/bobmcallan/vire-portal/internal/vire/common"
	"github.com/timshannon/badgerhold/v4"
)

// KVEntry represents a key-value pair stored in BadgerDB.
type KVEntry struct {
	Key   string `badgerhold:"key"`
	Value string
}

// KVStorage implements interfaces.KeyValueStorage using BadgerDB.
type KVStorage struct {
	db     *BadgerDB
	logger *common.Logger
}

// NewKVStorage creates a new key-value storage backed by BadgerDB.
func NewKVStorage(db *BadgerDB, logger *common.Logger) *KVStorage {
	return &KVStorage{
		db:     db,
		logger: logger,
	}
}

// Get retrieves a value by key.
func (s *KVStorage) Get(_ context.Context, key string) (string, error) {
	var entry KVEntry
	err := s.db.Store().Get(key, &entry)
	if err != nil {
		if err == badgerhold.ErrNotFound {
			return "", fmt.Errorf("key not found: %s", key)
		}
		return "", fmt.Errorf("failed to get key %s: %w", key, err)
	}
	return entry.Value, nil
}

// Set stores a key-value pair.
func (s *KVStorage) Set(_ context.Context, key, value string) error {
	entry := KVEntry{
		Key:   key,
		Value: value,
	}
	err := s.db.Store().Upsert(key, &entry)
	if err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}
	return nil
}

// Delete removes a key-value pair.
func (s *KVStorage) Delete(_ context.Context, key string) error {
	err := s.db.Store().Delete(key, KVEntry{})
	if err != nil {
		if err == badgerhold.ErrNotFound {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete key %s: %w", key, err)
	}
	return nil
}

// GetAll retrieves all key-value pairs.
func (s *KVStorage) GetAll(_ context.Context) (map[string]string, error) {
	var entries []KVEntry
	err := s.db.Store().Find(&entries, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get all keys: %w", err)
	}

	result := make(map[string]string, len(entries))
	for _, entry := range entries {
		result[entry.Key] = entry.Value
	}
	return result, nil
}

package interfaces

import "context"

// StorageManager provides access to domain-specific storage interfaces.
// Implementations can be swapped (BadgerDB now, centralised DB later).
type StorageManager interface {
	KeyValueStorage() KeyValueStorage
	DB() interface{}
	Close() error
}

// KeyValueStorage provides basic key-value operations.
type KeyValueStorage interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
	GetAll(ctx context.Context) (map[string]string, error)
}

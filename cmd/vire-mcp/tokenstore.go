package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/mark3labs/mcp-go/client/transport"
)

// FileTokenStore persists OAuth tokens to a JSON file.
// It implements transport.TokenStore for use with mcp-go's OAuth support.
type FileTokenStore struct {
	path string
	mu   sync.RWMutex
}

// NewFileTokenStore creates a token store that persists to the given path.
// The directory is created automatically on first write.
func NewFileTokenStore(path string) *FileTokenStore {
	return &FileTokenStore{path: path}
}

// GetToken reads the stored token from disk.
// Returns transport.ErrNoToken if the file is missing or corrupt.
func (s *FileTokenStore) GetToken(_ context.Context) (*transport.Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, transport.ErrNoToken
		}
		return nil, err
	}

	var token transport.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, transport.ErrNoToken // corrupt file, treat as absent
	}
	return &token, nil
}

// SaveToken writes the token to disk with 0600 permissions.
func (s *FileTokenStore) SaveToken(_ context.Context, token *transport.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}

	data, err := json.Marshal(token)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

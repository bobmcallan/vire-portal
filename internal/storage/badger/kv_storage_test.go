package badger

import (
	"context"
	"testing"

	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

func setupTestDB(t *testing.T) (*BadgerDB, func()) {
	t.Helper()

	dir := t.TempDir()
	logger := common.NewSilentLogger()

	cfg := &config.BadgerConfig{Path: dir}
	db, err := NewBadgerDB(logger, cfg)
	if err != nil {
		t.Fatalf("failed to create test DB: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

func TestKVStorage_SetAndGet(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := common.NewSilentLogger()
	kv := NewKVStorage(db, logger)
	ctx := context.Background()

	if err := kv.Set(ctx, "test-key", "test-value"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := kv.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "test-value" {
		t.Errorf("expected test-value, got %s", val)
	}
}

func TestKVStorage_GetNotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := common.NewSilentLogger()
	kv := NewKVStorage(db, logger)
	ctx := context.Background()

	_, err := kv.Get(ctx, "nonexistent-key")
	if err == nil {
		t.Error("expected error for nonexistent key, got nil")
	}
}

func TestKVStorage_Upsert(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := common.NewSilentLogger()
	kv := NewKVStorage(db, logger)
	ctx := context.Background()

	if err := kv.Set(ctx, "key", "value1"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Overwrite
	if err := kv.Set(ctx, "key", "value2"); err != nil {
		t.Fatalf("Set (upsert) failed: %v", err)
	}

	val, err := kv.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "value2" {
		t.Errorf("expected value2, got %s", val)
	}
}

func TestKVStorage_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := common.NewSilentLogger()
	kv := NewKVStorage(db, logger)
	ctx := context.Background()

	if err := kv.Set(ctx, "key", "value"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if err := kv.Delete(ctx, "key"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := kv.Get(ctx, "key")
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestKVStorage_DeleteNonexistent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := common.NewSilentLogger()
	kv := NewKVStorage(db, logger)
	ctx := context.Background()

	// Deleting a nonexistent key should not error
	if err := kv.Delete(ctx, "nonexistent"); err != nil {
		t.Errorf("Delete nonexistent key should not error: %v", err)
	}
}

func TestKVStorage_GetAll(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := common.NewSilentLogger()
	kv := NewKVStorage(db, logger)
	ctx := context.Background()

	kv.Set(ctx, "key1", "val1")
	kv.Set(ctx, "key2", "val2")
	kv.Set(ctx, "key3", "val3")

	all, err := kv.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}

	if len(all) != 3 {
		t.Errorf("expected 3 entries, got %d", len(all))
	}
	if all["key1"] != "val1" {
		t.Errorf("expected key1=val1, got key1=%s", all["key1"])
	}
	if all["key2"] != "val2" {
		t.Errorf("expected key2=val2, got key2=%s", all["key2"])
	}
	if all["key3"] != "val3" {
		t.Errorf("expected key3=val3, got key3=%s", all["key3"])
	}
}

func TestKVStorage_GetAllEmpty(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := common.NewSilentLogger()
	kv := NewKVStorage(db, logger)
	ctx := context.Background()

	all, err := kv.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll on empty store failed: %v", err)
	}

	if len(all) != 0 {
		t.Errorf("expected 0 entries, got %d", len(all))
	}
}

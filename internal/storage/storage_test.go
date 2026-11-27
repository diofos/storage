package storage

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestChunkStore(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "meshstore_storage_test")
	defer os.RemoveAll(tempDir)

	maxBytes := int64(1024 * 1024) // 1 MB
	store, err := NewChunkStore(tempDir, maxBytes)
	if err != nil {
		t.Fatalf("failed to create chunk store: %v", err)
	}

	data := []byte("hello world chunk data")
	chunkID := "testchunk123"

	n, err := store.Write(chunkID, int64(len(data)), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to write chunk: %v", err)
	}
	if n != int64(len(data)) {
		t.Fatalf("expected written %d, got %d", len(data), n)
	}

	if !store.Has(chunkID) {
		t.Fatalf("expected store to have chunk")
	}

	rc, size, err := store.OpenChunk(chunkID)
	if err != nil {
		t.Fatalf("failed to open chunk: %v", err)
	}
	defer rc.Close()

	if size != int64(len(data)) {
		t.Fatalf("expected size %d, got %d", len(data), size)
	}

	readBytes, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("failed to read chunk: %v", err)
	}
	if string(readBytes) != string(data) {
		t.Fatalf("content mismatch: got %q, expected %q", string(readBytes), string(data))
	}

	used, max, avail := store.Stats()
	if used != int64(len(data)) {
		t.Errorf("expected used %d, got %d", len(data), used)
	}
	if max != maxBytes {
		t.Errorf("expected max %d, got %d", maxBytes, max)
	}
	if avail != maxBytes-int64(len(data)) {
		t.Errorf("expected avail %d, got %d", maxBytes-int64(len(data)), avail)
	}

	// Test Quota Exceeded
	store.SetMaxStorageBytes(10)
	_, err = store.Write("largechunk", 100, bytes.NewReader(make([]byte, 100)))
	if err == nil {
		t.Fatalf("expected quota exceeded error, got nil")
	}

	// Test Delete
	if err := store.Delete(chunkID); err != nil {
		t.Fatalf("failed to delete chunk: %v", err)
	}
	if store.Has(chunkID) {
		t.Fatalf("expected chunk to be deleted")
	}
}

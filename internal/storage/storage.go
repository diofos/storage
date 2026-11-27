package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

var (
	ErrQuotaExceeded = errors.New("storage quota exceeded")
	ErrChunkNotFound = errors.New("chunk not found")
)

// ChunkStore manages stored binary chunks on local disk with quota tracking.
type ChunkStore struct {
	mu            sync.RWMutex
	baseDir       string
	chunksDir     string
	maxBytes      int64
	usedBytes     int64
}

// NewChunkStore initializes a chunk store under the given base directory.
func NewChunkStore(baseDir string, maxBytes int64) (*ChunkStore, error) {
	chunksDir := filepath.Join(baseDir, "chunks")
	if err := os.MkdirAll(chunksDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create chunks directory: %w", err)
	}

	cs := &ChunkStore{
		baseDir:   baseDir,
		chunksDir: chunksDir,
		maxBytes:  maxBytes,
	}

	// Calculate initial disk usage by scanning existing chunks
	var totalUsed int64
	entries, err := os.ReadDir(chunksDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				info, err := entry.Info()
				if err == nil {
					totalUsed += info.Size()
				}
			}
		}
	}
	cs.usedBytes = totalUsed

	return cs, nil
}

// chunkPath returns the full filesystem path for a chunk ID.
func (cs *ChunkStore) chunkPath(chunkID string) string {
	return filepath.Join(cs.chunksDir, chunkID)
}

// Has checks if a chunk exists locally.
func (cs *ChunkStore) Has(chunkID string) bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	_, err := os.Stat(cs.chunkPath(chunkID))
	return err == nil
}

// Write streams chunk data from r and persists it to disk.
func (cs *ChunkStore) Write(chunkID string, sizeBytes int64, r io.Reader) (int64, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	targetPath := cs.chunkPath(chunkID)

	var existingSize int64
	if stat, err := os.Stat(targetPath); err == nil {
		existingSize = stat.Size()
	}

	delta := sizeBytes - existingSize
	if delta > 0 && cs.maxBytes > 0 && (cs.usedBytes+delta) > cs.maxBytes {
		return 0, ErrQuotaExceeded
	}

	tmpFile, err := os.CreateTemp(cs.chunksDir, "chunk_tmp_*")
	if err != nil {
		return 0, fmt.Errorf("failed to create temp chunk file: %w", err)
	}
	tmpName := tmpFile.Name()

	n, err := io.Copy(tmpFile, r)
	tmpFile.Close()
	if err != nil {
		_ = os.Remove(tmpName)
		return 0, fmt.Errorf("failed writing chunk content: %w", err)
	}

	if err := os.Rename(tmpName, targetPath); err != nil {
		_ = os.Remove(tmpName)
		return 0, fmt.Errorf("failed to commit chunk file: %w", err)
	}

	cs.usedBytes += (n - existingSize)
	if cs.usedBytes < 0 {
		cs.usedBytes = 0
	}

	return n, nil
}

// OpenChunk opens a chunk for streaming reading and returns its size.
func (cs *ChunkStore) OpenChunk(chunkID string) (io.ReadCloser, int64, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	path := cs.chunkPath(chunkID)
	stat, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, ErrChunkNotFound
		}
		return nil, 0, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}

	return f, stat.Size(), nil
}

// Delete removes a chunk from disk and updates usage metrics.
func (cs *ChunkStore) Delete(chunkID string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	path := cs.chunkPath(chunkID)
	stat, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	if err := os.Remove(path); err != nil {
		return err
	}

	cs.usedBytes -= stat.Size()
	if cs.usedBytes < 0 {
		cs.usedBytes = 0
	}
	return nil
}

// Stats returns (usedBytes, maxBytes, availableBytes).
func (cs *ChunkStore) Stats() (int64, int64, int64) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	used := cs.usedBytes
	max := cs.maxBytes
	var avail int64
	if max > 0 {
		if max > used {
			avail = max - used
		} else {
			avail = 0
		}
	} else {
		avail = 1024 * 1024 * 1024 * 1024 // 1 TB fallback if unlimited
	}

	return used, max, avail
}

// SetMaxStorageBytes updates the maximum storage quota in bytes.
func (cs *ChunkStore) SetMaxStorageBytes(maxBytes int64) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.maxBytes = maxBytes
}

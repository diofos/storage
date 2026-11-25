package metadata

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthdm/foreverstore/internal/chunking"
)

func TestMetadataIndex(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "meshstore_metadata_test")
	defer os.RemoveAll(tempDir)

	idx, err := NewIndex(tempDir)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	// 1. Add nodes
	idx.UpsertNode(NodeRecord{
		NodeID:        "node-1",
		DisplayName:   "Primary",
		Address:       "127.0.0.1:7000",
		Status:        NodeOnline,
		CapacityBytes: 1000000,
		UsedBytes:     200000,
	})

	idx.UpsertNode(NodeRecord{
		NodeID:        "node-2",
		DisplayName:   "Secondary",
		Address:       "127.0.0.1:7001",
		Status:        NodeOnline,
		CapacityBytes: 2000000,
		UsedBytes:     100000, // Has more available space (1.9MB)
	})

	// 2. Test Placement: should pick node-2 first because it has 1.9MB available vs node-1's 800KB
	nodes := idx.SelectPlacementNodes(2, "node-1")
	if len(nodes) != 2 {
		t.Fatalf("expected 2 placement nodes, got %d", len(nodes))
	}
	if nodes[0] != "node-2" {
		t.Errorf("expected node-2 with higher capacity to be chosen first, got %s", nodes[0])
	}

	// 3. Commit file
	manifest := &chunking.Manifest{
		FileID:      "file-123",
		FileName:    "test.mp4",
		SizeBytes:   50000,
		ContentHash: "hash-full-file",
		Chunks: []chunking.ChunkInfo{
			{Index: 0, ChunkID: "chunk-a", SizeBytes: 25000},
			{Index: 1, ChunkID: "chunk-b", SizeBytes: 25000},
		},
	}
	idx.CommitFile(manifest, "/videos/test.mp4")
	idx.RecordChunkLocation("chunk-a", "node-1")
	idx.RecordChunkLocation("chunk-a", "node-2")

	// 4. Test Save and Reload
	if err := idx.Save(); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	idxReloaded, err := NewIndex(tempDir)
	if err != nil {
		t.Fatalf("failed to reload index: %v", err)
	}

	f, err := idxReloaded.GetFile("file-123")
	if err != nil {
		t.Fatalf("failed to get reloaded file: %v", err)
	}
	if f.FileName != "test.mp4" {
		t.Errorf("file name mismatch: %s", f.FileName)
	}

	locs := idxReloaded.GetChunkLocations("chunk-a")
	if len(locs) != 2 {
		t.Errorf("expected 2 locations for chunk-a, got %d", len(locs))
	}

	// 5. Node stale check
	idx.MarkStaleNodes(1 * time.Nanosecond)
	nodeRecords := idx.GetNodes()
	for _, nr := range nodeRecords {
		if nr.Status != NodeOffline {
			t.Errorf("node %s should have been marked offline", nr.NodeID)
		}
	}
}

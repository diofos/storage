package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIdentityPersistence(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "meshstore_identity_test")
	defer os.RemoveAll(tempDir)

	id1, err := LoadOrCreateIdentity(tempDir, "TestNode")
	if err != nil {
		t.Fatalf("failed to create identity: %v", err)
	}

	if len(id1.NodeID) != 64 {
		t.Errorf("expected 64 char hex node id, got %d", len(id1.NodeID))
	}
	if id1.DisplayName != "TestNode" {
		t.Errorf("expected TestNode display name, got %s", id1.DisplayName)
	}

	// Load again from same dir to verify persistence
	id2, err := LoadOrCreateIdentity(tempDir, "TestNode")
	if err != nil {
		t.Fatalf("failed to reload identity: %v", err)
	}

	if id1.NodeID != id2.NodeID {
		t.Errorf("identity changed across reload! have %s, got %s", id1.NodeID, id2.NodeID)
	}
}

package identity

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// NodeIdentity represents the persistent cryptographic identity of a node.
type NodeIdentity struct {
	NodeID      string    `json:"node_id"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

const identityFileName = "identity.json"

// LoadOrCreateIdentity loads existing identity from data directory or creates and persists a new one.
func LoadOrCreateIdentity(dataDir string, displayName string) (*NodeIdentity, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	identityPath := filepath.Join(dataDir, identityFileName)

	if data, err := os.ReadFile(identityPath); err == nil {
		id := &NodeIdentity{}
		if err := json.Unmarshal(data, id); err == nil && len(id.NodeID) > 0 {
			if len(displayName) > 0 && id.DisplayName != displayName {
				id.DisplayName = displayName
				_ = saveIdentity(identityPath, id)
			}
			return id, nil
		}
	}

	// Generate new random 32-byte identity
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("failed to generate random identity: %w", err)
	}

	if len(displayName) == 0 {
		hostname, _ := os.Hostname()
		if len(hostname) > 0 {
			displayName = hostname
		} else {
			displayName = "MeshNode-" + hex.EncodeToString(buf[:4])
		}
	}

	id := &NodeIdentity{
		NodeID:      hex.EncodeToString(buf),
		DisplayName: displayName,
		CreatedAt:   time.Now().UTC(),
	}

	if err := saveIdentity(identityPath, id); err != nil {
		return nil, err
	}

	return id, nil
}

func saveIdentity(path string, id *NodeIdentity) error {
	data, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

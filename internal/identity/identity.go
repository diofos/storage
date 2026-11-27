package identity

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NodeIdentity represents the persistent cryptographic identity of a node.
type NodeIdentity struct {
	NodeID      string    `json:"node_id"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

const identityFileName = "identity.json"

var adjectives = []string{
	"Cosmic", "Solar", "Quantum", "Apex", "Atlas", "Nova", "Zenith", "Polaris",
	"Stellar", "Titan", "Falcon", "Swift", "Nexus", "Orion", "Aero", "Pulse",
	"Radiant", "Vanguard", "Echo", "Vector", "Hyper", "Silver", "Cyber", "Amber",
	"Crystal", "Emerald", "Sapphire", "Iron", "Beacon", "Breeze", "Shadow", "Alpine",
}

var nouns = []string{
	"Vault", "Node", "Drive", "Core", "Mesh", "Station", "Array", "Relay",
	"Pod", "Haven", "Silo", "Cell", "Dock", "Tower", "Anchor", "Matrix",
	"Sanctuary", "Depot", "Beacon", "Fortress",
}

// GenerateSensibleName generates a friendly, memorable node name like "Cosmic-Vault-481".
func GenerateSensibleName() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("MeshNode-%d", time.Now().UnixNano()%10000)
	}
	adjIdx := int(b[0]) % len(adjectives)
	nounIdx := int(b[1]) % len(nouns)
	num := (int(b[2])<<8 | int(b[3])) % 900 + 100 // 100-999

	return fmt.Sprintf("%s-%s-%d", adjectives[adjIdx], nouns[nounIdx], num)
}

// LoadOrCreateIdentity loads existing identity from data directory or creates and persists a new one.
func LoadOrCreateIdentity(dataDir string, displayName string) (*NodeIdentity, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	identityPath := filepath.Join(dataDir, identityFileName)

	if data, err := os.ReadFile(identityPath); err == nil {
		id := &NodeIdentity{}
		if err := json.Unmarshal(data, id); err == nil && len(id.NodeID) > 0 {
			// If existing name is empty or a generic default, upgrade to sensible name or custom name
			if len(displayName) > 0 && displayName != id.DisplayName && !strings.HasPrefix(displayName, "Node-") {
				id.DisplayName = displayName
				_ = saveIdentity(identityPath, id)
			} else if strings.HasPrefix(id.DisplayName, "Node-") {
				id.DisplayName = GenerateSensibleName()
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

	if len(displayName) == 0 || strings.HasPrefix(displayName, "Node-") {
		displayName = GenerateSensibleName()
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

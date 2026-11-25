package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config represents the complete configuration for a MeshStore node.
type Config struct {
	Node    NodeConfig    `json:"node"`
	Mesh    MeshConfig    `json:"mesh"`
	Storage StorageConfig `json:"storage"`
	API     APIConfig     `json:"api"`
}

type NodeConfig struct {
	ListenAddress string `json:"listen_address"`
	DataDirectory string `json:"data_directory"`
	MaxStorageGB  int64  `json:"max_storage_gb"`
	DisplayName   string `json:"display_name"`
}

type MeshConfig struct {
	NetworkID string   `json:"network_id"`
	Peers     []string `json:"peers"`
}

type StorageConfig struct {
	ChunkSizeMB       int `json:"chunk_size_mb"`
	ReplicationFactor int `json:"replication_factor"`
}

type APIConfig struct {
	Enabled       bool   `json:"enabled"`
	ListenAddress string `json:"listen_address"`
	AuthToken     string `json:"auth_token,omitempty"`
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig(listenPort string) *Config {
	return &Config{
		Node: NodeConfig{
			ListenAddress: "127.0.0.1:" + listenPort,
			DataDirectory: "./meshstore_data_" + listenPort,
			MaxStorageGB:  10,
			DisplayName:   "Node-" + listenPort,
		},
		Mesh: MeshConfig{
			NetworkID: "default-mesh",
			Peers:     []string{},
		},
		Storage: StorageConfig{
			ChunkSizeMB:       1, // 1MB default chunk size for fast testing & transfers
			ReplicationFactor: 2,
		},
		API: APIConfig{
			Enabled:       true,
			ListenAddress: "127.0.0.1:80" + listenPort,
		},
	}
}

// LoadConfig loads configuration from a JSON file. If file doesn't exist, returns default.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// SaveConfig saves configuration to a JSON file.
func (c *Config) SaveConfig(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

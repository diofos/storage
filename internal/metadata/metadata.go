package metadata

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/anthdm/foreverstore/internal/chunking"
)

var (
	ErrFileNotFound = errors.New("file not found in mesh metadata")
)

type NodeStatus string

const (
	NodeOnline  NodeStatus = "online"
	NodeOffline NodeStatus = "offline"
)

// NodeRecord tracks an authorized node in the mesh.
type NodeRecord struct {
	NodeID        string     `json:"node_id"`
	DisplayName   string     `json:"display_name"`
	Address       string     `json:"address"`
	LastSeen      time.Time  `json:"last_seen"`
	Status        NodeStatus `json:"status"`
	CapacityBytes int64      `json:"capacity_bytes"`
	UsedBytes     int64      `json:"used_bytes"`
}

type FileStatus string

const (
	FileAvailable FileStatus = "available"
	FileUploading FileStatus = "uploading"
	FileDeleted   FileStatus = "deleted"
)

// FileRecord represents a logical file stored in the mesh.
type FileRecord struct {
	FileID      string               `json:"file_id"`
	LogicalPath string               `json:"logical_path"`
	FileName    string               `json:"file_name"`
	SizeBytes   int64                `json:"size_bytes"`
	ContentHash string               `json:"content_hash"`
	Status      FileStatus           `json:"status"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
	Chunks      []chunking.ChunkInfo `json:"chunks"`
}

// ChunkLocation tracks which node holds a replica of a chunk.
type ChunkLocation struct {
	ChunkID        string    `json:"chunk_id"`
	NodeID         string    `json:"node_id"`
	HealthStatus   string    `json:"health_status"` // "healthy", "corrupted"
	LastVerifiedAt time.Time `json:"last_verified_at"`
}

type TransferState string

const (
	TransferActive    TransferState = "active"
	TransferCompleted TransferState = "completed"
	TransferFailed    TransferState = "failed"
	TransferPaused    TransferState = "paused"
)

// TransferRecord records an active or historic transfer.
type TransferRecord struct {
	TransferID       string        `json:"transfer_id"`
	FileID           string        `json:"file_id"`
	FileName         string        `json:"file_name"`
	Type             string        `json:"type"` // "upload", "download"
	BytesTransferred int64         `json:"bytes_transferred"`
	TotalBytes       int64         `json:"total_bytes"`
	State            TransferState `json:"state"`
	SpeedBytesSec    int64         `json:"speed_bytes_sec"`
	ErrorMsg         string        `json:"error_msg,omitempty"`
	StartedAt        time.Time     `json:"started_at"`
	CompletedAt      time.Time     `json:"completed_at,omitempty"`
}

type metadataState struct {
	Nodes     map[string]*NodeRecord    `json:"nodes"`
	Files     map[string]*FileRecord    `json:"files"`
	Locations map[string][]ChunkLocation `json:"locations"` // chunkID -> locations
	Transfers map[string]*TransferRecord `json:"transfers"`
}

// Index manages persistent metadata for files, nodes, and placements.
type Index struct {
	filePath  string
	mu        sync.RWMutex
	nodes     map[string]*NodeRecord
	files     map[string]*FileRecord
	locations map[string][]ChunkLocation
	transfers map[string]*TransferRecord
}

// NewIndex loads metadata from disk or creates a new empty metadata index.
func NewIndex(dataDir string) (*Index, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	metaFile := filepath.Join(dataDir, "metadata.json")
	idx := &Index{
		filePath:  metaFile,
		nodes:     make(map[string]*NodeRecord),
		files:     make(map[string]*FileRecord),
		locations: make(map[string][]ChunkLocation),
		transfers: make(map[string]*TransferRecord),
	}

	if data, err := os.ReadFile(metaFile); err == nil {
		var state metadataState
		if err := json.Unmarshal(data, &state); err == nil {
			if state.Nodes != nil {
				idx.nodes = state.Nodes
			}
			if state.Files != nil {
				idx.files = state.Files
			}
			if state.Locations != nil {
				idx.locations = state.Locations
			}
			if state.Transfers != nil {
				idx.transfers = state.Transfers
			}
		}
	}

	return idx, nil
}

// Save persists the metadata index to disk atomically.
func (idx *Index) Save() error {
	idx.mu.RLock()
	state := metadataState{
		Nodes:     idx.nodes,
		Files:     idx.files,
		Locations: idx.locations,
		Transfers: idx.transfers,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	idx.mu.RUnlock()

	if err != nil {
		return err
	}

	dir := filepath.Dir(idx.filePath)
	tempFile, err := os.CreateTemp(dir, "meta_tmp_*")
	if err != nil {
		return err
	}
	tempName := tempFile.Name()

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		os.Remove(tempName)
		return err
	}

	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		os.Remove(tempName)
		return err
	}

	if err := tempFile.Close(); err != nil {
		os.Remove(tempName)
		return err
	}

	return os.Rename(tempName, idx.filePath)
}

// UpsertNode registers or updates a node's heartbeat, address, and capacity.
func (idx *Index) UpsertNode(node NodeRecord) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	existing, ok := idx.nodes[node.NodeID]
	if !ok {
		idx.nodes[node.NodeID] = &node
		return
	}

	existing.Address = node.Address
	existing.LastSeen = time.Now().UTC()
	existing.Status = NodeOnline
	if len(node.DisplayName) > 0 {
		existing.DisplayName = node.DisplayName
	}
	if node.CapacityBytes > 0 {
		existing.CapacityBytes = node.CapacityBytes
	}
	existing.UsedBytes = node.UsedBytes
}

// MarkStaleNodes marks nodes that haven't sent heartbeats in timeout as offline.
func (idx *Index) MarkStaleNodes(timeout time.Duration) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	threshold := time.Now().UTC().Add(-timeout)
	for _, n := range idx.nodes {
		if n.LastSeen.Before(threshold) {
			n.Status = NodeOffline
		}
	}
}

// GetNodes returns a list of all known nodes.
func (idx *Index) GetNodes() []NodeRecord {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	out := make([]NodeRecord, 0, len(idx.nodes))
	for _, n := range idx.nodes {
		out = append(out, *n)
	}
	return out
}

// SelectPlacementNodes selects the best distinct nodes to store chunk replicas,
// ranked by available capacity among healthy online nodes.
func (idx *Index) SelectPlacementNodes(replicationFactor int, localNodeID string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	type candidate struct {
		id        string
		availSize int64
	}

	candidates := make([]candidate, 0)
	for id, node := range idx.nodes {
		if node.Status != NodeOnline && id != localNodeID {
			continue
		}
		avail := node.CapacityBytes - node.UsedBytes
		if avail <= 0 {
			avail = 1 // Default min if capacity not explicitly reported
		}
		candidates = append(candidates, candidate{id: id, availSize: avail})
	}

	// Sort candidates descending by available storage
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].availSize > candidates[j].availSize
	})

	selected := make([]string, 0, replicationFactor)
	for _, c := range candidates {
		selected = append(selected, c.id)
		if len(selected) >= replicationFactor {
			break
		}
	}

	// Ensure localNode is included if available
	if len(selected) == 0 && len(localNodeID) > 0 {
		selected = append(selected, localNodeID)
	}

	return selected
}

// RecordChunkLocation notes that a chunk is stored on a specific node.
func (idx *Index) RecordChunkLocation(chunkID string, nodeID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	locs := idx.locations[chunkID]
	for _, loc := range locs {
		if loc.NodeID == nodeID {
			return
		}
	}

	idx.locations[chunkID] = append(locs, ChunkLocation{
		ChunkID:        chunkID,
		NodeID:         nodeID,
		HealthStatus:   "healthy",
		LastVerifiedAt: time.Now().UTC(),
	})
}

// GetChunkLocations returns the node IDs where replicas of this chunk live.
func (idx *Index) GetChunkLocations(chunkID string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	locs := idx.locations[chunkID]
	out := make([]string, 0, len(locs))
	for _, l := range locs {
		if l.HealthStatus == "healthy" {
			out = append(out, l.NodeID)
		}
	}
	return out
}

// CommitFile registers a successfully uploaded file and its manifest into the shared namespace.
func (idx *Index) CommitFile(manifest *chunking.Manifest, logicalPath string) *FileRecord {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	now := time.Now().UTC()
	record := &FileRecord{
		FileID:      manifest.FileID,
		LogicalPath: logicalPath,
		FileName:    manifest.FileName,
		SizeBytes:   manifest.SizeBytes,
		ContentHash: manifest.ContentHash,
		Status:      FileAvailable,
		CreatedAt:   now,
		UpdatedAt:   now,
		Chunks:      manifest.Chunks,
	}

	idx.files[manifest.FileID] = record
	return record
}

// GetFile retrieves a file record by ID.
func (idx *Index) GetFile(fileID string) (*FileRecord, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	f, ok := idx.files[fileID]
	if !ok || f.Status == FileDeleted {
		return nil, ErrFileNotFound
	}
	return f, nil
}

// ListFiles lists all available files.
func (idx *Index) ListFiles() []FileRecord {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	out := make([]FileRecord, 0)
	for _, f := range idx.files {
		if f.Status != FileDeleted {
			out = append(out, *f)
		}
	}
	return out
}

// DeleteFile marks a file as deleted.
func (idx *Index) DeleteFile(fileID string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	f, ok := idx.files[fileID]
	if !ok {
		return ErrFileNotFound
	}
	f.Status = FileDeleted
	f.UpdatedAt = time.Now().UTC()
	return nil
}

// StartTransfer creates or updates a transfer record.
func (idx *Index) StartTransfer(rec TransferRecord) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	rec.StartedAt = time.Now().UTC()
	rec.State = TransferActive
	idx.transfers[rec.TransferID] = &rec
}

// UpdateTransferProgress updates transfer progress.
func (idx *Index) UpdateTransferProgress(transferID string, bytes int64, speed int64) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if t, ok := idx.transfers[transferID]; ok {
		t.BytesTransferred = bytes
		t.SpeedBytesSec = speed
	}
}

// CompleteTransfer marks a transfer completed or failed.
func (idx *Index) CompleteTransfer(transferID string, err error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if t, ok := idx.transfers[transferID]; ok {
		t.CompletedAt = time.Now().UTC()
		if err != nil {
			t.State = TransferFailed
			t.ErrorMsg = err.Error()
		} else {
			t.State = TransferCompleted
			t.BytesTransferred = t.TotalBytes
		}
	}
}

// ListTransfers returns all recorded transfers.
func (idx *Index) ListTransfers() []TransferRecord {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	out := make([]TransferRecord, 0, len(idx.transfers))
	for _, t := range idx.transfers {
		out = append(out, *t)
	}
	return out
}

// MeshStorageStats calculates aggregated mesh statistics.
func (idx *Index) MeshStorageStats() (totalCap int64, totalUsed int64, available int64, onlineNodes int, totalNodes int) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	totalNodes = len(idx.nodes)
	for _, n := range idx.nodes {
		totalCap += n.CapacityBytes
		totalUsed += n.UsedBytes
		if n.Status == NodeOnline {
			onlineNodes++
		}
	}
	available = totalCap - totalUsed
	if available < 0 {
		available = 0
	}
	return
}

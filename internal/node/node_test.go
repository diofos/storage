package node

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthdm/foreverstore/internal/config"
)

func TestMeshStoreCluster(t *testing.T) {
	tempRoot := filepath.Join(os.TempDir(), "meshstore_cluster_test")
	defer os.RemoveAll(tempRoot)

	// Node 1 setup
	cfg1 := config.DefaultConfig("7101")
	cfg1.Node.DataDirectory = filepath.Join(tempRoot, "node1")
	cfg1.Node.DisplayName = "Node-1"
	n1, err := NewNode(cfg1)
	if err != nil {
		t.Fatalf("failed to create node 1: %v", err)
	}
	if err := n1.Start(); err != nil {
		t.Fatalf("failed to start node 1: %v", err)
	}
	defer n1.Stop()

	// Node 2 setup (peers with Node 1)
	cfg2 := config.DefaultConfig("7102")
	cfg2.Node.DataDirectory = filepath.Join(tempRoot, "node2")
	cfg2.Node.DisplayName = "Node-2"
	cfg2.Mesh.Peers = []string{"127.0.0.1:7101"}
	n2, err := NewNode(cfg2)
	if err != nil {
		t.Fatalf("failed to create node 2: %v", err)
	}
	if err := n2.Start(); err != nil {
		t.Fatalf("failed to start node 2: %v", err)
	}
	defer n2.Stop()

	// Wait for peer handshake
	time.Sleep(500 * time.Millisecond)

	// Verify n1 has n2, and n2 has n1
	n1.peersMu.RLock()
	peerCount1 := len(n1.peers)
	n1.peersMu.RUnlock()

	n2.peersMu.RLock()
	peerCount2 := len(n2.peers)
	n2.peersMu.RUnlock()

	if peerCount1 == 0 || peerCount2 == 0 {
		t.Fatalf("expected peers connected, got n1=%d, n2=%d", peerCount1, peerCount2)
	}

	// 1. Upload file on Node 1
	filePayload := []byte("Hello MeshStore! This is distributed content across mesh nodes.")
	record, err := n1.UploadFile("greeting.txt", "/documents/greeting.txt", bytes.NewReader(filePayload))
	if err != nil {
		t.Fatalf("failed to upload file: %v", err)
	}

	if record.SizeBytes != int64(len(filePayload)) {
		t.Errorf("expected size %d, got %d", len(filePayload), record.SizeBytes)
	}

	// Wait for sync manifest broadcast to reach n2
	time.Sleep(300 * time.Millisecond)

	// 2. Verify file is discoverable on Node 2
	fileOnN2, err := n2.Index.GetFile(record.FileID)
	if err != nil {
		t.Fatalf("file not found on node 2 index: %v", err)
	}
	if fileOnN2.FileName != "greeting.txt" {
		t.Errorf("file name mismatch on node 2: %s", fileOnN2.FileName)
	}

	// 3. Download file through Node 2 (fetches chunks over P2P network)
	var downloadedBuf bytes.Buffer
	if err := n2.DownloadFile(record.FileID, &downloadedBuf); err != nil {
		t.Fatalf("failed to download file from node 2: %v", err)
	}

	if !bytes.Equal(downloadedBuf.Bytes(), filePayload) {
		t.Errorf("downloaded content on node 2 does not match original! Got: %s", downloadedBuf.String())
	}

	// 4. Test delete
	if err := n1.Index.DeleteFile(record.FileID); err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}

	files := n1.Index.ListFiles()
	for _, f := range files {
		if f.FileID == record.FileID {
			t.Errorf("file should not appear in active file list after delete")
		}
	}
}

func TestSliceReader(t *testing.T) {
	data := []byte("test data")
	sr := &sliceReader{data: data}
	out, err := io.ReadAll(sr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) {
		t.Errorf("slice reader mismatch")
	}
}

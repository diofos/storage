package chunking

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"
)

func TestChunker(t *testing.T) {
	data := []byte("The quick brown fox jumps over the lazy dog! Repeat sentence multiple times to cross chunk boundaries.")
	expectedFileHash := sha256.Sum256(data)

	chunker := NewChunker(16) // Small 16-byte chunks
	chunksStored := make(map[string][]byte)

	manifest, err := chunker.ProcessStream("file-1", "fox.txt", bytes.NewReader(data), func(info ChunkInfo, r io.Reader) error {
		chunkBytes, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		chunksStored[info.ChunkID] = chunkBytes
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected chunker error: %v", err)
	}

	if manifest.SizeBytes != int64(len(data)) {
		t.Errorf("expected size %d, got %d", len(data), manifest.SizeBytes)
	}

	if manifest.ContentHash != hex.EncodeToString(expectedFileHash[:]) {
		t.Errorf("content hash mismatch")
	}

	// Verify chunks reconstruct exact original data
	var reconstructed bytes.Buffer
	for _, chunkInfo := range manifest.Chunks {
		chunkBytes, ok := chunksStored[chunkInfo.ChunkID]
		if !ok {
			t.Fatalf("missing chunk %s", chunkInfo.ChunkID)
		}
		reconstructed.Write(chunkBytes)
	}

	if !bytes.Equal(reconstructed.Bytes(), data) {
		t.Errorf("reconstructed data did not match original")
	}
}

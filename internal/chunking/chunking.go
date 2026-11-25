package chunking

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// ChunkInfo describes a single verified chunk.
type ChunkInfo struct {
	Index     int    `json:"index"`
	ChunkID   string `json:"chunk_id"` // SHA-256 hex string
	SizeBytes int64  `json:"size_bytes"`
}

// Manifest represents the complete chunk blueprint of a file.
type Manifest struct {
	FileID      string      `json:"file_id"`
	FileName    string      `json:"file_name"`
	SizeBytes   int64       `json:"size_bytes"`
	ContentHash string      `json:"content_hash"`
	Chunks      []ChunkInfo `json:"chunks"`
}

// Chunker splits an io.Reader into chunks and provides streaming access to each chunk.
type Chunker struct {
	ChunkSizeBytes int64
}

// NewChunker creates a new Chunker with given chunk size in bytes.
func NewChunker(chunkSizeBytes int64) *Chunker {
	if chunkSizeBytes <= 0 {
		chunkSizeBytes = 1024 * 1024 // 1MB default
	}
	return &Chunker{
		ChunkSizeBytes: chunkSizeBytes,
	}
}

// ChunkCallback is invoked for every chunk processed with its data reader.
type ChunkCallback func(info ChunkInfo, r io.Reader) error

// ProcessStream reads src, splits it into chunks, hashes each chunk and the whole file,
// invoking cb for each chunk. Returns the final Manifest.
func (c *Chunker) ProcessStream(fileID string, fileName string, src io.Reader, cb ChunkCallback) (*Manifest, error) {
	fileHasher := sha256.New()
	manifest := &Manifest{
		FileID:   fileID,
		FileName: fileName,
		Chunks:   make([]ChunkInfo, 0),
	}

	buf := make([]byte, c.ChunkSizeBytes)
	var totalSize int64
	var chunkIndex int

	for {
		n, err := io.ReadFull(src, buf)
		if n > 0 {
			chunkData := buf[:n]
			totalSize += int64(n)
			fileHasher.Write(chunkData)

			chunkHasher := sha256.New()
			chunkHasher.Write(chunkData)
			chunkHash := hex.EncodeToString(chunkHasher.Sum(nil))

			info := ChunkInfo{
				Index:     chunkIndex,
				ChunkID:   chunkHash,
				SizeBytes: int64(n),
			}

			if cb != nil {
				if err := cb(info, &sliceReader{data: chunkData}); err != nil {
					return nil, fmt.Errorf("chunk %d callback failed: %w", chunkIndex, err)
				}
			}

			manifest.Chunks = append(manifest.Chunks, info)
			chunkIndex++
		}

		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read error while chunking: %w", err)
		}
	}

	manifest.SizeBytes = totalSize
	manifest.ContentHash = hex.EncodeToString(fileHasher.Sum(nil))

	return manifest, nil
}

type sliceReader struct {
	data   []byte
	offset int
}

func (s *sliceReader) Read(p []byte) (int, error) {
	if s.offset >= len(s.data) {
		return 0, io.EOF
	}
	n := copy(p, s.data[s.offset:])
	s.offset += n
	return n, nil
}

package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"github.com/anthdm/foreverstore/internal/chunking"
)

type MsgType byte

const (
	TypeHello           MsgType = 0x10
	TypeHelloAck        MsgType = 0x11
	TypeHeartbeat       MsgType = 0x12
	TypeStoreChunk      MsgType = 0x20
	TypeStoreChunkAck   MsgType = 0x21
	TypeGetChunk        MsgType = 0x30
	TypeGetChunkAck     MsgType = 0x31
	TypeSyncManifest    MsgType = 0x40
	TypeSyncManifestAck MsgType = 0x41
)

type MsgHello struct {
	NetworkID     string `json:"network_id"`
	NodeID        string `json:"node_id"`
	DisplayName   string `json:"display_name"`
	ListenAddress string `json:"listen_address"`
	CapacityBytes int64  `json:"capacity_bytes"`
	UsedBytes     int64  `json:"used_bytes"`
}

type MsgHelloAck struct {
	Success  bool   `json:"success"`
	NodeID   string `json:"node_id"`
	ErrorMsg string `json:"error_msg,omitempty"`
}

type MsgHeartbeat struct {
	NodeID        string `json:"node_id"`
	CapacityBytes int64  `json:"capacity_bytes"`
	UsedBytes     int64  `json:"used_bytes"`
}

type MsgStoreChunk struct {
	ChunkID   string `json:"chunk_id"`
	SizeBytes int64  `json:"size_bytes"`
}

type MsgStoreChunkAck struct {
	ChunkID  string `json:"chunk_id"`
	Success  bool   `json:"success"`
	ErrorMsg string `json:"error_msg,omitempty"`
}

type MsgGetChunk struct {
	ChunkID string `json:"chunk_id"`
}

type MsgGetChunkAck struct {
	ChunkID   string `json:"chunk_id"`
	Exists    bool   `json:"exists"`
	SizeBytes int64  `json:"size_bytes"`
	ErrorMsg  string `json:"error_msg,omitempty"`
}

type MsgSyncManifest struct {
	Manifest    chunking.Manifest `json:"manifest"`
	LogicalPath string            `json:"logical_path"`
}

type MsgSyncManifestAck struct {
	FileID   string `json:"file_id"`
	Success  bool   `json:"success"`
	ErrorMsg string `json:"error_msg,omitempty"`
}

// WriteMessage writes [MsgType (1B)] [PayloadLen (4B)] [JSON payload] to w.
func WriteMessage(w io.Writer, msgType MsgType, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal protocol message: %w", err)
	}

	header := make([]byte, 5)
	header[0] = byte(msgType)
	binary.BigEndian.PutUint32(header[1:5], uint32(len(data)))

	if _, err := w.Write(header); err != nil {
		return err
	}

	_, err = w.Write(data)
	return err
}

// ReadMessageHeader reads the 5-byte frame header returning the MsgType and JSON payload length.
func ReadMessageHeader(r io.Reader) (MsgType, uint32, error) {
	header := make([]byte, 5)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, 0, err
	}

	msgType := MsgType(header[0])
	length := binary.BigEndian.Uint32(header[1:5])
	return msgType, length, nil
}

// ReadMessagePayload reads length bytes from r and unmarshals into target.
func ReadMessagePayload(r io.Reader, length uint32, target any) error {
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

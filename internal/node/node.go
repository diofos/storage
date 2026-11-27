package node

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/anthdm/foreverstore/internal/chunking"
	"github.com/anthdm/foreverstore/internal/config"
	"github.com/anthdm/foreverstore/internal/identity"
	"github.com/anthdm/foreverstore/internal/metadata"
	"github.com/anthdm/foreverstore/internal/protocol"
	"github.com/anthdm/foreverstore/internal/storage"
)

var (
	ErrNodeNotAvailable = errors.New("selected storage node is currently offline")
)

// PeerConn wraps a live network connection to an authenticated peer.
type getResult struct {
	ack  protocol.MsgGetChunkAck
	data []byte
	err  error
}

type PeerConn struct {
	NodeID      string
	DisplayName string
	Address     string
	Conn        net.Conn
	mu          sync.Mutex

	reqMu      sync.Mutex
	storeAckCh chan protocol.MsgStoreChunkAck
	getAckCh   chan getResult
}

func (p *PeerConn) Send(msgType protocol.MsgType, payload any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return protocol.WriteMessage(p.Conn, msgType, payload)
}

// Node represents a running MeshStore peer node agent.
type Node struct {
	Config   *config.Config
	Identity *identity.NodeIdentity
	Store    *storage.ChunkStore
	Index    *metadata.Index

	listener net.Listener
	peersMu  sync.RWMutex
	peers    map[string]*PeerConn // nodeID -> PeerConn

	quitCh chan struct{}
	wg     sync.WaitGroup
}

// NewNode boots a Node agent from the provided configuration.
func NewNode(cfg *config.Config) (*Node, error) {
	id, err := identity.LoadOrCreateIdentity(cfg.Node.DataDirectory, cfg.Node.DisplayName)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize identity: %w", err)
	}

	maxBytes := cfg.Node.MaxStorageGB * 1024 * 1024 * 1024
	store, err := storage.NewChunkStore(cfg.Node.DataDirectory, maxBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize chunk storage: %w", err)
	}

	idx, err := metadata.NewIndex(cfg.Node.DataDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metadata index: %w", err)
	}

	// Register self in local index
	used, max, _ := store.Stats()
	idx.UpsertNode(metadata.NodeRecord{
		NodeID:        id.NodeID,
		DisplayName:   id.DisplayName,
		Address:       cfg.Node.ListenAddress,
		Status:        metadata.NodeOnline,
		CapacityBytes: max,
		UsedBytes:     used,
		LastSeen:      time.Now().UTC(),
	})

	return &Node{
		Config:   cfg,
		Identity: id,
		Store:    store,
		Index:    idx,
		peers:    make(map[string]*PeerConn),
		quitCh:   make(chan struct{}),
	}, nil
}

// Start opens the network listener, connects to bootstrap peers, and starts background routines.
func (n *Node) Start() error {
	ln, err := net.Listen("tcp", n.Config.Node.ListenAddress)
	if err != nil {
		return fmt.Errorf("failed to bind listen address %s: %w", n.Config.Node.ListenAddress, err)
	}
	n.listener = ln
	log.Printf("[%s] MeshStore node started at %s (ID: %s)", n.Identity.DisplayName, ln.Addr().String(), n.Identity.NodeID[:12])

	n.wg.Add(1)
	go n.acceptLoop()

	n.wg.Add(1)
	go n.backgroundLoop()

	// Connect to configured peers
	for _, peerAddr := range n.Config.Mesh.Peers {
		if peerAddr != "" && peerAddr != n.Config.Node.ListenAddress {
			go n.ConnectPeer(peerAddr)
		}
	}

	return nil
}

// Stop shuts down listener, peer connections, and persists metadata.
func (n *Node) Stop() {
	close(n.quitCh)
	if n.listener != nil {
		n.listener.Close()
	}

	n.peersMu.Lock()
	for _, p := range n.peers {
		p.Conn.Close()
	}
	n.peers = make(map[string]*PeerConn)
	n.peersMu.Unlock()

	n.wg.Wait()
	_ = n.Index.Save()
	log.Printf("[%s] MeshStore node shut down gracefully", n.Identity.DisplayName)
}

func (n *Node) acceptLoop() {
	defer n.wg.Done()
	for {
		conn, err := n.listener.Accept()
		if err != nil {
			select {
			case <-n.quitCh:
				return
			default:
				log.Printf("listener accept error: %v", err)
				return
			}
		}

		go n.handleConn(conn, false)
	}
}

// ConnectPeer dials a remote peer and performs the hello handshake.
func (n *Node) ConnectPeer(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return err
	}

	go n.handleConn(conn, true)
	return nil
}

func (n *Node) handleConn(conn net.Conn, outbound bool) {
	defer conn.Close()

	var remoteNodeID string
	defer func() {
		if remoteNodeID != "" {
			n.peersMu.Lock()
			delete(n.peers, remoteNodeID)
			n.peersMu.Unlock()
			log.Printf("[%s] disconnected peer: %s", n.Identity.DisplayName, remoteNodeID[:12])
		}
	}()

	used, max, _ := n.Store.Stats()

	if outbound {
		// Outbound: Send hello first
		hello := protocol.MsgHello{
			NetworkID:     n.Config.Mesh.NetworkID,
			NodeID:        n.Identity.NodeID,
			DisplayName:   n.Identity.DisplayName,
			ListenAddress: n.Config.Node.ListenAddress,
			CapacityBytes: max,
			UsedBytes:     used,
		}
		if err := protocol.WriteMessage(conn, protocol.TypeHello, hello); err != nil {
			return
		}

		// Read hello ack
		msgType, length, err := protocol.ReadMessageHeader(conn)
		if err != nil || msgType != protocol.TypeHelloAck {
			return
		}
		var ack protocol.MsgHelloAck
		if err := protocol.ReadMessagePayload(conn, length, &ack); err != nil || !ack.Success {
			return
		}
		remoteNodeID = ack.NodeID
	} else {
		// Inbound: Read hello first
		msgType, length, err := protocol.ReadMessageHeader(conn)
		if err != nil || msgType != protocol.TypeHello {
			return
		}
		var hello protocol.MsgHello
		if err := protocol.ReadMessagePayload(conn, length, &hello); err != nil {
			return
		}

		if hello.NetworkID != n.Config.Mesh.NetworkID {
			protocol.WriteMessage(conn, protocol.TypeHelloAck, protocol.MsgHelloAck{
				Success:  false,
				ErrorMsg: "network ID mismatch",
			})
			return
		}

		remoteNodeID = hello.NodeID
		n.Index.UpsertNode(metadata.NodeRecord{
			NodeID:        hello.NodeID,
			DisplayName:   hello.DisplayName,
			Address:       hello.ListenAddress,
			Status:        metadata.NodeOnline,
			CapacityBytes: hello.CapacityBytes,
			UsedBytes:     hello.UsedBytes,
			LastSeen:      time.Now().UTC(),
		})

		// Send hello ack
		ack := protocol.MsgHelloAck{
			Success: true,
			NodeID:  n.Identity.NodeID,
		}
		if err := protocol.WriteMessage(conn, protocol.TypeHelloAck, ack); err != nil {
			return
		}
	}

	peer := &PeerConn{
		NodeID:  remoteNodeID,
		Address: conn.RemoteAddr().String(),
		Conn:    conn,
	}

	n.peersMu.Lock()
	n.peers[remoteNodeID] = peer
	n.peersMu.Unlock()

	log.Printf("[%s] peer handshake successful with %s", n.Identity.DisplayName, remoteNodeID[:12])

	// Main message read loop
	for {
		msgType, length, err := protocol.ReadMessageHeader(conn)
		if err != nil {
			return
		}

		if err := n.dispatchMessage(peer, msgType, length); err != nil {
			log.Printf("error handling message %x from %s: %v", msgType, remoteNodeID[:12], err)
			return
		}
	}
}

func (n *Node) dispatchMessage(peer *PeerConn, msgType protocol.MsgType, length uint32) error {
	switch msgType {
	case protocol.TypeHeartbeat:
		var hb protocol.MsgHeartbeat
		if err := protocol.ReadMessagePayload(peer.Conn, length, &hb); err != nil {
			return err
		}
		n.Index.UpsertNode(metadata.NodeRecord{
			NodeID:        hb.NodeID,
			Status:        metadata.NodeOnline,
			CapacityBytes: hb.CapacityBytes,
			UsedBytes:     hb.UsedBytes,
			LastSeen:      time.Now().UTC(),
		})
		return nil

	case protocol.TypeStoreChunk:
		var storeReq protocol.MsgStoreChunk
		if err := protocol.ReadMessagePayload(peer.Conn, length, &storeReq); err != nil {
			return err
		}

		// Read raw chunk bytes from stream
		limitReader := io.LimitReader(peer.Conn, storeReq.SizeBytes)
		_, err := n.Store.Write(storeReq.ChunkID, storeReq.SizeBytes, limitReader)

		ack := protocol.MsgStoreChunkAck{
			ChunkID: storeReq.ChunkID,
			Success: err == nil,
		}
		if err != nil {
			ack.ErrorMsg = err.Error()
		} else {
			n.Index.RecordChunkLocation(storeReq.ChunkID, n.Identity.NodeID)
		}
		return peer.Send(protocol.TypeStoreChunkAck, ack)

	case protocol.TypeGetChunk:
		var getReq protocol.MsgGetChunk
		if err := protocol.ReadMessagePayload(peer.Conn, length, &getReq); err != nil {
			return err
		}

		rc, size, err := n.Store.OpenChunk(getReq.ChunkID)
		if err != nil {
			return peer.Send(protocol.TypeGetChunkAck, protocol.MsgGetChunkAck{
				ChunkID:  getReq.ChunkID,
				Exists:   false,
				ErrorMsg: err.Error(),
			})
		}
		defer rc.Close()

		peer.mu.Lock()
		defer peer.mu.Unlock()
		if err := protocol.WriteMessage(peer.Conn, protocol.TypeGetChunkAck, protocol.MsgGetChunkAck{
			ChunkID:   getReq.ChunkID,
			Exists:    true,
			SizeBytes: size,
		}); err != nil {
			return err
		}
		_, err = io.Copy(peer.Conn, rc)
		return err

	case protocol.TypeStoreChunkAck:
		var ack protocol.MsgStoreChunkAck
		if err := protocol.ReadMessagePayload(peer.Conn, length, &ack); err != nil {
			return err
		}
		peer.reqMu.Lock()
		ch := peer.storeAckCh
		peer.reqMu.Unlock()
		if ch != nil {
			select {
			case ch <- ack:
			default:
			}
		}
		return nil

	case protocol.TypeGetChunkAck:
		var ack protocol.MsgGetChunkAck
		if err := protocol.ReadMessagePayload(peer.Conn, length, &ack); err != nil {
			return err
		}
		var chunkData []byte
		var readErr error
		if ack.Exists && ack.SizeBytes > 0 {
			chunkData = make([]byte, ack.SizeBytes)
			_, readErr = io.ReadFull(peer.Conn, chunkData)
		}
		peer.reqMu.Lock()
		ch := peer.getAckCh
		peer.reqMu.Unlock()
		if ch != nil {
			select {
			case ch <- getResult{ack: ack, data: chunkData, err: readErr}:
			default:
			}
		}
		return nil

	case protocol.TypeSyncManifest:
		var syncReq protocol.MsgSyncManifest
		if err := protocol.ReadMessagePayload(peer.Conn, length, &syncReq); err != nil {
			return err
		}

		n.Index.CommitFile(&syncReq.Manifest, syncReq.LogicalPath)
		for _, ch := range syncReq.Manifest.Chunks {
			n.Index.RecordChunkLocation(ch.ChunkID, peer.NodeID)
		}

		return peer.Send(protocol.TypeSyncManifestAck, protocol.MsgSyncManifestAck{
			FileID:  syncReq.Manifest.FileID,
			Success: true,
		})

	case protocol.TypeSyncManifestAck:
		// Drain and discard payload
		io.CopyN(io.Discard, peer.Conn, int64(length))
		return nil
	}

	// Discard unhandled payload
	io.CopyN(io.Discard, peer.Conn, int64(length))
	return nil
}

func (n *Node) backgroundLoop() {
	defer n.wg.Done()
	heartbeatTicker := time.NewTicker(5 * time.Second)
	saveTicker := time.NewTicker(15 * time.Second)
	defer heartbeatTicker.Stop()
	defer saveTicker.Stop()

	for {
		select {
		case <-heartbeatTicker.C:
			used, max, _ := n.Store.Stats()

			// Keep local node marked online and fresh in index
			n.Index.UpsertNode(metadata.NodeRecord{
				NodeID:        n.Identity.NodeID,
				DisplayName:   n.Identity.DisplayName,
				Address:       n.Config.Node.ListenAddress,
				Status:        metadata.NodeOnline,
				CapacityBytes: max,
				UsedBytes:     used,
				LastSeen:      time.Now().UTC(),
			})

			hb := protocol.MsgHeartbeat{
				NodeID:        n.Identity.NodeID,
				CapacityBytes: max,
				UsedBytes:     used,
			}

			n.peersMu.RLock()
			for _, p := range n.peers {
				go p.Send(protocol.TypeHeartbeat, hb)
			}
			n.peersMu.RUnlock()

			// Mark stale remote nodes
			n.Index.MarkStaleNodes(15 * time.Second)

		case <-saveTicker.C:
			_ = n.Index.Save()

		case <-n.quitCh:
			return
		}
	}
}

// UploadFile chunks src, replicates chunks according to policy, and publishes manifest.
func (n *Node) UploadFile(fileName string, logicalPath string, src io.Reader) (*metadata.FileRecord, error) {
	buf := make([]byte, 16)
	rand.Read(buf)
	fileID := hex.EncodeToString(buf)

	transferID := "up_" + fileID[:8]
	n.Index.StartTransfer(metadata.TransferRecord{
		TransferID: transferID,
		FileID:     fileID,
		FileName:   fileName,
		Type:       "upload",
	})

	chunkSize := int64(n.Config.Storage.ChunkSizeMB) * 1024 * 1024
	chunker := chunking.NewChunker(chunkSize)

	var totalBytesUploaded int64
	startTime := time.Now()

	manifest, err := chunker.ProcessStream(fileID, fileName, src, func(info chunking.ChunkInfo, r io.Reader) error {
		// Read chunk into memory buffer for distribution
		chunkData, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		// Select nodes for replication
		targetNodes := n.Index.SelectPlacementNodes(n.Config.Storage.ReplicationFactor, n.Identity.NodeID)

		for _, nodeID := range targetNodes {
			if nodeID == n.Identity.NodeID {
				// Store locally
				if _, err := n.Store.Write(info.ChunkID, info.SizeBytes, &sliceReader{data: chunkData}); err != nil {
					return fmt.Errorf("local store write failed: %w", err)
				}
				n.Index.RecordChunkLocation(info.ChunkID, n.Identity.NodeID)
			} else {
				// Transmit to peer
				if err := n.sendChunkToPeer(nodeID, info.ChunkID, chunkData); err != nil {
					log.Printf("warning: replica to peer %s failed: %v", nodeID[:8], err)
				} else {
					n.Index.RecordChunkLocation(info.ChunkID, nodeID)
				}
			}
		}

		totalBytesUploaded += info.SizeBytes
		elapsed := time.Since(startTime).Seconds()
		speed := int64(0)
		if elapsed > 0 {
			speed = int64(float64(totalBytesUploaded) / elapsed)
		}
		n.Index.UpdateTransferProgress(transferID, totalBytesUploaded, speed)
		return nil
	})

	if err != nil {
		n.Index.CompleteTransfer(transferID, err)
		return nil, err
	}

	// Commit file to index
	record := n.Index.CommitFile(manifest, logicalPath)
	n.Index.CompleteTransfer(transferID, nil)

	// Broadcast sync manifest to all peers
	syncMsg := protocol.MsgSyncManifest{
		Manifest:    *manifest,
		LogicalPath: logicalPath,
	}
	n.peersMu.RLock()
	for _, p := range n.peers {
		go p.Send(protocol.TypeSyncManifest, syncMsg)
	}
	n.peersMu.RUnlock()

	_ = n.Index.Save()
	return record, nil
}

func (n *Node) sendChunkToPeer(nodeID string, chunkID string, data []byte) error {
	n.peersMu.RLock()
	peer, ok := n.peers[nodeID]
	n.peersMu.RUnlock()

	if !ok {
		return ErrNodeNotAvailable
	}

	ackCh := make(chan protocol.MsgStoreChunkAck, 1)
	peer.reqMu.Lock()
	peer.storeAckCh = ackCh
	peer.reqMu.Unlock()

	defer func() {
		peer.reqMu.Lock()
		peer.storeAckCh = nil
		peer.reqMu.Unlock()
	}()

	// 1. Send header & store chunk request + stream bytes
	req := protocol.MsgStoreChunk{
		ChunkID:   chunkID,
		SizeBytes: int64(len(data)),
	}
	peer.mu.Lock()
	if err := protocol.WriteMessage(peer.Conn, protocol.TypeStoreChunk, req); err != nil {
		peer.mu.Unlock()
		return err
	}
	if _, err := peer.Conn.Write(data); err != nil {
		peer.mu.Unlock()
		return err
	}
	peer.mu.Unlock()

	// 2. Wait for Ack
	select {
	case ack := <-ackCh:
		if !ack.Success {
			return fmt.Errorf("chunk ack reported error: %s", ack.ErrorMsg)
		}
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timed out waiting for chunk ack from %s", nodeID)
	}
}

// DownloadFile reconstructs the file from chunks and writes to dst.
func (n *Node) DownloadFile(fileID string, dst io.Writer) error {
	record, err := n.Index.GetFile(fileID)
	if err != nil {
		return err
	}

	transferID := "down_" + fileID[:8]
	n.Index.StartTransfer(metadata.TransferRecord{
		TransferID: transferID,
		FileID:     fileID,
		FileName:   record.FileName,
		TotalBytes: record.SizeBytes,
		Type:       "download",
	})

	var bytesDownloaded int64
	startTime := time.Now()

	for _, chunk := range record.Chunks {
		chunkReader, err := n.fetchChunk(chunk.ChunkID)
		if err != nil {
			n.Index.CompleteTransfer(transferID, err)
			return fmt.Errorf("failed to retrieve chunk %s: %w", chunk.ChunkID, err)
		}

		nWritten, err := io.Copy(dst, chunkReader)
		chunkReader.Close()
		if err != nil {
			n.Index.CompleteTransfer(transferID, err)
			return err
		}

		bytesDownloaded += nWritten
		elapsed := time.Since(startTime).Seconds()
		speed := int64(0)
		if elapsed > 0 {
			speed = int64(float64(bytesDownloaded) / elapsed)
		}
		n.Index.UpdateTransferProgress(transferID, bytesDownloaded, speed)
	}

	n.Index.CompleteTransfer(transferID, nil)
	return nil
}

func (n *Node) fetchChunk(chunkID string) (io.ReadCloser, error) {
	// 1. Check local store
	if n.Store.Has(chunkID) {
		rc, _, err := n.Store.OpenChunk(chunkID)
		return rc, err
	}

	// 2. Check remote peers
	locations := n.Index.GetChunkLocations(chunkID)
	for _, nodeID := range locations {
		n.peersMu.RLock()
		peer, ok := n.peers[nodeID]
		n.peersMu.RUnlock()

		if !ok {
			continue
		}

		getCh := make(chan getResult, 1)
		peer.reqMu.Lock()
		peer.getAckCh = getCh
		peer.reqMu.Unlock()

		req := protocol.MsgGetChunk{ChunkID: chunkID}
		peer.mu.Lock()
		err := protocol.WriteMessage(peer.Conn, protocol.TypeGetChunk, req)
		peer.mu.Unlock()
		if err != nil {
			peer.reqMu.Lock()
			peer.getAckCh = nil
			peer.reqMu.Unlock()
			continue
		}

		var res getResult
		select {
		case res = <-getCh:
		case <-time.After(10 * time.Second):
			peer.reqMu.Lock()
			peer.getAckCh = nil
			peer.reqMu.Unlock()
			continue
		}

		peer.reqMu.Lock()
		peer.getAckCh = nil
		peer.reqMu.Unlock()

		if res.err != nil || !res.ack.Exists {
			continue
		}

		// Cache chunk locally
		n.Store.Write(chunkID, res.ack.SizeBytes, &sliceReader{data: res.data})
		return &sliceReaderCloser{sliceReader: sliceReader{data: res.data}}, nil
	}

	return nil, fmt.Errorf("chunk %s unavailable on all replicas", chunkID)
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

type sliceReaderCloser struct {
	sliceReader
}

func (s *sliceReaderCloser) Close() error {
	return nil
}

// SetStorageQuotaGB updates the storage quota for this node.
func (n *Node) SetStorageQuotaGB(gb int64) {
	if gb <= 0 {
		return
	}
	n.Config.Node.MaxStorageGB = gb
	maxBytes := gb * 1024 * 1024 * 1024
	n.Store.SetMaxStorageBytes(maxBytes)
	used, max, _ := n.Store.Stats()
	n.Index.UpsertNode(metadata.NodeRecord{
		NodeID:        n.Identity.NodeID,
		DisplayName:   n.Identity.DisplayName,
		Address:       n.Config.Node.ListenAddress,
		Status:        metadata.NodeOnline,
		CapacityBytes: max,
		UsedBytes:     used,
		LastSeen:      time.Now().UTC(),
	})
	_ = n.Index.Save()
}


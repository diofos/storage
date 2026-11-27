package discovery

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// Beacon represents the periodic broadcast announcement over local Ethernet/Wi-Fi.
type Beacon struct {
	NodeID      string `json:"node_id"`
	DisplayName string `json:"display_name"`
	ListenAddr  string `json:"listen_addr"`
	NetworkID   string `json:"network_id"`
}

// DiscoveredNode represents a discovered peer on the local subnet.
type DiscoveredNode struct {
	Beacon
	RemoteIP string    `json:"remote_ip"`
	LastSeen time.Time `json:"last_seen"`
}

// DiscoveryService broadcasts and receives LAN beacons over UDP.
type DiscoveryService struct {
	beacon       Beacon
	port         int
	conn         *net.UDPConn
	nodesMu      sync.RWMutex
	nodes        map[string]DiscoveredNode
	quitCh       chan struct{}
	autoConnect  bool
	onPeerFound  func(node DiscoveredNode)
}

// NewDiscoveryService initializes a discovery agent on the given UDP port (e.g. 7005).
func NewDiscoveryService(b Beacon, port int, autoConnect bool, onPeerFound func(DiscoveredNode)) *DiscoveryService {
	if port <= 0 {
		port = 7005
	}
	return &DiscoveryService{
		beacon:      b,
		port:        port,
		nodes:       make(map[string]DiscoveredNode),
		quitCh:      make(chan struct{}),
		autoConnect: autoConnect,
		onPeerFound: onPeerFound,
	}
}

// Start begins broadcasting and listening on the LAN broadcast port.
func (ds *DiscoveryService) Start() error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("0.0.0.0:%d", ds.port))
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	ds.conn = conn

	go ds.listenLoop()
	go ds.broadcastLoop()
	go ds.cleanupLoop()

	return nil
}

// Stop shuts down the discovery service.
func (ds *DiscoveryService) Stop() {
	close(ds.quitCh)
	if ds.conn != nil {
		ds.conn.Close()
	}
}

// GetDiscoveredNodes returns currently active peers discovered on the LAN.
func (ds *DiscoveryService) GetDiscoveredNodes() []DiscoveredNode {
	ds.nodesMu.RLock()
	defer ds.nodesMu.RUnlock()

	out := make([]DiscoveredNode, 0, len(ds.nodes))
	for _, n := range ds.nodes {
		out = append(out, n)
	}
	return out
}

func (ds *DiscoveryService) broadcastLoop() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	broadcastAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("255.255.255.255:%d", ds.port))
	if err != nil {
		return
	}

	data, err := json.Marshal(ds.beacon)
	if err != nil {
		return
	}

	// Send initial burst
	_, _ = ds.conn.WriteToUDP(data, broadcastAddr)

	for {
		select {
		case <-ds.quitCh:
			return
		case <-ticker.C:
			_, _ = ds.conn.WriteToUDP(data, broadcastAddr)
		}
	}
}

func (ds *DiscoveryService) listenLoop() {
	buf := make([]byte, 2048)
	for {
		n, remoteAddr, err := ds.conn.ReadFrom(buf)
		if err != nil {
			select {
			case <-ds.quitCh:
				return
			default:
				continue
			}
		}

		var beacon Beacon
		if err := json.Unmarshal(buf[:n], &beacon); err != nil {
			continue
		}

		// Ignore own beacons
		if beacon.NodeID == ds.beacon.NodeID {
			continue
		}

		// Filter by network ID
		if beacon.NetworkID != ds.beacon.NetworkID {
			continue
		}

		remoteIP := remoteAddr.(*net.UDPAddr).IP.String()
		
		// If peer advertised 127.0.0.1 or localhost, resolve with actual sender IP
		listenHost, listenPort, err := net.SplitHostPort(beacon.ListenAddr)
		if err == nil && (listenHost == "127.0.0.1" || listenHost == "0.0.0.0" || listenHost == "") {
			beacon.ListenAddr = net.JoinHostPort(remoteIP, listenPort)
		}

		discovered := DiscoveredNode{
			Beacon:   beacon,
			RemoteIP: remoteIP,
			LastSeen: time.Now(),
		}

		ds.nodesMu.Lock()
		_, exists := ds.nodes[beacon.NodeID]
		ds.nodes[beacon.NodeID] = discovered
		ds.nodesMu.Unlock()

		if !exists && ds.onPeerFound != nil {
			go ds.onPeerFound(discovered)
		}
	}
}

func (ds *DiscoveryService) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ds.quitCh:
			return
		case <-ticker.C:
			ds.nodesMu.Lock()
			threshold := time.Now().Add(-30 * time.Second)
			for id, n := range ds.nodes {
				if n.LastSeen.Before(threshold) {
					delete(ds.nodes, id)
				}
			}
			ds.nodesMu.Unlock()
		}
	}
}

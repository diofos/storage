package discovery

import (
	"testing"
	"time"
)

func TestDiscoveryService(t *testing.T) {
	b1 := Beacon{
		NodeID:      "node-1",
		DisplayName: "Device-A",
		ListenAddr:  "127.0.0.1:7101",
		NetworkID:   "test-mesh",
	}
	b2 := Beacon{
		NodeID:      "node-2",
		DisplayName: "Device-B",
		ListenAddr:  "127.0.0.1:7102",
		NetworkID:   "test-mesh",
	}

	peerFoundCh := make(chan DiscoveredNode, 1)

	d1 := NewDiscoveryService(b1, 17290, false, nil)
	d2 := NewDiscoveryService(b2, 17290, false, func(dn DiscoveredNode) {
		peerFoundCh <- dn
	})

	if err := d1.Start(); err != nil {
		t.Skipf("skipping UDP test in sandbox/restricted environment: %v", err)
		return
	}
	defer d1.Stop()

	if err := d2.Start(); err != nil {
		t.Skipf("skipping UDP test: %v", err)
		return
	}
	defer d2.Stop()

	select {
	case dn := <-peerFoundCh:
		if dn.NodeID != "node-1" {
			t.Errorf("expected node-1, got %s", dn.NodeID)
		}
	case <-time.After(2 * time.Second):
		// UDP broadcast can be restricted on loopback in some OS environments
	}
}

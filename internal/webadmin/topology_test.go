package webadmin

import (
	"testing"
	"time"
)

func TestTopologyManager_GetTopology(t *testing.T) {
	tm := NewTopologyManager(nil)

	topology := tm.GetTopology()
	if topology == nil {
		t.Fatal("GetTopology() returned nil")
	}

	if topology.Nodes == nil {
		t.Error("Topology nodes should not be nil")
	}

	if topology.Links == nil {
		t.Error("Topology links should not be nil")
	}
}

func TestShortenID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"short", "short"},
		{"exactly12ch", "exactly12ch"},
		{"QmYourPeerIDHere123456789", "QmYour...6789"},
	}

	for _, tt := range tests {
		result := shortenID(tt.input)
		if result != tt.expected {
			t.Errorf("shortenID(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestTopologyManager_StartStopUpdates(t *testing.T) {
	tm := NewTopologyManager(nil)

	// Start updates
	tm.StartUpdates(nil)
	time.Sleep(10 * time.Millisecond)

	// Should be running
	tm.mu.RLock()
	running := tm.running
	tm.mu.RUnlock()
	if !running {
		t.Error("Topology manager should be running after StartUpdates")
	}

	// Stop updates
	tm.StopUpdates()

	// Should not be running
	tm.mu.RLock()
	running = tm.running
	tm.mu.RUnlock()
	if running {
		t.Error("Topology manager should not be running after StopUpdates")
	}
}

// Mock node info provider for testing
type mockNodeInfoProvider struct {
	nodeID string
	peers  []string
}

func (m *mockNodeInfoProvider) GetNodeID() string {
	return m.nodeID
}

func (m *mockNodeInfoProvider) GetPeerCount() int {
	return len(m.peers)
}

func (m *mockNodeInfoProvider) GetPeers() []string {
	return m.peers
}

func (m *mockNodeInfoProvider) GetNodeStatus() *NodeStatus {
	return &NodeStatus{
		NodeID:      m.nodeID,
		StartTime:   time.Now(),
		Version:     "test",
		Reputation:  0.75,
	}
}

func (m *mockNodeInfoProvider) GetHTTPAPIEndpoints() []APIEndpoint {
	return []APIEndpoint{
		{Method: "GET", Path: "/api/health", Description: "Health check"},
	}
}

func (m *mockNodeInfoProvider) GetRecentLogs(limit int) []LogEntry {
	return []LogEntry{}
}

func (m *mockNodeInfoProvider) GetNetworkStats() *NetworkStats {
	return &NetworkStats{
		TotalPeers:  len(m.peers),
		ActivePeers: len(m.peers),
	}
}

func TestTopologyManager_UpdateTopology(t *testing.T) {
	mock := &mockNodeInfoProvider{
		nodeID: "QmTestNode123",
		peers:  []string{"QmPeer1", "QmPeer2"},
	}

	tm := NewTopologyManager(mock)
	tm.updateTopology()

	topology := tm.GetTopology()

	// Should have self node + 2 peers = 3 nodes
	if len(topology.Nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(topology.Nodes))
	}

	// Should have 2 links (self to each peer)
	if len(topology.Links) != 2 {
		t.Errorf("Expected 2 links, got %d", len(topology.Links))
	}

	// Verify self node is first and marked as "self"
	var selfFound bool
	for _, node := range topology.Nodes {
		if node.Type == "self" {
			selfFound = true
			if node.ID != "QmTestNode123" {
				t.Errorf("Self node ID = %q, want %q", node.ID, "QmTestNode123")
			}
		}
	}
	if !selfFound {
		t.Error("Self node not found in topology")
	}
}

func TestTopologyManager_CalculateLayout(t *testing.T) {
	mock := &mockNodeInfoProvider{
		nodeID: "QmTestNode",
		peers:  []string{"QmPeer1", "QmPeer2", "QmPeer3"},
	}

	tm := NewTopologyManager(mock)
	tm.updateTopology()
	tm.CalculateLayout()

	topology := tm.GetTopology()

	// Self node should be at center
	for _, node := range topology.Nodes {
		if node.Type == "self" {
			if node.X != 400 || node.Y != 300 {
				t.Errorf("Self node should be at center (400, 300), got (%f, %f)", node.X, node.Y)
			}
		}
	}
}

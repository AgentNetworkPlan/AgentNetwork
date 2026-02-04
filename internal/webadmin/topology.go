package webadmin

import (
	"encoding/json"
	"sync"
	"time"
)

// TopologyNode represents a node in the network topology.
type TopologyNode struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Type       string  `json:"type"` // "self", "peer", "supernode", "genesis"
	X          float64 `json:"x,omitempty"`
	Y          float64 `json:"y,omitempty"`
	Reputation float64 `json:"reputation"`
	Status     string  `json:"status"` // "online", "offline", "unknown"
}

// TopologyLink represents a link between nodes.
type TopologyLink struct {
	Source  string  `json:"source"`
	Target  string  `json:"target"`
	Latency float64 `json:"latency"` // in milliseconds
	Status  string  `json:"status"`  // "active", "inactive"
}

// Topology represents the network topology.
type Topology struct {
	Nodes     []TopologyNode `json:"nodes"`
	Links     []TopologyLink `json:"links"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// TopologyManager manages network topology visualization data.
type TopologyManager struct {
	nodeInfo NodeInfoProvider
	topology *Topology
	mu       sync.RWMutex

	stopChan chan struct{}
	running  bool
}

// NewTopologyManager creates a new topology manager.
func NewTopologyManager(nodeInfo NodeInfoProvider) *TopologyManager {
	return &TopologyManager{
		nodeInfo: nodeInfo,
		topology: &Topology{
			Nodes:     []TopologyNode{},
			Links:     []TopologyLink{},
			UpdatedAt: time.Now(),
		},
		stopChan: make(chan struct{}),
	}
}

// GetTopology returns the current topology.
func (tm *TopologyManager) GetTopology() *Topology {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// Return a copy
	nodes := make([]TopologyNode, len(tm.topology.Nodes))
	copy(nodes, tm.topology.Nodes)

	links := make([]TopologyLink, len(tm.topology.Links))
	copy(links, tm.topology.Links)

	return &Topology{
		Nodes:     nodes,
		Links:     links,
		UpdatedAt: tm.topology.UpdatedAt,
	}
}

// StartUpdates starts the topology update loop.
func (tm *TopologyManager) StartUpdates(hub *WebSocketHub) {
	tm.mu.Lock()
	if tm.running {
		tm.mu.Unlock()
		return
	}
	tm.running = true
	tm.stopChan = make(chan struct{})
	tm.mu.Unlock()

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				tm.updateTopology()

				// Broadcast to WebSocket clients
				if hub != nil && hub.ClientCount("topology") > 0 {
					data, err := json.Marshal(tm.GetTopology())
					if err == nil {
						hub.Broadcast("topology", data)
					}
				}

			case <-tm.stopChan:
				return
			}
		}
	}()
}

// StopUpdates stops the topology update loop.
func (tm *TopologyManager) StopUpdates() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.running {
		close(tm.stopChan)
		tm.running = false
	}
}

// updateTopology updates the topology from the node info provider.
func (tm *TopologyManager) updateTopology() {
	if tm.nodeInfo == nil {
		return
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Get current node info
	status := tm.nodeInfo.GetNodeStatus()
	peers := tm.nodeInfo.GetPeers()

	nodes := []TopologyNode{}
	links := []TopologyLink{}

	// Add self node
	if status != nil {
		nodes = append(nodes, TopologyNode{
			ID:         status.NodeID,
			Name:       "Self",
			Type:       "self", // Always mark self node as "self" type
			Reputation: status.Reputation,
			Status:     "online",
		})
	}

	// Add peer nodes and links
	for _, peerID := range peers {
		nodes = append(nodes, TopologyNode{
			ID:         peerID,
			Name:       shortenID(peerID),
			Type:       "peer",
			Reputation: 0.5, // Default, would need to fetch actual reputation
			Status:     "online",
		})

		// Add link from self to peer
		if status != nil {
			links = append(links, TopologyLink{
				Source:  status.NodeID,
				Target:  peerID,
				Latency: 0, // Would need actual latency measurement
				Status:  "active",
			})
		}
	}

	tm.topology = &Topology{
		Nodes:     nodes,
		Links:     links,
		UpdatedAt: time.Now(),
	}
}

// shortenID shortens a node ID for display.
func shortenID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:6] + "..." + id[len(id)-4:]
}

// CalculateLayout calculates node positions using a simple force-directed layout.
// This is a simplified version; the frontend will do the actual visualization.
func (tm *TopologyManager) CalculateLayout() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	n := len(tm.topology.Nodes)
	if n == 0 {
		return
	}

	// Simple circular layout
	radius := 200.0
	centerX, centerY := 400.0, 300.0

	for i := range tm.topology.Nodes {
		angle := float64(i) * (2 * 3.14159 / float64(n))
		if tm.topology.Nodes[i].Type == "self" {
			// Center the self node
			tm.topology.Nodes[i].X = centerX
			tm.topology.Nodes[i].Y = centerY
		} else {
			tm.topology.Nodes[i].X = centerX + radius*cos(angle)
			tm.topology.Nodes[i].Y = centerY + radius*sin(angle)
		}
	}
}

// Simple math functions to avoid importing math package for just these
func cos(x float64) float64 {
	// Taylor series approximation (good enough for layout)
	x = mod(x, 2*3.14159)
	return 1 - (x*x)/2 + (x*x*x*x)/24 - (x*x*x*x*x*x)/720
}

func sin(x float64) float64 {
	// Taylor series approximation
	x = mod(x, 2*3.14159)
	return x - (x*x*x)/6 + (x*x*x*x*x)/120 - (x*x*x*x*x*x*x)/5040
}

func mod(x, y float64) float64 {
	for x >= y {
		x -= y
	}
	for x < 0 {
		x += y
	}
	return x
}

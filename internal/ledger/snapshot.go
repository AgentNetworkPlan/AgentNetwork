package ledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StateSnapshot represents a point-in-time snapshot of the ledger state
type StateSnapshot struct {
	Sequence   uint64                   `json:"seq"`        // Snapshot at this sequence
	Timestamp  int64                    `json:"timestamp"`  // When snapshot was taken
	Nodes      map[string]*NodeState    `json:"nodes"`      // Node states
	Guarantees map[string]*GuaranteeState `json:"guarantees"` // Guarantee states
	Hash       string                   `json:"hash"`       // Snapshot hash
}

// NodeState represents the state of a node at a point in time
type NodeState struct {
	NodeID     string   `json:"node_id"`
	PubKey     string   `json:"pubkey"`
	Reputation float64  `json:"reputation"`
	Status     string   `json:"status"` // active, suspended, kicked
	JoinedAt   int64    `json:"joined_at"`
	SponsorID  string   `json:"sponsor_id"`
	Guarantees []string `json:"guarantees"` // Guarantee IDs where this node is sponsor
}

// GuaranteeState represents the state of a guarantee
type GuaranteeState struct {
	GuaranteeID     string  `json:"guarantee_id"`
	SponsorID       string  `json:"sponsor_id"`
	NewNodeID       string  `json:"new_node_id"`
	GuaranteeAmount float64 `json:"guarantee_amount"`
	LiabilityRatio  float64 `json:"liability_ratio"`
	ValidUntil      int64   `json:"valid_until"`
	Status          string  `json:"status"` // active, expired, revoked, settled
	CreatedAt       int64   `json:"created_at"`
}

// SnapshotManager manages state snapshots for the ledger
type SnapshotManager struct {
	dataDir       string
	snapshots     []*StateSnapshot
	latestSeq     uint64
	snapshotInterval uint64 // Create snapshot every N events
	mu            sync.RWMutex
}

// NewSnapshotManager creates a new snapshot manager
func NewSnapshotManager(dataDir string, interval uint64) (*SnapshotManager, error) {
	if interval == 0 {
		interval = 100 // Default: snapshot every 100 events
	}

	sm := &SnapshotManager{
		dataDir:          dataDir,
		snapshots:        make([]*StateSnapshot, 0),
		latestSeq:        0,
		snapshotInterval: interval,
	}

	// Create data directory if not exists
	if dataDir != "" {
		snapshotDir := filepath.Join(dataDir, "snapshots")
		if err := os.MkdirAll(snapshotDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create snapshot directory: %w", err)
		}

		// Load existing snapshots
		if err := sm.loadSnapshots(); err != nil {
			// If no snapshots exist, that's OK
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to load snapshots: %w", err)
			}
		}
	}

	return sm, nil
}

// ShouldSnapshot checks if a snapshot should be created at the given sequence
func (sm *SnapshotManager) ShouldSnapshot(seq uint64) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return seq > 0 && seq%sm.snapshotInterval == 0
}

// CreateSnapshot creates a new snapshot from the ledger
func (sm *SnapshotManager) CreateSnapshot(ledger *Ledger) (*StateSnapshot, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Build state from events
	snapshot, err := sm.buildStateFromEvents(ledger)
	if err != nil {
		return nil, fmt.Errorf("failed to build state: %w", err)
	}

	// Add to snapshots list
	sm.snapshots = append(sm.snapshots, snapshot)
	sm.latestSeq = snapshot.Sequence

	// Persist snapshot
	if sm.dataDir != "" {
		if err := sm.saveSnapshot(snapshot); err != nil {
			return nil, fmt.Errorf("failed to save snapshot: %w", err)
		}
	}

	return snapshot, nil
}

// GetLatestSnapshot returns the most recent snapshot
func (sm *SnapshotManager) GetLatestSnapshot() *StateSnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.snapshots) == 0 {
		return nil
	}
	return sm.snapshots[len(sm.snapshots)-1]
}

// GetSnapshotAt returns the snapshot closest to (but not after) the given sequence
func (sm *SnapshotManager) GetSnapshotAt(seq uint64) *StateSnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var result *StateSnapshot
	for _, s := range sm.snapshots {
		if s.Sequence <= seq {
			result = s
		} else {
			break
		}
	}
	return result
}

// GetStateAt reconstructs the state at a given sequence
// by starting from the nearest snapshot and replaying events
func (sm *SnapshotManager) GetStateAt(ledger *Ledger, seq uint64) (*StateSnapshot, error) {
	// Find nearest snapshot
	baseSnapshot := sm.GetSnapshotAt(seq)

	// Start from snapshot or genesis
	var state *StateSnapshot
	var startSeq uint64

	if baseSnapshot != nil {
		state = sm.cloneSnapshot(baseSnapshot)
		startSeq = baseSnapshot.Sequence + 1
	} else {
		state = &StateSnapshot{
			Sequence:   0,
			Timestamp:  time.Now().Unix(),
			Nodes:      make(map[string]*NodeState),
			Guarantees: make(map[string]*GuaranteeState),
		}
		startSeq = 1
	}

	// Replay events from startSeq to seq
	events := ledger.GetEvents(startSeq, seq)
	for _, event := range events {
		if err := sm.applyEvent(state, event); err != nil {
			return nil, fmt.Errorf("failed to apply event %d: %w", event.Sequence, err)
		}
		state.Sequence = event.Sequence
		state.Timestamp = event.Timestamp
	}

	// Compute hash
	state.Hash = sm.computeSnapshotHash(state)

	return state, nil
}

// buildStateFromEvents builds the current state by replaying all events
func (sm *SnapshotManager) buildStateFromEvents(ledger *Ledger) (*StateSnapshot, error) {
	state := &StateSnapshot{
		Sequence:   0,
		Timestamp:  time.Now().Unix(),
		Nodes:      make(map[string]*NodeState),
		Guarantees: make(map[string]*GuaranteeState),
	}

	// Replay all events
	lastSeq := ledger.GetLastSequence()
	events := ledger.GetEvents(1, lastSeq)

	for _, event := range events {
		if err := sm.applyEvent(state, event); err != nil {
			return nil, fmt.Errorf("failed to apply event %d: %w", event.Sequence, err)
		}
		state.Sequence = event.Sequence
		state.Timestamp = event.Timestamp
	}

	// Compute hash
	state.Hash = sm.computeSnapshotHash(state)

	return state, nil
}

// applyEvent applies an event to the state
func (sm *SnapshotManager) applyEvent(state *StateSnapshot, event *Event) error {
	switch event.Type {
	case EventNodeJoin:
		var data NodeJoinData
		if err := event.GetData(&data); err != nil {
			return err
		}
		state.Nodes[data.NodeID] = &NodeState{
			NodeID:     data.NodeID,
			PubKey:     data.NodePubKey,
			Reputation: data.InitialReputation,
			Status:     "active",
			JoinedAt:   event.Timestamp,
			SponsorID:  data.SponsorID,
			Guarantees: []string{},
		}

	case EventNodeLeave:
		var data NodeLeaveData
		if err := event.GetData(&data); err != nil {
			return err
		}
		if node, ok := state.Nodes[data.NodeID]; ok {
			node.Status = "left"
		}

	case EventReputationChange:
		var data ReputationChangeData
		if err := event.GetData(&data); err != nil {
			return err
		}
		if node, ok := state.Nodes[data.NodeID]; ok {
			node.Reputation = data.NewValue
		}

	case EventGuaranteeCreate:
		var data GuaranteeCreateData
		if err := event.GetData(&data); err != nil {
			return err
		}
		state.Guarantees[data.GuaranteeID] = &GuaranteeState{
			GuaranteeID:     data.GuaranteeID,
			SponsorID:       data.SponsorID,
			NewNodeID:       data.NewNodeID,
			GuaranteeAmount: data.GuaranteeAmount,
			LiabilityRatio:  data.LiabilityRatio,
			ValidUntil:      data.ValidUntil,
			Status:          "active",
			CreatedAt:       event.Timestamp,
		}
		// Add guarantee to sponsor's list
		if sponsor, ok := state.Nodes[data.SponsorID]; ok {
			sponsor.Guarantees = append(sponsor.Guarantees, data.GuaranteeID)
		}

	case EventGuaranteeExpire:
		var data GuaranteeExpireData
		if err := event.GetData(&data); err != nil {
			return err
		}
		if g, ok := state.Guarantees[data.GuaranteeID]; ok {
			g.Status = data.Reason
		}

	case EventViolation:
		var data ViolationData
		if err := event.GetData(&data); err != nil {
			return err
		}
		if node, ok := state.Nodes[data.NodeID]; ok {
			node.Reputation -= data.Penalty
			if node.Reputation < 0 {
				node.Reputation = 0
			}
		}

	case EventLiabilitySettle:
		var data LiabilitySettleData
		if err := event.GetData(&data); err != nil {
			return err
		}
		if sponsor, ok := state.Nodes[data.SponsorID]; ok {
			sponsor.Reputation -= data.SponsorPenalty
			if sponsor.Reputation < 0 {
				sponsor.Reputation = 0
			}
		}
		if g, ok := state.Guarantees[data.GuaranteeID]; ok {
			g.Status = "settled"
		}
	}

	return nil
}

// cloneSnapshot creates a deep copy of a snapshot
func (sm *SnapshotManager) cloneSnapshot(s *StateSnapshot) *StateSnapshot {
	clone := &StateSnapshot{
		Sequence:   s.Sequence,
		Timestamp:  s.Timestamp,
		Nodes:      make(map[string]*NodeState),
		Guarantees: make(map[string]*GuaranteeState),
		Hash:       s.Hash,
	}

	for k, v := range s.Nodes {
		nodeCopy := *v
		nodeCopy.Guarantees = make([]string, len(v.Guarantees))
		copy(nodeCopy.Guarantees, v.Guarantees)
		clone.Nodes[k] = &nodeCopy
	}

	for k, v := range s.Guarantees {
		gCopy := *v
		clone.Guarantees[k] = &gCopy
	}

	return clone
}

// computeSnapshotHash computes the hash of a snapshot
func (sm *SnapshotManager) computeSnapshotHash(s *StateSnapshot) string {
	// Simple hash based on sequence and state
	data := struct {
		Seq   uint64 `json:"seq"`
		Time  int64  `json:"time"`
		Nodes int    `json:"nodes"`
		Guarantees int `json:"guarantees"`
	}{
		Seq:   s.Sequence,
		Time:  s.Timestamp,
		Nodes: len(s.Nodes),
		Guarantees: len(s.Guarantees),
	}

	bytes, _ := json.Marshal(data)
	return fmt.Sprintf("%x", bytes)[:16]
}

// saveSnapshot saves a snapshot to disk
func (sm *SnapshotManager) saveSnapshot(s *StateSnapshot) error {
	snapshotDir := filepath.Join(sm.dataDir, "snapshots")
	filePath := filepath.Join(snapshotDir, fmt.Sprintf("snapshot_%d.json", s.Sequence))

	bytes, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, bytes, 0644)
}

// loadSnapshots loads all snapshots from disk
func (sm *SnapshotManager) loadSnapshots() error {
	snapshotDir := filepath.Join(sm.dataDir, "snapshots")

	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filePath := filepath.Join(snapshotDir, entry.Name())
		bytes, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var snapshot StateSnapshot
		if err := json.Unmarshal(bytes, &snapshot); err != nil {
			continue
		}

		sm.snapshots = append(sm.snapshots, &snapshot)
		if snapshot.Sequence > sm.latestSeq {
			sm.latestSeq = snapshot.Sequence
		}
	}

	return nil
}

// GetSnapshotCount returns the number of snapshots
func (sm *SnapshotManager) GetSnapshotCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.snapshots)
}

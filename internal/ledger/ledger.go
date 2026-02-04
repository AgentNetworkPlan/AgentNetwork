package ledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SignFunc is a function type for signing event data
type SignFunc func(data []byte) (string, error)

// VerifyFunc is a function type for verifying signatures
type VerifyFunc func(signerID string, data []byte, signature string) bool

// Ledger manages the event ledger (Event Sourcing)
type Ledger struct {
	dataDir string         // Data directory
	events  []*Event       // Event list
	index   map[string][]*Event // Node index: nodeID -> events
	typeIndex map[EventType][]*Event // Type index: eventType -> events
	lastSeq uint64         // Last sequence number
	lastHash string        // Last event hash
	
	signFunc   SignFunc   // Signing function
	verifyFunc VerifyFunc // Verification function
	
	mu sync.RWMutex
}

// NewLedger creates a new ledger
func NewLedger(dataDir string) (*Ledger, error) {
	l := &Ledger{
		dataDir:   dataDir,
		events:    make([]*Event, 0),
		index:     make(map[string][]*Event),
		typeIndex: make(map[EventType][]*Event),
		lastSeq:   0,
		lastHash:  "", // Genesis hash is empty
	}

	// Create data directory if not exists
	if dataDir != "" {
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}

		// Load existing events
		if err := l.load(); err != nil {
			// If file doesn't exist, that's OK
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to load ledger: %w", err)
			}
		}
	}

	return l, nil
}

// SetSignFunc sets the signing function
func (l *Ledger) SetSignFunc(fn SignFunc) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.signFunc = fn
}

// SetVerifyFunc sets the verification function
func (l *Ledger) SetVerifyFunc(fn VerifyFunc) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.verifyFunc = fn
}

// AppendEvent adds a new event to the ledger
func (l *Ledger) AppendEvent(eventType EventType, nodeID string, data interface{}, signerID string) (*Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Create event
	seq := l.lastSeq + 1
	event, err := NewEvent(seq, eventType, nodeID, data, signerID, l.lastHash)
	if err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}

	// Sign event if signing function is available
	if l.signFunc != nil {
		hashBytes := []byte(event.Hash)
		sig, err := l.signFunc(hashBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to sign event: %w", err)
		}
		event.Signature = sig
	}

	// Append to events list
	l.events = append(l.events, event)

	// Update indices
	l.index[nodeID] = append(l.index[nodeID], event)
	l.typeIndex[eventType] = append(l.typeIndex[eventType], event)

	// Update last sequence and hash
	l.lastSeq = seq
	l.lastHash = event.Hash

	// Persist to disk
	if l.dataDir != "" {
		if err := l.save(); err != nil {
			// Log error but don't fail - event is already in memory
			fmt.Printf("Warning: failed to persist event: %v\n", err)
		}
	}

	return event, nil
}

// AppendSignedEvent adds an externally signed event to the ledger
func (l *Ledger) AppendSignedEvent(event *Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Validate event
	if err := event.Validate(); err != nil {
		return fmt.Errorf("invalid event: %w", err)
	}

	// Verify sequence
	expectedSeq := l.lastSeq + 1
	if event.Sequence != expectedSeq {
		return fmt.Errorf("sequence mismatch: expected %d, got %d", expectedSeq, event.Sequence)
	}

	// Verify prev hash
	if event.PrevHash != l.lastHash {
		return fmt.Errorf("prev hash mismatch: expected %s, got %s", l.lastHash, event.PrevHash)
	}

	// Verify signature if verify function is available
	if l.verifyFunc != nil && event.Signature != "" {
		hashBytes := []byte(event.Hash)
		if !l.verifyFunc(event.SignerID, hashBytes, event.Signature) {
			return fmt.Errorf("invalid signature")
		}
	}

	// Append to events list
	l.events = append(l.events, event)

	// Update indices
	l.index[event.NodeID] = append(l.index[event.NodeID], event)
	l.typeIndex[event.Type] = append(l.typeIndex[event.Type], event)

	// Update last sequence and hash
	l.lastSeq = event.Sequence
	l.lastHash = event.Hash

	// Persist to disk
	if l.dataDir != "" {
		if err := l.save(); err != nil {
			fmt.Printf("Warning: failed to persist event: %v\n", err)
		}
	}

	return nil
}

// GetEvent returns an event by sequence number
func (l *Ledger) GetEvent(seq uint64) *Event {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if seq == 0 || seq > uint64(len(l.events)) {
		return nil
	}
	return l.events[seq-1] // seq is 1-based
}

// GetEvents returns events in a range [startSeq, endSeq]
func (l *Ledger) GetEvents(startSeq, endSeq uint64) []*Event {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if startSeq == 0 {
		startSeq = 1
	}
	if endSeq > l.lastSeq {
		endSeq = l.lastSeq
	}
	if startSeq > endSeq {
		return nil
	}

	result := make([]*Event, 0, endSeq-startSeq+1)
	for i := startSeq; i <= endSeq; i++ {
		result = append(result, l.events[i-1])
	}
	return result
}

// GetEventsByNode returns all events related to a node
func (l *Ledger) GetEventsByNode(nodeID string) []*Event {
	l.mu.RLock()
	defer l.mu.RUnlock()

	events := l.index[nodeID]
	if events == nil {
		return []*Event{}
	}

	// Return a copy
	result := make([]*Event, len(events))
	copy(result, events)
	return result
}

// GetEventsByType returns all events of a specific type
func (l *Ledger) GetEventsByType(eventType EventType) []*Event {
	l.mu.RLock()
	defer l.mu.RUnlock()

	events := l.typeIndex[eventType]
	if events == nil {
		return []*Event{}
	}

	// Return a copy
	result := make([]*Event, len(events))
	copy(result, events)
	return result
}

// GetRecentEvents returns the most recent N events
func (l *Ledger) GetRecentEvents(count int) []*Event {
	l.mu.RLock()
	defer l.mu.RUnlock()

	total := len(l.events)
	if count > total {
		count = total
	}

	result := make([]*Event, count)
	for i := 0; i < count; i++ {
		result[i] = l.events[total-count+i]
	}
	return result
}

// GetLastSequence returns the last sequence number
func (l *Ledger) GetLastSequence() uint64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lastSeq
}

// GetLastHash returns the hash of the last event
func (l *Ledger) GetLastHash() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lastHash
}

// EventCount returns the total number of events
func (l *Ledger) EventCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.events)
}

// VerifyChain verifies the integrity of the event chain
func (l *Ledger) VerifyChain() error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	prevHash := ""
	for i, event := range l.events {
		// Check sequence
		expectedSeq := uint64(i + 1)
		if event.Sequence != expectedSeq {
			return fmt.Errorf("sequence mismatch at index %d: expected %d, got %d",
				i, expectedSeq, event.Sequence)
		}

		// Check prev hash
		if event.PrevHash != prevHash {
			return fmt.Errorf("prev hash mismatch at seq %d", event.Sequence)
		}

		// Check hash
		expectedHash := event.ComputeHash()
		if event.Hash != expectedHash {
			return fmt.Errorf("hash mismatch at seq %d", event.Sequence)
		}

		prevHash = event.Hash
	}

	return nil
}

// QueryEvents queries events with filters
func (l *Ledger) QueryEvents(filters EventFilters) []*Event {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*Event, 0)
	for _, event := range l.events {
		if filters.Match(event) {
			result = append(result, event)
		}
	}
	return result
}

// EventFilters defines filters for querying events
type EventFilters struct {
	NodeID     string      // Filter by node ID
	Types      []EventType // Filter by event types
	SignerID   string      // Filter by signer ID
	StartTime  int64       // Start timestamp (inclusive)
	EndTime    int64       // End timestamp (inclusive)
	StartSeq   uint64      // Start sequence (inclusive)
	EndSeq     uint64      // End sequence (inclusive)
}

// Match checks if an event matches the filters
func (f *EventFilters) Match(event *Event) bool {
	if f.NodeID != "" && event.NodeID != f.NodeID {
		return false
	}
	if f.SignerID != "" && event.SignerID != f.SignerID {
		return false
	}
	if len(f.Types) > 0 {
		matched := false
		for _, t := range f.Types {
			if event.Type == t {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if f.StartTime > 0 && event.Timestamp < f.StartTime {
		return false
	}
	if f.EndTime > 0 && event.Timestamp > f.EndTime {
		return false
	}
	if f.StartSeq > 0 && event.Sequence < f.StartSeq {
		return false
	}
	if f.EndSeq > 0 && event.Sequence > f.EndSeq {
		return false
	}
	return true
}

// ledgerData is used for serialization
type ledgerData struct {
	Events   []*Event `json:"events"`
	LastSeq  uint64   `json:"last_seq"`
	LastHash string   `json:"last_hash"`
	SavedAt  int64    `json:"saved_at"`
}

// save persists the ledger to disk
func (l *Ledger) save() error {
	data := ledgerData{
		Events:   l.events,
		LastSeq:  l.lastSeq,
		LastHash: l.lastHash,
		SavedAt:  time.Now().Unix(),
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(l.dataDir, "ledger.json")
	return os.WriteFile(filePath, bytes, 0644)
}

// load loads the ledger from disk
func (l *Ledger) load() error {
	filePath := filepath.Join(l.dataDir, "ledger.json")
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var data ledgerData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return err
	}

	l.events = data.Events
	l.lastSeq = data.LastSeq
	l.lastHash = data.LastHash

	// Rebuild indices
	l.index = make(map[string][]*Event)
	l.typeIndex = make(map[EventType][]*Event)
	for _, event := range l.events {
		l.index[event.NodeID] = append(l.index[event.NodeID], event)
		l.typeIndex[event.Type] = append(l.typeIndex[event.Type], event)
	}

	return nil
}

// Reset clears all events (for testing)
func (l *Ledger) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.events = make([]*Event, 0)
	l.index = make(map[string][]*Event)
	l.typeIndex = make(map[EventType][]*Event)
	l.lastSeq = 0
	l.lastHash = ""
}

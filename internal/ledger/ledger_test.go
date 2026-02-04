package ledger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewEvent(t *testing.T) {
	data := NodeJoinData{
		NodeID:            "node1",
		NodePubKey:        "pubkey1",
		SponsorID:         "sponsor1",
		GuaranteeID:       "guarantee1",
		InitialReputation: 1.0,
	}

	event, err := NewEvent(1, EventNodeJoin, "node1", data, "sponsor1", "")
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	if event.Sequence != 1 {
		t.Errorf("Expected sequence 1, got %d", event.Sequence)
	}
	if event.Type != EventNodeJoin {
		t.Errorf("Expected type NODE_JOIN, got %s", event.Type)
	}
	if event.NodeID != "node1" {
		t.Errorf("Expected nodeID node1, got %s", event.NodeID)
	}
	if event.Hash == "" {
		t.Error("Expected hash to be set")
	}
}

func TestEventValidate(t *testing.T) {
	data := NodeJoinData{
		NodeID:            "node1",
		NodePubKey:        "pubkey1",
		SponsorID:         "sponsor1",
		GuaranteeID:       "guarantee1",
		InitialReputation: 1.0,
	}

	event, err := NewEvent(1, EventNodeJoin, "node1", data, "sponsor1", "")
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// Valid event should pass
	if err := event.Validate(); err != nil {
		t.Errorf("Valid event should pass validation: %v", err)
	}

	// Test invalid sequence
	badEvent := *event
	badEvent.Sequence = 0
	if err := badEvent.Validate(); err == nil {
		t.Error("Should fail with sequence 0")
	}

	// Test invalid hash
	badEvent = *event
	badEvent.Hash = "wrong"
	if err := badEvent.Validate(); err == nil {
		t.Error("Should fail with wrong hash")
	}
}

func TestEventGetData(t *testing.T) {
	data := NodeJoinData{
		NodeID:            "node1",
		NodePubKey:        "pubkey1",
		SponsorID:         "sponsor1",
		GuaranteeID:       "guarantee1",
		InitialReputation: 5.0,
	}

	event, err := NewEvent(1, EventNodeJoin, "node1", data, "sponsor1", "")
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	var extracted NodeJoinData
	if err := event.GetData(&extracted); err != nil {
		t.Fatalf("Failed to get data: %v", err)
	}

	if extracted.NodeID != data.NodeID {
		t.Errorf("Expected nodeID %s, got %s", data.NodeID, extracted.NodeID)
	}
	if extracted.InitialReputation != data.InitialReputation {
		t.Errorf("Expected reputation %f, got %f", data.InitialReputation, extracted.InitialReputation)
	}
}

func TestLedgerAppendAndGet(t *testing.T) {
	ledger, err := NewLedger("")
	if err != nil {
		t.Fatalf("Failed to create ledger: %v", err)
	}

	// Append first event
	data1 := NodeJoinData{NodeID: "node1", SponsorID: "genesis"}
	event1, err := ledger.AppendEvent(EventNodeJoin, "node1", data1, "genesis")
	if err != nil {
		t.Fatalf("Failed to append event: %v", err)
	}

	if event1.Sequence != 1 {
		t.Errorf("Expected sequence 1, got %d", event1.Sequence)
	}
	if event1.PrevHash != "" {
		t.Errorf("Expected empty prev hash for first event")
	}

	// Append second event
	data2 := NodeJoinData{NodeID: "node2", SponsorID: "node1"}
	event2, err := ledger.AppendEvent(EventNodeJoin, "node2", data2, "node1")
	if err != nil {
		t.Fatalf("Failed to append second event: %v", err)
	}

	if event2.Sequence != 2 {
		t.Errorf("Expected sequence 2, got %d", event2.Sequence)
	}
	if event2.PrevHash != event1.Hash {
		t.Errorf("PrevHash should point to first event")
	}

	// Get event
	retrieved := ledger.GetEvent(1)
	if retrieved == nil {
		t.Fatal("Failed to get event 1")
	}
	if retrieved.Sequence != 1 {
		t.Errorf("Expected sequence 1, got %d", retrieved.Sequence)
	}
}

func TestLedgerGetEventsByNode(t *testing.T) {
	ledger, _ := NewLedger("")

	// Add events for different nodes
	ledger.AppendEvent(EventNodeJoin, "node1", NodeJoinData{NodeID: "node1"}, "genesis")
	ledger.AppendEvent(EventReputationChange, "node1", ReputationChangeData{NodeID: "node1", Delta: 5}, "system")
	ledger.AppendEvent(EventNodeJoin, "node2", NodeJoinData{NodeID: "node2"}, "node1")

	// Get events by node
	events := ledger.GetEventsByNode("node1")
	if len(events) != 2 {
		t.Errorf("Expected 2 events for node1, got %d", len(events))
	}

	events = ledger.GetEventsByNode("node2")
	if len(events) != 1 {
		t.Errorf("Expected 1 event for node2, got %d", len(events))
	}
}

func TestLedgerGetEventsByType(t *testing.T) {
	ledger, _ := NewLedger("")

	ledger.AppendEvent(EventNodeJoin, "node1", NodeJoinData{NodeID: "node1"}, "genesis")
	ledger.AppendEvent(EventNodeJoin, "node2", NodeJoinData{NodeID: "node2"}, "node1")
	ledger.AppendEvent(EventReputationChange, "node1", ReputationChangeData{NodeID: "node1"}, "system")

	joinEvents := ledger.GetEventsByType(EventNodeJoin)
	if len(joinEvents) != 2 {
		t.Errorf("Expected 2 join events, got %d", len(joinEvents))
	}

	repEvents := ledger.GetEventsByType(EventReputationChange)
	if len(repEvents) != 1 {
		t.Errorf("Expected 1 reputation event, got %d", len(repEvents))
	}
}

func TestLedgerVerifyChain(t *testing.T) {
	ledger, _ := NewLedger("")

	// Add some events
	ledger.AppendEvent(EventNodeJoin, "node1", NodeJoinData{NodeID: "node1"}, "genesis")
	ledger.AppendEvent(EventNodeJoin, "node2", NodeJoinData{NodeID: "node2"}, "node1")
	ledger.AppendEvent(EventReputationChange, "node1", ReputationChangeData{NodeID: "node1"}, "system")

	// Verify chain
	if err := ledger.VerifyChain(); err != nil {
		t.Errorf("Chain verification failed: %v", err)
	}
}

func TestLedgerQueryEvents(t *testing.T) {
	ledger, _ := NewLedger("")

	ledger.AppendEvent(EventNodeJoin, "node1", NodeJoinData{NodeID: "node1"}, "genesis")
	ledger.AppendEvent(EventNodeJoin, "node2", NodeJoinData{NodeID: "node2"}, "node1")
	ledger.AppendEvent(EventReputationChange, "node1", ReputationChangeData{NodeID: "node1"}, "system")

	// Query by node
	filters := EventFilters{NodeID: "node1"}
	events := ledger.QueryEvents(filters)
	if len(events) != 2 {
		t.Errorf("Expected 2 events for node1, got %d", len(events))
	}

	// Query by type
	filters = EventFilters{Types: []EventType{EventNodeJoin}}
	events = ledger.QueryEvents(filters)
	if len(events) != 2 {
		t.Errorf("Expected 2 join events, got %d", len(events))
	}

	// Query by sequence range
	filters = EventFilters{StartSeq: 1, EndSeq: 2}
	events = ledger.QueryEvents(filters)
	if len(events) != 2 {
		t.Errorf("Expected 2 events in range, got %d", len(events))
	}
}

func TestLedgerPersistence(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "ledger_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "ledger")

	// Create ledger and add events
	ledger1, err := NewLedger(dataDir)
	if err != nil {
		t.Fatalf("Failed to create ledger: %v", err)
	}

	ledger1.AppendEvent(EventNodeJoin, "node1", NodeJoinData{NodeID: "node1"}, "genesis")
	ledger1.AppendEvent(EventNodeJoin, "node2", NodeJoinData{NodeID: "node2"}, "node1")

	// Create new ledger from same directory
	ledger2, err := NewLedger(dataDir)
	if err != nil {
		t.Fatalf("Failed to load ledger: %v", err)
	}

	if ledger2.EventCount() != 2 {
		t.Errorf("Expected 2 events after load, got %d", ledger2.EventCount())
	}

	if ledger2.GetLastSequence() != 2 {
		t.Errorf("Expected last sequence 2, got %d", ledger2.GetLastSequence())
	}
}

func TestLedgerGetRecentEvents(t *testing.T) {
	ledger, _ := NewLedger("")

	for i := 1; i <= 10; i++ {
		ledger.AppendEvent(EventNodeJoin, "node1", NodeJoinData{NodeID: "node1"}, "genesis")
	}

	// Get last 3
	recent := ledger.GetRecentEvents(3)
	if len(recent) != 3 {
		t.Errorf("Expected 3 recent events, got %d", len(recent))
	}
	if recent[0].Sequence != 8 {
		t.Errorf("Expected first recent to be seq 8, got %d", recent[0].Sequence)
	}

	// Request more than available
	recent = ledger.GetRecentEvents(20)
	if len(recent) != 10 {
		t.Errorf("Expected 10 events (all), got %d", len(recent))
	}
}

func TestLedgerReset(t *testing.T) {
	ledger, _ := NewLedger("")

	ledger.AppendEvent(EventNodeJoin, "node1", NodeJoinData{NodeID: "node1"}, "genesis")
	ledger.AppendEvent(EventNodeJoin, "node2", NodeJoinData{NodeID: "node2"}, "node1")

	if ledger.EventCount() != 2 {
		t.Errorf("Expected 2 events before reset")
	}

	ledger.Reset()

	if ledger.EventCount() != 0 {
		t.Errorf("Expected 0 events after reset, got %d", ledger.EventCount())
	}
	if ledger.GetLastSequence() != 0 {
		t.Errorf("Expected last sequence 0 after reset")
	}
}

package transfer

import (
	"os"
	"testing"
	"time"
)

func TestNewTransferManager(t *testing.T) {
	config := &TransferConfig{
		DataDir:          t.TempDir(),
		DefaultChunkSize: 64 * 1024,
	}

	tm := NewTransferManager(config)
	if tm == nil {
		t.Fatal("NewTransferManager returned nil")
	}
}

func TestCreateTransfer(t *testing.T) {
	tm := NewTransferManager(&TransferConfig{
		DataDir:                t.TempDir(),
		DefaultChunkSize:       1024,
		MaxConcurrentTransfers: 5,
		TransferTimeout:        30 * time.Minute,
	})

	req := &TransferRequest{
		SenderID:   "sender1",
		ReceiverID: "receiver1",
		FileHash:   "hash123",
		FileName:   "test.txt",
		FileSize:   10240, // 10KB
	}

	err := tm.CreateTransfer(req)
	if err != nil {
		t.Fatalf("CreateTransfer failed: %v", err)
	}

	if req.ID == "" {
		t.Error("Transfer ID should be generated")
	}

	if req.Status != TransferPending {
		t.Errorf("Status should be pending, got %s", req.Status)
	}

	// Check chunk calculation
	expectedChunks := 10 // 10KB / 1KB = 10
	if req.TotalChunks != expectedChunks {
		t.Errorf("Expected %d chunks, got %d", expectedChunks, req.TotalChunks)
	}
}

func TestTransferAcceptAndStart(t *testing.T) {
	tm := NewTransferManager(&TransferConfig{
		DataDir:                t.TempDir(),
		DefaultChunkSize:       1024,
		MaxConcurrentTransfers: 5,
	})

	req := &TransferRequest{
		SenderID:   "sender1",
		ReceiverID: "receiver1",
		FileHash:   "hash123",
		FileName:   "test.txt",
		FileSize:   5120,
	}

	tm.CreateTransfer(req)

	// Accept transfer
	err := tm.AcceptTransfer(req.ID, "receiver1", "sig")
	if err != nil {
		t.Errorf("AcceptTransfer failed: %v", err)
	}

	transfer, _ := tm.GetTransfer(req.ID)
	if transfer.Status != TransferAccepted {
		t.Errorf("Status should be accepted, got %s", transfer.Status)
	}

	// Unauthorized accept
	err = tm.AcceptTransfer(req.ID, "stranger", "sig")
	if err != ErrUnauthorized {
		t.Errorf("Expected unauthorized error, got %v", err)
	}

	// Start transfer
	err = tm.StartTransfer(req.ID, "sender1")
	if err != nil {
		t.Errorf("StartTransfer failed: %v", err)
	}

	transfer, _ = tm.GetTransfer(req.ID)
	if transfer.Status != TransferInProgress {
		t.Errorf("Status should be in_progress, got %s", transfer.Status)
	}
}

func TestChunkTransfer(t *testing.T) {
	tm := NewTransferManager(&TransferConfig{
		DataDir:                t.TempDir(),
		DefaultChunkSize:       1024,
		MaxConcurrentTransfers: 5,
	})

	req := &TransferRequest{
		SenderID:   "sender1",
		ReceiverID: "receiver1",
		FileHash:   "hash123",
		FileName:   "test.txt",
		FileSize:   3072, // 3 chunks
	}

	tm.CreateTransfer(req)
	tm.AcceptTransfer(req.ID, "receiver1", "sig")
	tm.StartTransfer(req.ID, "sender1")

	// Receive chunks
	for i := 0; i < 3; i++ {
		chunk := &TransferChunk{
			TransferID: req.ID,
			Index:      i,
			Data:       make([]byte, 1024),
			Hash:       "chunk_hash",
			Size:       1024,
		}

		err := tm.ReceiveChunk(chunk)
		if err != nil {
			t.Errorf("ReceiveChunk failed for index %d: %v", i, err)
		}
	}

	// Check completion
	transfer, _ := tm.GetTransfer(req.ID)
	if transfer.Status != TransferCompleted {
		t.Errorf("Status should be completed, got %s", transfer.Status)
	}

	if transfer.Progress != 1.0 {
		t.Errorf("Progress should be 1.0, got %.2f", transfer.Progress)
	}
}

func TestChunkProgress(t *testing.T) {
	tm := NewTransferManager(&TransferConfig{
		DataDir:                t.TempDir(),
		DefaultChunkSize:       1024,
		MaxConcurrentTransfers: 5,
	})

	req := &TransferRequest{
		SenderID:   "sender1",
		ReceiverID: "receiver1",
		FileSize:   5120, // 5 chunks
	}

	tm.CreateTransfer(req)
	tm.AcceptTransfer(req.ID, "receiver1", "sig")
	tm.StartTransfer(req.ID, "sender1")

	// Receive 2 out of 5 chunks
	for i := 0; i < 2; i++ {
		chunk := &TransferChunk{
			TransferID: req.ID,
			Index:      i,
			Size:       1024,
		}
		tm.ReceiveChunk(chunk)
	}

	transfer, _ := tm.GetTransfer(req.ID)
	expectedProgress := 2.0 / 5.0
	if transfer.Progress != expectedProgress {
		t.Errorf("Expected progress %.2f, got %.2f", expectedProgress, transfer.Progress)
	}

	// Check missing chunks
	missing, err := tm.GetMissingChunks(req.ID)
	if err != nil {
		t.Errorf("GetMissingChunks failed: %v", err)
	}

	if len(missing) != 3 {
		t.Errorf("Expected 3 missing chunks, got %d", len(missing))
	}
}

func TestInvalidChunk(t *testing.T) {
	tm := NewTransferManager(&TransferConfig{
		DataDir:                t.TempDir(),
		DefaultChunkSize:       1024,
		MaxConcurrentTransfers: 5,
	})

	req := &TransferRequest{
		SenderID:   "sender1",
		ReceiverID: "receiver1",
		FileSize:   3072, // 3 chunks (0, 1, 2)
	}

	tm.CreateTransfer(req)
	tm.AcceptTransfer(req.ID, "receiver1", "sig")
	tm.StartTransfer(req.ID, "sender1")

	// Invalid chunk index
	chunk := &TransferChunk{
		TransferID: req.ID,
		Index:      10, // Out of range
	}

	err := tm.ReceiveChunk(chunk)
	if err == nil {
		t.Error("Should fail with invalid chunk index")
	}

	// Negative index
	chunk.Index = -1
	err = tm.ReceiveChunk(chunk)
	if err == nil {
		t.Error("Should fail with negative chunk index")
	}
}

func TestTransferPauseAndResume(t *testing.T) {
	tm := NewTransferManager(&TransferConfig{
		DataDir:                t.TempDir(),
		DefaultChunkSize:       1024,
		MaxConcurrentTransfers: 5,
	})

	req := &TransferRequest{
		SenderID:   "sender1",
		ReceiverID: "receiver1",
		FileSize:   5120,
	}

	tm.CreateTransfer(req)
	tm.AcceptTransfer(req.ID, "receiver1", "sig")
	tm.StartTransfer(req.ID, "sender1")

	// Pause
	err := tm.PauseTransfer(req.ID, "sender1")
	if err != nil {
		t.Errorf("PauseTransfer failed: %v", err)
	}

	transfer, _ := tm.GetTransfer(req.ID)
	if transfer.Status != TransferPaused {
		t.Errorf("Status should be paused, got %s", transfer.Status)
	}

	// Resume
	err = tm.ResumeTransfer(req.ID, "sender1")
	if err != nil {
		t.Errorf("ResumeTransfer failed: %v", err)
	}

	transfer, _ = tm.GetTransfer(req.ID)
	if transfer.Status != TransferInProgress {
		t.Errorf("Status should be in_progress, got %s", transfer.Status)
	}
}

func TestTransferCancel(t *testing.T) {
	tm := NewTransferManager(&TransferConfig{
		DataDir:                t.TempDir(),
		DefaultChunkSize:       1024,
		MaxConcurrentTransfers: 5,
	})

	req := &TransferRequest{
		SenderID:   "sender1",
		ReceiverID: "receiver1",
		FileSize:   5120,
	}

	tm.CreateTransfer(req)
	tm.AcceptTransfer(req.ID, "receiver1", "sig")

	// Cancel
	err := tm.CancelTransfer(req.ID, "sender1", "No longer needed")
	if err != nil {
		t.Errorf("CancelTransfer failed: %v", err)
	}

	transfer, _ := tm.GetTransfer(req.ID)
	if transfer.Status != TransferCancelled {
		t.Errorf("Status should be cancelled, got %s", transfer.Status)
	}

	// Cannot cancel completed or already cancelled
	err = tm.CancelTransfer(req.ID, "sender1", "Again")
	if err == nil {
		t.Error("Should not be able to cancel already cancelled transfer")
	}
}

func TestTransferQueries(t *testing.T) {
	tm := NewTransferManager(&TransferConfig{
		DataDir:                t.TempDir(),
		DefaultChunkSize:       1024,
		MaxConcurrentTransfers: 10,
	})

	// Create multiple transfers
	req1 := &TransferRequest{
		SenderID:   "sender1",
		ReceiverID: "receiver1",
		FileSize:   1024,
	}
	req2 := &TransferRequest{
		SenderID:   "sender1",
		ReceiverID: "receiver2",
		FileSize:   2048,
	}
	req3 := &TransferRequest{
		SenderID:   "sender2",
		ReceiverID: "receiver1",
		FileSize:   3072,
	}

	tm.CreateTransfer(req1)
	tm.CreateTransfer(req2)
	tm.CreateTransfer(req3)

	// Query by sender
	transfers := tm.GetTransfersByNode("sender1")
	if len(transfers) != 2 {
		t.Errorf("Expected 2 transfers for sender1, got %d", len(transfers))
	}

	// Query by receiver
	transfers = tm.GetTransfersByNode("receiver1")
	if len(transfers) != 2 {
		t.Errorf("Expected 2 transfers for receiver1, got %d", len(transfers))
	}
}

func TestTransferCheckpoint(t *testing.T) {
	tm := NewTransferManager(&TransferConfig{
		DataDir:                t.TempDir(),
		DefaultChunkSize:       1024,
		MaxConcurrentTransfers: 5,
	})

	req := &TransferRequest{
		SenderID:   "sender1",
		ReceiverID: "receiver1",
		FileSize:   5120, // 5 chunks
	}

	tm.CreateTransfer(req)
	tm.AcceptTransfer(req.ID, "receiver1", "sig")
	tm.StartTransfer(req.ID, "sender1")

	// Receive some chunks
	for i := 0; i < 3; i++ {
		chunk := &TransferChunk{
			TransferID: req.ID,
			Index:      i,
			Size:       1024,
		}
		tm.ReceiveChunk(chunk)
	}

	// Get checkpoint
	checkpoint, err := tm.GetCheckpoint(req.ID)
	if err != nil {
		t.Errorf("GetCheckpoint failed: %v", err)
	}

	if checkpoint.LastChunkIndex != 2 {
		t.Errorf("Expected last chunk index 2, got %d", checkpoint.LastChunkIndex)
	}

	if checkpoint.BytesReceived != 3072 {
		t.Errorf("Expected 3072 bytes received, got %d", checkpoint.BytesReceived)
	}
}

func TestBandwidthCheck(t *testing.T) {
	tm := NewTransferManager(&TransferConfig{
		DataDir:             t.TempDir(),
		DefaultChunkSize:    1024,
		MaxBandwidthPerNode: 1000000, // 1MB
	})

	// Should be able to transfer initially
	canTransfer := tm.CheckBandwidth("node1", 500000)
	if !canTransfer {
		t.Error("Should be able to transfer within bandwidth limit")
	}
}

func TestTransferStatistics(t *testing.T) {
	tm := NewTransferManager(&TransferConfig{
		DataDir:                t.TempDir(),
		DefaultChunkSize:       1024,
		MaxConcurrentTransfers: 5,
	})

	// Create and complete some transfers
	req1 := &TransferRequest{
		SenderID:   "sender1",
		ReceiverID: "receiver1",
		FileSize:   3072,
	}

	tm.CreateTransfer(req1)
	tm.AcceptTransfer(req1.ID, "receiver1", "sig")
	tm.StartTransfer(req1.ID, "sender1")

	// Complete transfer
	for i := 0; i < 3; i++ {
		chunk := &TransferChunk{
			TransferID: req1.ID,
			Index:      i,
			Size:       1024,
		}
		tm.ReceiveChunk(chunk)
	}

	stats := tm.GetStatistics()
	if stats.TotalTransfers != 1 {
		t.Errorf("Expected 1 transfer, got %d", stats.TotalTransfers)
	}

	if stats.CompletedBytes != 3072 {
		t.Errorf("Expected 3072 completed bytes, got %d", stats.CompletedBytes)
	}

	if stats.ByStatus[TransferCompleted] != 1 {
		t.Errorf("Expected 1 completed transfer, got %d", stats.ByStatus[TransferCompleted])
	}
}

func TestConcurrentTransferLimit(t *testing.T) {
	tm := NewTransferManager(&TransferConfig{
		DataDir:                t.TempDir(),
		DefaultChunkSize:       1024,
		MaxConcurrentTransfers: 2,
	})

	// Create transfers at limit
	for i := 0; i < 2; i++ {
		req := &TransferRequest{
			SenderID:   "sender1",
			ReceiverID: "receiver1",
			FileSize:   1024,
		}
		err := tm.CreateTransfer(req)
		if err != nil {
			t.Errorf("CreateTransfer %d failed: %v", i, err)
		}
		tm.AcceptTransfer(req.ID, "receiver1", "sig")
		tm.StartTransfer(req.ID, "sender1")
	}

	// Should fail at limit
	req := &TransferRequest{
		SenderID:   "sender1",
		ReceiverID: "receiver2",
		FileSize:   1024,
	}
	err := tm.CreateTransfer(req)
	if err == nil {
		t.Error("Should fail when concurrent transfer limit is exceeded")
	}
}

func TestTransferPersistence(t *testing.T) {
	tempDir := t.TempDir()
	config := &TransferConfig{
		DataDir:                tempDir,
		DefaultChunkSize:       1024,
		MaxConcurrentTransfers: 5,
	}

	// Create transfer
	tm1 := NewTransferManager(config)
	req := &TransferRequest{
		SenderID:   "sender1",
		ReceiverID: "receiver1",
		FileSize:   5120,
		FileName:   "test.txt",
	}
	tm1.CreateTransfer(req)
	transferID := req.ID

	// Load in new manager
	tm2 := NewTransferManager(config)
	loadedTransfer, err := tm2.GetTransfer(transferID)
	if err != nil {
		t.Fatalf("Failed to load transfer: %v", err)
	}

	if loadedTransfer.FileName != "test.txt" {
		t.Errorf("Expected file name 'test.txt', got '%s'", loadedTransfer.FileName)
	}

	os.RemoveAll(tempDir)
}

func TestDuplicateChunk(t *testing.T) {
	tm := NewTransferManager(&TransferConfig{
		DataDir:                t.TempDir(),
		DefaultChunkSize:       1024,
		MaxConcurrentTransfers: 5,
	})

	req := &TransferRequest{
		SenderID:   "sender1",
		ReceiverID: "receiver1",
		FileSize:   3072,
	}

	tm.CreateTransfer(req)
	tm.AcceptTransfer(req.ID, "receiver1", "sig")
	tm.StartTransfer(req.ID, "sender1")

	chunk := &TransferChunk{
		TransferID: req.ID,
		Index:      0,
		Size:       1024,
	}

	// First receive
	err := tm.ReceiveChunk(chunk)
	if err != nil {
		t.Errorf("First ReceiveChunk failed: %v", err)
	}

	// Duplicate receive (should be ignored)
	err = tm.ReceiveChunk(chunk)
	if err != nil {
		t.Errorf("Duplicate chunk should be ignored, not error: %v", err)
	}

	// Progress should not double count
	transfer, _ := tm.GetTransfer(req.ID)
	expectedProgress := 1.0 / 3.0
	if transfer.Progress != expectedProgress {
		t.Errorf("Progress should be %.4f, got %.4f", expectedProgress, transfer.Progress)
	}
}

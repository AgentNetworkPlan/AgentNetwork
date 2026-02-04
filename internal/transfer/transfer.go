// Package transfer 提供文件传输功能
// Task 27: 委托任务与文件传输
package transfer

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrTransferNotFound   = errors.New("transfer not found")
	ErrTransferExpired    = errors.New("transfer expired")
	ErrInvalidChunk       = errors.New("invalid chunk")
	ErrChunkMissing       = errors.New("chunk missing")
	ErrTransferInProgress = errors.New("transfer already in progress")
	ErrBandwidthExceeded  = errors.New("bandwidth limit exceeded")
	ErrUnauthorized       = errors.New("unauthorized")
)

// TransferStatus 传输状态
type TransferStatus string

const (
	TransferPending    TransferStatus = "pending"     // 等待接受
	TransferAccepted   TransferStatus = "accepted"    // 已接受
	TransferInProgress TransferStatus = "in_progress" // 传输中
	TransferPaused     TransferStatus = "paused"      // 已暂停
	TransferCompleted  TransferStatus = "completed"   // 已完成
	TransferFailed     TransferStatus = "failed"      // 失败
	TransferCancelled  TransferStatus = "cancelled"   // 已取消
)

// TransferRequest 文件传输请求
type TransferRequest struct {
	ID     string `json:"id"`
	TaskID string `json:"task_id"` // 关联的任务ID（可选）

	// 参与方
	SenderID   string `json:"sender_id"`
	ReceiverID string `json:"receiver_id"`

	// 文件信息
	FileHash    string `json:"file_hash"`    // 文件哈希
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`    // 字节
	ChunkSize   int64  `json:"chunk_size"`   // 分片大小
	TotalChunks int    `json:"total_chunks"`

	// 流量控制
	MaxBandwidth int64 `json:"max_bandwidth"` // 最大带宽(bytes/s)

	// 押金（防止滥用）
	SenderDeposit   float64 `json:"sender_deposit"`
	ReceiverDeposit float64 `json:"receiver_deposit"`

	// 状态
	Status   TransferStatus `json:"status"`
	Progress float64        `json:"progress"` // 0-1

	// 时间
	CreatedAt   int64 `json:"created_at"`
	AcceptedAt  int64 `json:"accepted_at"`
	CompletedAt int64 `json:"completed_at"`
	ExpiresAt   int64 `json:"expires_at"`

	// 签名
	SenderSig   string `json:"sender_sig"`
	ReceiverSig string `json:"receiver_sig"`
}

// TransferChunk 传输分片
type TransferChunk struct {
	TransferID string `json:"transfer_id"`
	Index      int    `json:"index"`
	Data       []byte `json:"data"`
	Hash       string `json:"hash"` // 分片哈希
	Size       int    `json:"size"`
	Signature  string `json:"signature"`
	SentAt     int64  `json:"sent_at"`
	AckedAt    int64  `json:"acked_at"` // 确认时间
}

// TransferCheckpoint 传输检查点（断点续传）
type TransferCheckpoint struct {
	TransferID     string `json:"transfer_id"`
	LastChunkIndex int    `json:"last_chunk_index"`
	ReceivedChunks []bool `json:"received_chunks"` // 位图：哪些分片已收到
	PartialHash    string `json:"partial_hash"`    // 已收到部分的哈希
	BytesReceived  int64  `json:"bytes_received"`
	SavedAt        int64  `json:"saved_at"`
}

// TransferConfig 传输配置
type TransferConfig struct {
	DataDir            string        // 数据目录
	DefaultChunkSize   int64         // 默认分片大小
	MaxBandwidthPerNode int64         // 每节点最大带宽
	MaxConcurrentTransfers int        // 最大并发传输数
	TransferTimeout    time.Duration // 传输超时
	ChunkRetryLimit    int           // 分片重试次数
	CheckpointInterval time.Duration // 检查点间隔
}

// DefaultTransferConfig 返回默认配置
func DefaultTransferConfig() *TransferConfig {
	return &TransferConfig{
		DataDir:             "data/transfer",
		DefaultChunkSize:    64 * 1024, // 64KB
		MaxBandwidthPerNode: 10 * 1024 * 1024, // 10MB/s
		MaxConcurrentTransfers: 5,
		TransferTimeout:     30 * time.Minute,
		ChunkRetryLimit:     3,
		CheckpointInterval:  30 * time.Second,
	}
}

// TransferManager 传输管理器
type TransferManager struct {
	mu     sync.RWMutex
	config *TransferConfig

	// 传输记录
	transfers map[string]*TransferRequest // transferID -> transfer

	// 索引
	transfersBySender   map[string][]string // senderID -> []transferID
	transfersByReceiver map[string][]string // receiverID -> []transferID
	transfersByTask     map[string]string   // taskID -> transferID
	transfersByStatus   map[TransferStatus][]string

	// 分片跟踪
	chunkStatus map[string][]bool // transferID -> 分片接收状态

	// 检查点
	checkpoints map[string]*TransferCheckpoint // transferID -> checkpoint

	// 带宽控制
	bandwidthUsage map[string]*BandwidthRecord // nodeID -> usage
}

// BandwidthRecord 带宽使用记录
type BandwidthRecord struct {
	NodeID      string
	BytesSent   int64
	BytesRecv   int64
	LastReset   time.Time
	ActiveCount int // 活跃传输数
}

// NewTransferManager 创建传输管理器
func NewTransferManager(config *TransferConfig) *TransferManager {
	if config == nil {
		config = DefaultTransferConfig()
	}

	tm := &TransferManager{
		config:              config,
		transfers:           make(map[string]*TransferRequest),
		transfersBySender:   make(map[string][]string),
		transfersByReceiver: make(map[string][]string),
		transfersByTask:     make(map[string]string),
		transfersByStatus:   make(map[TransferStatus][]string),
		chunkStatus:         make(map[string][]bool),
		checkpoints:         make(map[string]*TransferCheckpoint),
		bandwidthUsage:      make(map[string]*BandwidthRecord),
	}

	tm.load()
	return tm
}

// CreateTransfer 创建传输请求
func (tm *TransferManager) CreateTransfer(req *TransferRequest) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 检查并发限制
	senderActive := tm.getActiveCount(req.SenderID)
	if senderActive >= tm.config.MaxConcurrentTransfers {
		return fmt.Errorf("sender has too many active transfers: %d", senderActive)
	}

	// 生成ID
	if req.ID == "" {
		req.ID = tm.generateID()
	}

	// 计算分片
	if req.ChunkSize == 0 {
		req.ChunkSize = tm.config.DefaultChunkSize
	}
	req.TotalChunks = int((req.FileSize + req.ChunkSize - 1) / req.ChunkSize)

	// 设置默认值
	req.CreatedAt = time.Now().Unix()
	req.Status = TransferPending
	req.Progress = 0

	if req.ExpiresAt == 0 {
		req.ExpiresAt = time.Now().Add(tm.config.TransferTimeout).Unix()
	}

	// 存储
	tm.transfers[req.ID] = req
	tm.addToIndex(req)

	// 初始化分片状态
	tm.chunkStatus[req.ID] = make([]bool, req.TotalChunks)

	tm.save()
	return nil
}

// AcceptTransfer 接受传输请求
func (tm *TransferManager) AcceptTransfer(transferID, receiverID, signature string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	transfer, exists := tm.transfers[transferID]
	if !exists {
		return ErrTransferNotFound
	}

	if transfer.ReceiverID != receiverID {
		return ErrUnauthorized
	}

	if transfer.Status != TransferPending {
		return fmt.Errorf("cannot accept: transfer status is %s", transfer.Status)
	}

	// 检查并发限制
	receiverActive := tm.getActiveCount(receiverID)
	if receiverActive >= tm.config.MaxConcurrentTransfers {
		return fmt.Errorf("receiver has too many active transfers: %d", receiverActive)
	}

	transfer.Status = TransferAccepted
	transfer.AcceptedAt = time.Now().Unix()
	transfer.ReceiverSig = signature

	tm.updateStatusIndex(transferID, TransferPending, TransferAccepted)
	tm.save()

	return nil
}

// StartTransfer 开始传输
func (tm *TransferManager) StartTransfer(transferID, senderID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	transfer, exists := tm.transfers[transferID]
	if !exists {
		return ErrTransferNotFound
	}

	if transfer.SenderID != senderID {
		return ErrUnauthorized
	}

	if transfer.Status != TransferAccepted {
		return fmt.Errorf("cannot start: transfer status is %s", transfer.Status)
	}

	oldStatus := transfer.Status
	transfer.Status = TransferInProgress

	// 更新带宽记录
	tm.updateBandwidth(senderID, true)
	tm.updateBandwidth(transfer.ReceiverID, true)

	tm.updateStatusIndex(transferID, oldStatus, TransferInProgress)
	tm.save()

	return nil
}

// ReceiveChunk 接收分片
func (tm *TransferManager) ReceiveChunk(chunk *TransferChunk) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	transfer, exists := tm.transfers[chunk.TransferID]
	if !exists {
		return ErrTransferNotFound
	}

	if transfer.Status != TransferInProgress {
		return fmt.Errorf("transfer not in progress: %s", transfer.Status)
	}

	// 验证分片索引
	if chunk.Index < 0 || chunk.Index >= transfer.TotalChunks {
		return fmt.Errorf("%w: index %d out of range [0, %d)", ErrInvalidChunk, chunk.Index, transfer.TotalChunks)
	}

	// 标记分片已接收
	chunkStatus := tm.chunkStatus[chunk.TransferID]
	if chunkStatus[chunk.Index] {
		// 重复分片，忽略
		return nil
	}

	chunkStatus[chunk.Index] = true
	chunk.AckedAt = time.Now().Unix()

	// 更新进度
	receivedCount := 0
	for _, received := range chunkStatus {
		if received {
			receivedCount++
		}
	}
	transfer.Progress = float64(receivedCount) / float64(transfer.TotalChunks)

	// 更新检查点
	tm.updateCheckpoint(chunk.TransferID, chunk.Index, int64(chunk.Size))

	// 检查是否完成
	if receivedCount == transfer.TotalChunks {
		transfer.Status = TransferCompleted
		transfer.CompletedAt = time.Now().Unix()
		transfer.Progress = 1.0

		// 释放带宽配额
		tm.updateBandwidth(transfer.SenderID, false)
		tm.updateBandwidth(transfer.ReceiverID, false)

		tm.updateStatusIndex(chunk.TransferID, TransferInProgress, TransferCompleted)
	}

	tm.save()
	return nil
}

// GetMissingChunks 获取缺失的分片索引
func (tm *TransferManager) GetMissingChunks(transferID string) ([]int, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	chunkStatus, exists := tm.chunkStatus[transferID]
	if !exists {
		return nil, ErrTransferNotFound
	}

	var missing []int
	for i, received := range chunkStatus {
		if !received {
			missing = append(missing, i)
		}
	}

	return missing, nil
}

// PauseTransfer 暂停传输
func (tm *TransferManager) PauseTransfer(transferID, nodeID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	transfer, exists := tm.transfers[transferID]
	if !exists {
		return ErrTransferNotFound
	}

	if transfer.SenderID != nodeID && transfer.ReceiverID != nodeID {
		return ErrUnauthorized
	}

	if transfer.Status != TransferInProgress {
		return fmt.Errorf("cannot pause: transfer status is %s", transfer.Status)
	}

	oldStatus := transfer.Status
	transfer.Status = TransferPaused

	// 保存检查点
	tm.saveCheckpoint(transferID)

	tm.updateStatusIndex(transferID, oldStatus, TransferPaused)
	tm.save()

	return nil
}

// ResumeTransfer 恢复传输
func (tm *TransferManager) ResumeTransfer(transferID, nodeID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	transfer, exists := tm.transfers[transferID]
	if !exists {
		return ErrTransferNotFound
	}

	if transfer.SenderID != nodeID && transfer.ReceiverID != nodeID {
		return ErrUnauthorized
	}

	if transfer.Status != TransferPaused {
		return fmt.Errorf("cannot resume: transfer status is %s", transfer.Status)
	}

	oldStatus := transfer.Status
	transfer.Status = TransferInProgress

	tm.updateStatusIndex(transferID, oldStatus, TransferInProgress)
	tm.save()

	return nil
}

// CancelTransfer 取消传输
func (tm *TransferManager) CancelTransfer(transferID, nodeID, reason string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	transfer, exists := tm.transfers[transferID]
	if !exists {
		return ErrTransferNotFound
	}

	if transfer.SenderID != nodeID && transfer.ReceiverID != nodeID {
		return ErrUnauthorized
	}

	if transfer.Status == TransferCompleted || transfer.Status == TransferCancelled {
		return fmt.Errorf("cannot cancel: transfer status is %s", transfer.Status)
	}

	oldStatus := transfer.Status
	transfer.Status = TransferCancelled

	// 释放带宽配额
	if oldStatus == TransferInProgress {
		tm.updateBandwidth(transfer.SenderID, false)
		tm.updateBandwidth(transfer.ReceiverID, false)
	}

	tm.updateStatusIndex(transferID, oldStatus, TransferCancelled)
	tm.save()

	return nil
}

// GetTransfer 获取传输信息
func (tm *TransferManager) GetTransfer(transferID string) (*TransferRequest, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	transfer, exists := tm.transfers[transferID]
	if !exists {
		return nil, ErrTransferNotFound
	}
	return transfer, nil
}

// GetTransfersByNode 获取节点的所有传输
func (tm *TransferManager) GetTransfersByNode(nodeID string) []*TransferRequest {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var transfers []*TransferRequest

	// 作为发送方
	for _, id := range tm.transfersBySender[nodeID] {
		if t, exists := tm.transfers[id]; exists {
			transfers = append(transfers, t)
		}
	}

	// 作为接收方
	for _, id := range tm.transfersByReceiver[nodeID] {
		if t, exists := tm.transfers[id]; exists {
			transfers = append(transfers, t)
		}
	}

	return transfers
}

// GetCheckpoint 获取检查点
func (tm *TransferManager) GetCheckpoint(transferID string) (*TransferCheckpoint, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	checkpoint, exists := tm.checkpoints[transferID]
	if !exists {
		return nil, errors.New("checkpoint not found")
	}
	return checkpoint, nil
}

// CheckBandwidth 检查带宽是否可用
func (tm *TransferManager) CheckBandwidth(nodeID string, bytes int64) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	record, exists := tm.bandwidthUsage[nodeID]
	if !exists {
		return true
	}

	// 检查是否需要重置（每小时重置）
	if time.Since(record.LastReset) > time.Hour {
		return true
	}

	return record.BytesSent+record.BytesRecv+bytes <= tm.config.MaxBandwidthPerNode*3600
}

// GetStatistics 获取统计信息
func (tm *TransferManager) GetStatistics() *TransferStatistics {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	stats := &TransferStatistics{
		TotalTransfers: len(tm.transfers),
		ByStatus:       make(map[TransferStatus]int),
		TotalBytes:     0,
		CompletedBytes: 0,
	}

	for _, transfer := range tm.transfers {
		stats.ByStatus[transfer.Status]++
		stats.TotalBytes += transfer.FileSize

		if transfer.Status == TransferCompleted {
			stats.CompletedBytes += transfer.FileSize
		}
	}

	return stats
}

// TransferStatistics 传输统计
type TransferStatistics struct {
	TotalTransfers int
	ByStatus       map[TransferStatus]int
	TotalBytes     int64
	CompletedBytes int64
}

// 内部方法

func (tm *TransferManager) generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return "transfer_" + hex.EncodeToString(bytes)
}

func (tm *TransferManager) addToIndex(req *TransferRequest) {
	tm.transfersBySender[req.SenderID] = append(tm.transfersBySender[req.SenderID], req.ID)
	tm.transfersByReceiver[req.ReceiverID] = append(tm.transfersByReceiver[req.ReceiverID], req.ID)
	tm.transfersByStatus[req.Status] = append(tm.transfersByStatus[req.Status], req.ID)

	if req.TaskID != "" {
		tm.transfersByTask[req.TaskID] = req.ID
	}
}

func (tm *TransferManager) updateStatusIndex(transferID string, oldStatus, newStatus TransferStatus) {
	// 从旧状态列表中移除
	oldList := tm.transfersByStatus[oldStatus]
	for i, id := range oldList {
		if id == transferID {
			tm.transfersByStatus[oldStatus] = append(oldList[:i], oldList[i+1:]...)
			break
		}
	}

	// 添加到新状态列表
	tm.transfersByStatus[newStatus] = append(tm.transfersByStatus[newStatus], transferID)
}

func (tm *TransferManager) getActiveCount(nodeID string) int {
	count := 0
	for _, id := range tm.transfersBySender[nodeID] {
		if t, exists := tm.transfers[id]; exists {
			if t.Status == TransferInProgress || t.Status == TransferAccepted {
				count++
			}
		}
	}
	for _, id := range tm.transfersByReceiver[nodeID] {
		if t, exists := tm.transfers[id]; exists {
			if t.Status == TransferInProgress || t.Status == TransferAccepted {
				count++
			}
		}
	}
	return count
}

func (tm *TransferManager) updateBandwidth(nodeID string, increment bool) {
	record, exists := tm.bandwidthUsage[nodeID]
	if !exists {
		record = &BandwidthRecord{
			NodeID:    nodeID,
			LastReset: time.Now(),
		}
		tm.bandwidthUsage[nodeID] = record
	}

	if increment {
		record.ActiveCount++
	} else {
		if record.ActiveCount > 0 {
			record.ActiveCount--
		}
	}
}

func (tm *TransferManager) updateCheckpoint(transferID string, chunkIndex int, bytes int64) {
	checkpoint, exists := tm.checkpoints[transferID]
	if !exists {
		checkpoint = &TransferCheckpoint{
			TransferID:     transferID,
			LastChunkIndex: -1,
			ReceivedChunks: tm.chunkStatus[transferID],
		}
		tm.checkpoints[transferID] = checkpoint
	}

	if chunkIndex > checkpoint.LastChunkIndex {
		checkpoint.LastChunkIndex = chunkIndex
	}
	checkpoint.BytesReceived += bytes
	checkpoint.SavedAt = time.Now().Unix()
}

func (tm *TransferManager) saveCheckpoint(transferID string) {
	if checkpoint, exists := tm.checkpoints[transferID]; exists {
		checkpoint.SavedAt = time.Now().Unix()
	}
}

func (tm *TransferManager) load() {
	filePath := filepath.Join(tm.config.DataDir, "transfers.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	var stored struct {
		Transfers   map[string]*TransferRequest    `json:"transfers"`
		ChunkStatus map[string][]bool              `json:"chunk_status"`
		Checkpoints map[string]*TransferCheckpoint `json:"checkpoints"`
	}

	if err := json.Unmarshal(data, &stored); err != nil {
		return
	}

	if stored.Transfers != nil {
		tm.transfers = stored.Transfers
		for _, t := range tm.transfers {
			tm.addToIndex(t)
		}
	}

	if stored.ChunkStatus != nil {
		tm.chunkStatus = stored.ChunkStatus
	}

	if stored.Checkpoints != nil {
		tm.checkpoints = stored.Checkpoints
	}
}

func (tm *TransferManager) save() {
	if err := os.MkdirAll(tm.config.DataDir, 0755); err != nil {
		return
	}

	stored := struct {
		Transfers   map[string]*TransferRequest    `json:"transfers"`
		ChunkStatus map[string][]bool              `json:"chunk_status"`
		Checkpoints map[string]*TransferCheckpoint `json:"checkpoints"`
	}{
		Transfers:   tm.transfers,
		ChunkStatus: tm.chunkStatus,
		Checkpoints: tm.checkpoints,
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return
	}

	filePath := filepath.Join(tm.config.DataDir, "transfers.json")
	os.WriteFile(filePath, data, 0644)
}

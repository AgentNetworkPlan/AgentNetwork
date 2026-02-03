// Package storage 提供节点数据持久化存储功能
// 支持配置、邻居、任务、声誉、指责、消息等数据的存储和加载
package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// 错误定义
var (
	ErrNotInitialized = errors.New("storage not initialized")
	ErrKeyNotFound    = errors.New("key not found")
	ErrInvalidData    = errors.New("invalid data format")
)

// Config 存储配置
type Config struct {
	DataDir      string        // 数据目录
	SyncInterval time.Duration // 同步间隔
	BackupEnabled bool         // 是否启用备份
	MaxBackups   int           // 最大备份数量
}

// DefaultConfig 返回默认配置
func DefaultConfig(dataDir string) *Config {
	return &Config{
		DataDir:       dataDir,
		SyncInterval:  5 * time.Second,
		BackupEnabled: true,
		MaxBackups:    5,
	}
}

// Storage 存储管理器
type Storage struct {
	config *Config
	mu     sync.RWMutex

	// 内存缓存
	neighbors   *NeighborStore
	tasks       *TaskStore
	reputation  *ReputationStore
	accusations *AccusationStore
	messages    *MessageStore
	nodeConfig  *NodeConfigStore
}

// New 创建存储管理器
func New(config *Config) (*Storage, error) {
	if config == nil {
		config = DefaultConfig("./data")
	}

	// 确保目录存在
	dirs := []string{
		config.DataDir,
		filepath.Join(config.DataDir, "neighbors"),
		filepath.Join(config.DataDir, "tasks"),
		filepath.Join(config.DataDir, "reputation"),
		filepath.Join(config.DataDir, "accusations"),
		filepath.Join(config.DataDir, "messages"),
		filepath.Join(config.DataDir, "backup"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	s := &Storage{
		config:      config,
		neighbors:   NewNeighborStore(filepath.Join(config.DataDir, "neighbors.json")),
		tasks:       NewTaskStore(filepath.Join(config.DataDir, "tasks.json")),
		reputation:  NewReputationStore(filepath.Join(config.DataDir, "reputation.json")),
		accusations: NewAccusationStore(filepath.Join(config.DataDir, "accusations.json")),
		messages:    NewMessageStore(filepath.Join(config.DataDir, "messages")),
		nodeConfig:  NewNodeConfigStore(filepath.Join(config.DataDir, "config.json")),
	}

	// 加载已有数据
	if err := s.Load(); err != nil {
		// 首次运行，忽略加载错误
	}

	return s, nil
}

// Load 加载所有数据
func (s *Storage) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error
	if err := s.neighbors.Load(); err != nil {
		errs = append(errs, err)
	}
	if err := s.tasks.Load(); err != nil {
		errs = append(errs, err)
	}
	if err := s.reputation.Load(); err != nil {
		errs = append(errs, err)
	}
	if err := s.accusations.Load(); err != nil {
		errs = append(errs, err)
	}
	if err := s.nodeConfig.Load(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Save 保存所有数据
func (s *Storage) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error
	if err := s.neighbors.Save(); err != nil {
		errs = append(errs, err)
	}
	if err := s.tasks.Save(); err != nil {
		errs = append(errs, err)
	}
	if err := s.reputation.Save(); err != nil {
		errs = append(errs, err)
	}
	if err := s.accusations.Save(); err != nil {
		errs = append(errs, err)
	}
	if err := s.nodeConfig.Save(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Close 关闭存储（保存数据）
func (s *Storage) Close() error {
	return s.Save()
}

// Neighbors 返回邻居存储
func (s *Storage) Neighbors() *NeighborStore {
	return s.neighbors
}

// Tasks 返回任务存储
func (s *Storage) Tasks() *TaskStore {
	return s.tasks
}

// Reputation 返回声誉存储
func (s *Storage) Reputation() *ReputationStore {
	return s.reputation
}

// Accusations 返回指责存储
func (s *Storage) Accusations() *AccusationStore {
	return s.accusations
}

// Messages 返回消息存储
func (s *Storage) Messages() *MessageStore {
	return s.messages
}

// NodeConfig 返回节点配置存储
func (s *Storage) NodeConfig() *NodeConfigStore {
	return s.nodeConfig
}

// Backup 创建备份
func (s *Storage) Backup() error {
	if !s.config.BackupEnabled {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	backupDir := filepath.Join(s.config.DataDir, "backup")
	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(backupDir, timestamp)

	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return err
	}

	// 复制文件
	files := []string{
		"neighbors.json",
		"tasks.json",
		"reputation.json",
		"accusations.json",
		"config.json",
	}

	for _, file := range files {
		src := filepath.Join(s.config.DataDir, file)
		dst := filepath.Join(backupPath, file)
		if err := copyFile(src, dst); err != nil {
			// 文件不存在时忽略
			continue
		}
	}

	// 清理旧备份
	s.cleanOldBackups()

	return nil
}

// cleanOldBackups 清理旧备份
func (s *Storage) cleanOldBackups() {
	backupDir := filepath.Join(s.config.DataDir, "backup")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return
	}

	if len(entries) <= s.config.MaxBackups {
		return
	}

	// 删除最旧的备份
	for i := 0; i < len(entries)-s.config.MaxBackups; i++ {
		path := filepath.Join(backupDir, entries[i].Name())
		os.RemoveAll(path)
	}
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// ============ 基础存储类型 ============

// BaseStore 基础存储
type BaseStore struct {
	path string
	mu   sync.RWMutex
}

func (b *BaseStore) loadJSON(v interface{}) error {
	data, err := os.ReadFile(b.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, v)
}

func (b *BaseStore) saveJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(b.path, data, 0644)
}

// ============ 邻居存储 ============

// Neighbor 邻居信息
type Neighbor struct {
	NodeID     string    `json:"node_id"`
	IP         string    `json:"ip"`
	Port       int       `json:"port"`
	Reputation float64   `json:"reputation"`
	LastSeen   time.Time `json:"last_seen"`
	Status     string    `json:"status"` // online, offline
}

// NeighborStore 邻居存储
type NeighborStore struct {
	BaseStore
	data map[string]*Neighbor
}

// NewNeighborStore 创建邻居存储
func NewNeighborStore(path string) *NeighborStore {
	return &NeighborStore{
		BaseStore: BaseStore{path: path},
		data:      make(map[string]*Neighbor),
	}
}

// Load 加载数据
func (s *NeighborStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	var list []*Neighbor
	if err := s.loadJSON(&list); err != nil {
		return err
	}
	s.data = make(map[string]*Neighbor)
	for _, n := range list {
		s.data[n.NodeID] = n
	}
	return nil
}

// Save 保存数据
func (s *NeighborStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	list := make([]*Neighbor, 0, len(s.data))
	for _, n := range s.data {
		list = append(list, n)
	}
	return s.saveJSON(list)
}

// Add 添加邻居
func (s *NeighborStore) Add(n *Neighbor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n.LastSeen = time.Now()
	s.data[n.NodeID] = n
}

// Remove 删除邻居
func (s *NeighborStore) Remove(nodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, nodeID)
}

// Get 获取邻居
func (s *NeighborStore) Get(nodeID string) (*Neighbor, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.data[nodeID]
	return n, ok
}

// List 获取所有邻居
func (s *NeighborStore) List() []*Neighbor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*Neighbor, 0, len(s.data))
	for _, n := range s.data {
		list = append(list, n)
	}
	return list
}

// Count 获取邻居数量
func (s *NeighborStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// UpdateStatus 更新状态
func (s *NeighborStore) UpdateStatus(nodeID string, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.data[nodeID]; ok {
		n.Status = status
		n.LastSeen = time.Now()
	}
}

// ============ 任务存储 ============

// Task 任务信息
type Task struct {
	TaskID       string                 `json:"task_id"`
	Creator      string                 `json:"creator"`
	Target       string                 `json:"target"`
	Type         string                 `json:"type"`
	Status       string                 `json:"status"` // created, assigned, completed, verified, failed
	Description  string                 `json:"description,omitempty"`
	ResultDigest string                 `json:"result_digest,omitempty"`
	Verifier     string                 `json:"verifier,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	CompletedAt  time.Time              `json:"completed_at,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TaskStore 任务存储
type TaskStore struct {
	BaseStore
	data map[string]*Task
}

// NewTaskStore 创建任务存储
func NewTaskStore(path string) *TaskStore {
	return &TaskStore{
		BaseStore: BaseStore{path: path},
		data:      make(map[string]*Task),
	}
}

// Load 加载数据
func (s *TaskStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	var list []*Task
	if err := s.loadJSON(&list); err != nil {
		return err
	}
	s.data = make(map[string]*Task)
	for _, t := range list {
		s.data[t.TaskID] = t
	}
	return nil
}

// Save 保存数据
func (s *TaskStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	list := make([]*Task, 0, len(s.data))
	for _, t := range s.data {
		list = append(list, t)
	}
	return s.saveJSON(list)
}

// Add 添加任务
func (s *TaskStore) Add(t *Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	s.data[t.TaskID] = t
}

// Get 获取任务
func (s *TaskStore) Get(taskID string) (*Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.data[taskID]
	return t, ok
}

// Update 更新任务
func (s *TaskStore) Update(taskID string, status string, result string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.data[taskID]; ok {
		t.Status = status
		t.ResultDigest = result
		if status == "completed" || status == "verified" || status == "failed" {
			t.CompletedAt = time.Now()
		}
		return true
	}
	return false
}

// List 获取所有任务
func (s *TaskStore) List() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*Task, 0, len(s.data))
	for _, t := range s.data {
		list = append(list, t)
	}
	return list
}

// ListByStatus 按状态获取任务
func (s *TaskStore) ListByStatus(status string) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*Task
	for _, t := range s.data {
		if t.Status == status {
			list = append(list, t)
		}
	}
	return list
}

// Count 获取任务数量
func (s *TaskStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// ============ 声誉存储 ============

// ReputationRecord 声誉记录
type ReputationRecord struct {
	NodeID     string    `json:"node_id"`
	Delta      float64   `json:"delta"`
	Reason     string    `json:"reason"`
	SourceTask string    `json:"source_task,omitempty"`
	Originator string    `json:"originator"`
	Timestamp  time.Time `json:"timestamp"`
}

// NodeReputation 节点声誉
type NodeReputation struct {
	NodeID  string              `json:"node_id"`
	Score   float64             `json:"score"`
	History []*ReputationRecord `json:"history,omitempty"`
}

// ReputationStore 声誉存储
type ReputationStore struct {
	BaseStore
	data map[string]*NodeReputation
}

// NewReputationStore 创建声誉存储
func NewReputationStore(path string) *ReputationStore {
	return &ReputationStore{
		BaseStore: BaseStore{path: path},
		data:      make(map[string]*NodeReputation),
	}
}

// Load 加载数据
func (s *ReputationStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	var list []*NodeReputation
	if err := s.loadJSON(&list); err != nil {
		return err
	}
	s.data = make(map[string]*NodeReputation)
	for _, r := range list {
		s.data[r.NodeID] = r
	}
	return nil
}

// Save 保存数据
func (s *ReputationStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	list := make([]*NodeReputation, 0, len(s.data))
	for _, r := range s.data {
		list = append(list, r)
	}
	return s.saveJSON(list)
}

// GetScore 获取声誉分数
func (s *ReputationStore) GetScore(nodeID string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if r, ok := s.data[nodeID]; ok {
		return r.Score
	}
	return 10.0 // 默认初始声誉
}

// UpdateScore 更新声誉分数
func (s *ReputationStore) UpdateScore(nodeID string, delta float64, reason string, originator string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, ok := s.data[nodeID]; !ok {
		s.data[nodeID] = &NodeReputation{
			NodeID: nodeID,
			Score:  10.0,
		}
	}
	
	r := s.data[nodeID]
	r.Score += delta
	r.History = append(r.History, &ReputationRecord{
		NodeID:     nodeID,
		Delta:      delta,
		Reason:     reason,
		Originator: originator,
		Timestamp:  time.Now(),
	})
}

// Get 获取节点声誉详情
func (s *ReputationStore) Get(nodeID string) (*NodeReputation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.data[nodeID]
	return r, ok
}

// List 获取所有声誉
func (s *ReputationStore) List() []*NodeReputation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*NodeReputation, 0, len(s.data))
	for _, r := range s.data {
		list = append(list, r)
	}
	return list
}

// ============ 指责存储 ============

// Accusation 指责记录
type Accusation struct {
	ID          string    `json:"id"`
	AccuserID   string    `json:"accuser_id"`
	AccusedID   string    `json:"accused_id"`
	Reason      string    `json:"reason"`
	Evidence    string    `json:"evidence,omitempty"`
	Status      string    `json:"status"` // pending, verified, rejected
	ProcessedBy []string  `json:"processed_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ResolvedAt  time.Time `json:"resolved_at,omitempty"`
}

// AccusationStore 指责存储
type AccusationStore struct {
	BaseStore
	data map[string]*Accusation
}

// NewAccusationStore 创建指责存储
func NewAccusationStore(path string) *AccusationStore {
	return &AccusationStore{
		BaseStore: BaseStore{path: path},
		data:      make(map[string]*Accusation),
	}
}

// Load 加载数据
func (s *AccusationStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	var list []*Accusation
	if err := s.loadJSON(&list); err != nil {
		return err
	}
	s.data = make(map[string]*Accusation)
	for _, a := range list {
		s.data[a.ID] = a
	}
	return nil
}

// Save 保存数据
func (s *AccusationStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	list := make([]*Accusation, 0, len(s.data))
	for _, a := range s.data {
		list = append(list, a)
	}
	return s.saveJSON(list)
}

// Add 添加指责
func (s *AccusationStore) Add(a *Accusation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}
	if a.Status == "" {
		a.Status = "pending"
	}
	s.data[a.ID] = a
}

// Get 获取指责
func (s *AccusationStore) Get(id string) (*Accusation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.data[id]
	return a, ok
}

// Update 更新指责状态
func (s *AccusationStore) Update(id string, status string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a, ok := s.data[id]; ok {
		a.Status = status
		if status == "verified" || status == "rejected" {
			a.ResolvedAt = time.Now()
		}
		return true
	}
	return false
}

// List 获取所有指责
func (s *AccusationStore) List() []*Accusation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*Accusation, 0, len(s.data))
	for _, a := range s.data {
		list = append(list, a)
	}
	return list
}

// ListByAccused 按被指责者获取
func (s *AccusationStore) ListByAccused(accusedID string) []*Accusation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*Accusation
	for _, a := range s.data {
		if a.AccusedID == accusedID {
			list = append(list, a)
		}
	}
	return list
}

// ============ 消息存储 ============

// Message 消息
type Message struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Subject   string    `json:"subject,omitempty"`
	Content   string    `json:"content"`
	Type      string    `json:"type"` // normal, system, task
	Read      bool      `json:"read"`
	Timestamp time.Time `json:"timestamp"`
	Signature string    `json:"signature,omitempty"`
}

// MessageStore 消息存储（基于文件目录）
type MessageStore struct {
	baseDir string
	mu      sync.RWMutex
}

// NewMessageStore 创建消息存储
func NewMessageStore(baseDir string) *MessageStore {
	return &MessageStore{baseDir: baseDir}
}

// Save 保存消息
func (s *MessageStore) Save(msg *Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 按接收者分目录
	dir := filepath.Join(s.baseDir, msg.To)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	path := filepath.Join(dir, msg.ID+".json")
	data, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Get 获取消息
func (s *MessageStore) Get(to, msgID string) (*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.baseDir, to, msgID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// List 列出某节点的所有消息
func (s *MessageStore) List(to string) ([]*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Join(s.baseDir, to)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var messages []*Message
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			path := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var msg Message
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			messages = append(messages, &msg)
		}
	}
	return messages, nil
}

// MarkRead 标记已读
func (s *MessageStore) MarkRead(to, msgID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.baseDir, to, msgID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}

	msg.Read = true
	data, err = json.MarshalIndent(msg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Delete 删除消息
func (s *MessageStore) Delete(to, msgID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.baseDir, to, msgID+".json")
	return os.Remove(path)
}

// ============ 节点配置存储 ============

// NodeConfig 节点配置
type NodeConfig struct {
	NodeID         string                 `json:"node_id"`
	Version        string                 `json:"version"`
	Network        NetworkSettings        `json:"network"`
	Neighbors      NeighborSettings       `json:"neighbors"`
	Reputation     ReputationSettings     `json:"reputation"`
	Logging        LoggingSettings        `json:"logging"`
	Extra          map[string]interface{} `json:"extra,omitempty"`
}

// NetworkSettings 网络设置
type NetworkSettings struct {
	Port       int      `json:"port"`
	NAT        bool     `json:"nat"`
	SuperNodes []string `json:"super_nodes,omitempty"`
}

// NeighborSettings 邻居设置
type NeighborSettings struct {
	MaxNeighbors    int `json:"max_neighbors"`
	RefreshInterval int `json:"refresh_interval"` // 秒
}

// ReputationSettings 声誉设置
type ReputationSettings struct {
	Initial          float64 `json:"initial"`
	DecayPerDay      float64 `json:"decay_per_day"`
	AccuseTolerance  int     `json:"accuse_tolerance"`
}

// LoggingSettings 日志设置
type LoggingSettings struct {
	Level string `json:"level"`
	File  string `json:"file"`
}

// NodeConfigStore 节点配置存储
type NodeConfigStore struct {
	BaseStore
	config *NodeConfig
}

// NewNodeConfigStore 创建节点配置存储
func NewNodeConfigStore(path string) *NodeConfigStore {
	return &NodeConfigStore{
		BaseStore: BaseStore{path: path},
		config:    DefaultNodeConfig(),
	}
}

// DefaultNodeConfig 默认配置
func DefaultNodeConfig() *NodeConfig {
	return &NodeConfig{
		Version: "0.1.0",
		Network: NetworkSettings{
			Port: 18345,
			NAT:  true,
		},
		Neighbors: NeighborSettings{
			MaxNeighbors:    8,
			RefreshInterval: 3600,
		},
		Reputation: ReputationSettings{
			Initial:         10,
			DecayPerDay:     0.1,
			AccuseTolerance: 5,
		},
		Logging: LoggingSettings{
			Level: "INFO",
			File:  "./logs/node.log",
		},
	}
}

// Load 加载配置
func (s *NodeConfigStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadJSON(&s.config)
}

// Save 保存配置
func (s *NodeConfigStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.saveJSON(s.config)
}

// Get 获取配置
func (s *NodeConfigStore) Get() *NodeConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// Set 设置配置
func (s *NodeConfigStore) Set(cfg *NodeConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}

// Update 更新配置项
func (s *NodeConfigStore) Update(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.config.Extra == nil {
		s.config.Extra = make(map[string]interface{})
	}
	s.config.Extra[key] = value
}

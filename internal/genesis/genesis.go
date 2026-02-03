package genesis

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tjfoc/gmsm/sm2"
	"github.com/tjfoc/gmsm/sm3"
)

// 错误定义
var (
	ErrInvalidGenesisSignature = errors.New("无效的创世签名")
	ErrGenesisAlreadyExists    = errors.New("创世信息已存在")
	ErrGenesisNotFound         = errors.New("创世信息未找到")
	ErrInvalidInvitation       = errors.New("无效的邀请函")
	ErrInviterNotTrusted       = errors.New("邀请节点不可信")
	ErrInvitationExpired       = errors.New("邀请函已过期")
	ErrNodeAlreadyJoined       = errors.New("节点已加入网络")
)

// GenesisInfo 创世信息
type GenesisInfo struct {
	// 创世节点信息
	GenesisNodeID  string `json:"genesis_node_id"`  // 创世节点ID
	GenesisKey     string `json:"genesis_key"`      // 创世节点公钥(hex)
	Timestamp      int64  `json:"timestamp"`        // 创世时间戳
	NetworkName    string `json:"network_name"`     // 网络名称
	NetworkVersion string `json:"network_version"`  // 网络版本

	// 初始配置
	InitialReputation   int64   `json:"initial_reputation"`    // 新节点初始声誉
	MinInviterReputation int64  `json:"min_inviter_reputation"` // 邀请节点最低声誉
	InvitationValidHours int    `json:"invitation_valid_hours"` // 邀请函有效期(小时)
	MaxNeighbors        int     `json:"max_neighbors"`          // 最大邻居数
	MinNeighbors        int     `json:"min_neighbors"`          // 最小邻居数

	// 引导节点列表
	BootstrapNodes []BootstrapNode `json:"bootstrap_nodes"`

	// 签名
	Signature string `json:"signature"` // SM2签名(hex)
}

// BootstrapNode 引导节点
type BootstrapNode struct {
	NodeID    string   `json:"node_id"`
	PublicKey string   `json:"public_key"`
	Addresses []string `json:"addresses"`
}

// Invitation 邀请函
type Invitation struct {
	InviterNodeID  string `json:"inviter_node_id"`  // 邀请节点ID
	InviterKey     string `json:"inviter_key"`      // 邀请节点公钥(hex)
	NewNodeKey     string `json:"new_node_key"`     // 新节点公钥(hex)
	Timestamp      int64  `json:"timestamp"`        // 邀请时间戳
	ExpiresAt      int64  `json:"expires_at"`       // 过期时间戳
	InitReputation int64  `json:"init_reputation"`  // 初始声誉
	Signature      string `json:"signature"`        // SM2签名(hex)
}

// JoinRequest 加入请求
type JoinRequest struct {
	NewNodeID   string      `json:"new_node_id"`   // 新节点ID
	NewNodeKey  string      `json:"new_node_key"`  // 新节点公钥(hex)
	Invitation  *Invitation `json:"invitation"`    // 邀请函
	Timestamp   int64       `json:"timestamp"`     // 请求时间戳
	Signature   string      `json:"signature"`     // 新节点签名(hex)
}

// JoinResponse 加入响应
type JoinResponse struct {
	Accepted        bool              `json:"accepted"`         // 是否接受
	Reason          string            `json:"reason,omitempty"` // 拒绝原因
	NodeID          string            `json:"node_id"`          // 分配的节点ID
	InitReputation  int64             `json:"init_reputation"`  // 初始声誉
	Neighbors       []NeighborInfo    `json:"neighbors"`        // 推荐邻居
	Timestamp       int64             `json:"timestamp"`        // 响应时间戳
	ResponderNodeID string            `json:"responder_node_id"`// 响应节点ID
	Signature       string            `json:"signature"`        // 响应节点签名
}

// NeighborInfo 邻居信息
type NeighborInfo struct {
	NodeID     string   `json:"node_id"`
	PublicKey  string   `json:"public_key"`
	Addresses  []string `json:"addresses"`
	Reputation int64    `json:"reputation"`
}

// GenesisManager 创世管理器
type GenesisManager struct {
	genesis     *GenesisInfo
	privateKey  *sm2.PrivateKey
	publicKey   *sm2.PublicKey
	nodeID      string
	dataDir     string
	
	// 已加入节点
	joinedNodes map[string]*JoinedNode
	mu          sync.RWMutex
}

// JoinedNode 已加入节点信息
type JoinedNode struct {
	NodeID     string    `json:"node_id"`
	PublicKey  string    `json:"public_key"`
	Reputation int64     `json:"reputation"`
	JoinedAt   time.Time `json:"joined_at"`
	InviterID  string    `json:"inviter_id"`
}

// NewGenesisManager 创建创世管理器
func NewGenesisManager(dataDir string) (*GenesisManager, error) {
	gm := &GenesisManager{
		dataDir:     dataDir,
		joinedNodes: make(map[string]*JoinedNode),
	}

	// 尝试加载已有的创世信息
	genesisPath := filepath.Join(dataDir, "genesis.json")
	if data, err := os.ReadFile(genesisPath); err == nil {
		var genesis GenesisInfo
		if err := json.Unmarshal(data, &genesis); err == nil {
			gm.genesis = &genesis
		}
	}

	// 尝试加载已有的密钥
	keyPath := filepath.Join(dataDir, "node_key.hex")
	if data, err := os.ReadFile(keyPath); err == nil {
		if priv, err := loadPrivateKey(string(data)); err == nil {
			gm.privateKey = priv
			gm.publicKey = &priv.PublicKey
			gm.nodeID = generateNodeID(gm.publicKey)
		}
	}

	// 加载已加入节点
	nodesPath := filepath.Join(dataDir, "joined_nodes.json")
	if data, err := os.ReadFile(nodesPath); err == nil {
		var nodes map[string]*JoinedNode
		if err := json.Unmarshal(data, &nodes); err == nil {
			gm.joinedNodes = nodes
		}
	}

	return gm, nil
}

// InitGenesis 初始化创世信息（只能执行一次）
func (gm *GenesisManager) InitGenesis(networkName, networkVersion string) (*GenesisInfo, error) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if gm.genesis != nil {
		return nil, ErrGenesisAlreadyExists
	}

	// 生成创世节点密钥
	priv, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成密钥失败: %w", err)
	}

	gm.privateKey = priv
	gm.publicKey = &priv.PublicKey
	gm.nodeID = generateNodeID(gm.publicKey)

	// 创建创世信息
	genesis := &GenesisInfo{
		GenesisNodeID:       gm.nodeID,
		GenesisKey:          hex.EncodeToString(sm2.Compress(gm.publicKey)),
		Timestamp:           time.Now().UnixMilli(),
		NetworkName:         networkName,
		NetworkVersion:      networkVersion,
		InitialReputation:   1,
		MinInviterReputation: 10,
		InvitationValidHours: 72,
		MaxNeighbors:        15,
		MinNeighbors:        3,
		BootstrapNodes:      []BootstrapNode{},
	}

	// 签名创世信息
	signature, err := gm.signGenesis(genesis)
	if err != nil {
		return nil, fmt.Errorf("签名失败: %w", err)
	}
	genesis.Signature = signature

	gm.genesis = genesis

	// 将创世节点加入已加入节点列表
	gm.joinedNodes[gm.nodeID] = &JoinedNode{
		NodeID:     gm.nodeID,
		PublicKey:  genesis.GenesisKey,
		Reputation: 100, // 创世节点高声誉
		JoinedAt:   time.Now(),
		InviterID:  "", // 无邀请者
	}

	// 保存到文件
	if err := gm.save(); err != nil {
		return nil, fmt.Errorf("保存失败: %w", err)
	}

	return genesis, nil
}

// LoadGenesis 加载创世信息
func (gm *GenesisManager) LoadGenesis(genesisJSON []byte) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	var genesis GenesisInfo
	if err := json.Unmarshal(genesisJSON, &genesis); err != nil {
		return fmt.Errorf("解析创世信息失败: %w", err)
	}

	// 验证签名
	if err := verifyGenesisSignature(&genesis); err != nil {
		return err
	}

	gm.genesis = &genesis
	return nil
}

// GetGenesis 获取创世信息
func (gm *GenesisManager) GetGenesis() *GenesisInfo {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return gm.genesis
}

// CreateInvitation 创建邀请函
func (gm *GenesisManager) CreateInvitation(newNodeKeyHex string) (*Invitation, error) {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	if gm.genesis == nil {
		return nil, ErrGenesisNotFound
	}

	if gm.privateKey == nil {
		return nil, errors.New("无私钥，无法创建邀请函")
	}

	// 检查自己的声誉是否足够
	myNode, ok := gm.joinedNodes[gm.nodeID]
	if !ok {
		return nil, errors.New("当前节点未加入网络")
	}
	if myNode.Reputation < gm.genesis.MinInviterReputation {
		return nil, fmt.Errorf("声誉不足，需要 %d，当前 %d", gm.genesis.MinInviterReputation, myNode.Reputation)
	}

	now := time.Now()
	invitation := &Invitation{
		InviterNodeID:  gm.nodeID,
		InviterKey:     hex.EncodeToString(sm2.Compress(gm.publicKey)),
		NewNodeKey:     newNodeKeyHex,
		Timestamp:      now.UnixMilli(),
		ExpiresAt:      now.Add(time.Duration(gm.genesis.InvitationValidHours) * time.Hour).UnixMilli(),
		InitReputation: gm.genesis.InitialReputation,
	}

	// 签名
	signature, err := gm.signInvitation(invitation)
	if err != nil {
		return nil, fmt.Errorf("签名邀请函失败: %w", err)
	}
	invitation.Signature = signature

	return invitation, nil
}

// VerifyInvitation 验证邀请函
func (gm *GenesisManager) VerifyInvitation(invitation *Invitation) error {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return gm.verifyInvitationLocked(invitation)
}

// verifyInvitationLocked 验证邀请函（调用者已持有锁）
func (gm *GenesisManager) verifyInvitationLocked(invitation *Invitation) error {
	if gm.genesis == nil {
		return ErrGenesisNotFound
	}

	// 检查过期
	if time.Now().UnixMilli() > invitation.ExpiresAt {
		return ErrInvitationExpired
	}

	// 检查邀请者是否在网络中
	inviter, ok := gm.joinedNodes[invitation.InviterNodeID]
	if !ok {
		return ErrInviterNotTrusted
	}

	// 检查邀请者声誉
	if inviter.Reputation < gm.genesis.MinInviterReputation {
		return ErrInviterNotTrusted
	}

	// 验证签名
	pubKey, err := parsePublicKey(invitation.InviterKey)
	if err != nil {
		return fmt.Errorf("解析邀请者公钥失败: %w", err)
	}

	// 构建签名数据
	signData := fmt.Sprintf("%s|%s|%s|%d|%d|%d",
		invitation.InviterNodeID,
		invitation.InviterKey,
		invitation.NewNodeKey,
		invitation.Timestamp,
		invitation.ExpiresAt,
		invitation.InitReputation,
	)

	sigBytes, err := hex.DecodeString(invitation.Signature)
	if err != nil {
		return fmt.Errorf("解析签名失败: %w", err)
	}

	hash := sm3.Sm3Sum([]byte(signData))
	if !pubKey.Verify(hash[:], sigBytes) {
		return ErrInvalidInvitation
	}

	return nil
}

// ProcessJoinRequest 处理加入请求
func (gm *GenesisManager) ProcessJoinRequest(req *JoinRequest) (*JoinResponse, error) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if gm.genesis == nil {
		return nil, ErrGenesisNotFound
	}

	// 检查节点是否已加入
	if _, ok := gm.joinedNodes[req.NewNodeID]; ok {
		return &JoinResponse{
			Accepted:  false,
			Reason:    "节点已加入网络",
			Timestamp: time.Now().UnixMilli(),
		}, ErrNodeAlreadyJoined
	}

	// 验证邀请函（使用内部方法避免死锁）
	if err := gm.verifyInvitationLocked(req.Invitation); err != nil {
		return &JoinResponse{
			Accepted:  false,
			Reason:    err.Error(),
			Timestamp: time.Now().UnixMilli(),
		}, err
	}

	// 验证新节点签名
	newPubKey, err := parsePublicKey(req.NewNodeKey)
	if err != nil {
		return nil, fmt.Errorf("解析新节点公钥失败: %w", err)
	}

	// 验证新节点公钥与邀请函中的一致
	if req.NewNodeKey != req.Invitation.NewNodeKey {
		return &JoinResponse{
			Accepted:  false,
			Reason:    "公钥与邀请函不匹配",
			Timestamp: time.Now().UnixMilli(),
		}, errors.New("公钥与邀请函不匹配")
	}

	// 生成节点ID
	nodeID := generateNodeIDFromKey(newPubKey)
	if nodeID != req.NewNodeID {
		return &JoinResponse{
			Accepted:  false,
			Reason:    "节点ID不匹配",
			Timestamp: time.Now().UnixMilli(),
		}, errors.New("节点ID不匹配")
	}

	// 添加新节点
	gm.joinedNodes[nodeID] = &JoinedNode{
		NodeID:     nodeID,
		PublicKey:  req.NewNodeKey,
		Reputation: req.Invitation.InitReputation,
		JoinedAt:   time.Now(),
		InviterID:  req.Invitation.InviterNodeID,
	}

	// 推荐邻居
	neighbors := gm.recommendNeighbors(nodeID)

	// 构建响应
	response := &JoinResponse{
		Accepted:        true,
		NodeID:          nodeID,
		InitReputation:  req.Invitation.InitReputation,
		Neighbors:       neighbors,
		Timestamp:       time.Now().UnixMilli(),
		ResponderNodeID: gm.nodeID,
	}

	// 签名响应
	if gm.privateKey != nil {
		sig, err := gm.signJoinResponse(response)
		if err == nil {
			response.Signature = sig
		}
	}

	// 保存节点列表
	gm.saveNodes()

	return response, nil
}

// recommendNeighbors 推荐邻居
func (gm *GenesisManager) recommendNeighbors(excludeID string) []NeighborInfo {
	var neighbors []NeighborInfo
	count := 0
	maxCount := gm.genesis.MinNeighbors * 2 // 推荐双倍最小邻居数

	for nodeID, node := range gm.joinedNodes {
		if nodeID == excludeID {
			continue
		}
		if count >= maxCount {
			break
		}

		neighbors = append(neighbors, NeighborInfo{
			NodeID:     node.NodeID,
			PublicKey:  node.PublicKey,
			Reputation: node.Reputation,
			Addresses:  []string{}, // 地址需要从其他模块获取
		})
		count++
	}

	return neighbors
}

// GetNodeReputation 获取节点声誉
func (gm *GenesisManager) GetNodeReputation(nodeID string) (int64, error) {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	node, ok := gm.joinedNodes[nodeID]
	if !ok {
		return 0, fmt.Errorf("节点 %s 未加入网络", nodeID)
	}
	return node.Reputation, nil
}

// UpdateNodeReputation 更新节点声誉
func (gm *GenesisManager) UpdateNodeReputation(nodeID string, delta int64) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	node, ok := gm.joinedNodes[nodeID]
	if !ok {
		return fmt.Errorf("节点 %s 未加入网络", nodeID)
	}

	node.Reputation += delta
	if node.Reputation < 0 {
		node.Reputation = 0
	}

	return gm.saveNodes()
}

// GetJoinedNodes 获取已加入节点列表
func (gm *GenesisManager) GetJoinedNodes() []*JoinedNode {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	nodes := make([]*JoinedNode, 0, len(gm.joinedNodes))
	for _, node := range gm.joinedNodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// IsNodeJoined 检查节点是否已加入
func (gm *GenesisManager) IsNodeJoined(nodeID string) bool {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	_, ok := gm.joinedNodes[nodeID]
	return ok
}

// GetNodeID 获取当前节点ID
func (gm *GenesisManager) GetNodeID() string {
	return gm.nodeID
}

// GetPublicKeyHex 获取当前节点公钥
func (gm *GenesisManager) GetPublicKeyHex() string {
	if gm.publicKey == nil {
		return ""
	}
	return hex.EncodeToString(sm2.Compress(gm.publicKey))
}

// signGenesis 签名创世信息
func (gm *GenesisManager) signGenesis(genesis *GenesisInfo) (string, error) {
	signData := fmt.Sprintf("%s|%s|%d|%s|%s|%d|%d|%d|%d|%d",
		genesis.GenesisNodeID,
		genesis.GenesisKey,
		genesis.Timestamp,
		genesis.NetworkName,
		genesis.NetworkVersion,
		genesis.InitialReputation,
		genesis.MinInviterReputation,
		genesis.InvitationValidHours,
		genesis.MaxNeighbors,
		genesis.MinNeighbors,
	)

	hash := sm3.Sm3Sum([]byte(signData))
	sig, err := gm.privateKey.Sign(rand.Reader, hash[:], nil)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(sig), nil
}

// signInvitation 签名邀请函
func (gm *GenesisManager) signInvitation(inv *Invitation) (string, error) {
	signData := fmt.Sprintf("%s|%s|%s|%d|%d|%d",
		inv.InviterNodeID,
		inv.InviterKey,
		inv.NewNodeKey,
		inv.Timestamp,
		inv.ExpiresAt,
		inv.InitReputation,
	)

	hash := sm3.Sm3Sum([]byte(signData))
	sig, err := gm.privateKey.Sign(rand.Reader, hash[:], nil)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(sig), nil
}

// signJoinResponse 签名加入响应
func (gm *GenesisManager) signJoinResponse(resp *JoinResponse) (string, error) {
	signData := fmt.Sprintf("%v|%s|%s|%d|%d|%s",
		resp.Accepted,
		resp.NodeID,
		resp.Reason,
		resp.InitReputation,
		resp.Timestamp,
		resp.ResponderNodeID,
	)

	hash := sm3.Sm3Sum([]byte(signData))
	sig, err := gm.privateKey.Sign(rand.Reader, hash[:], nil)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(sig), nil
}

// save 保存所有数据
func (gm *GenesisManager) save() error {
	// 确保目录存在
	if err := os.MkdirAll(gm.dataDir, 0755); err != nil {
		return err
	}

	// 保存创世信息
	if gm.genesis != nil {
		genesisPath := filepath.Join(gm.dataDir, "genesis.json")
		data, err := json.MarshalIndent(gm.genesis, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(genesisPath, data, 0644); err != nil {
			return err
		}
	}

	// 保存私钥
	if gm.privateKey != nil {
		keyPath := filepath.Join(gm.dataDir, "node_key.hex")
		keyHex := hex.EncodeToString(gm.privateKey.D.Bytes())
		if err := os.WriteFile(keyPath, []byte(keyHex), 0600); err != nil {
			return err
		}
	}

	// 保存节点列表
	return gm.saveNodes()
}

// saveNodes 保存节点列表
func (gm *GenesisManager) saveNodes() error {
	if err := os.MkdirAll(gm.dataDir, 0755); err != nil {
		return err
	}

	nodesPath := filepath.Join(gm.dataDir, "joined_nodes.json")
	data, err := json.MarshalIndent(gm.joinedNodes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(nodesPath, data, 0644)
}

// 辅助函数

func generateNodeID(pubKey *sm2.PublicKey) string {
	pubBytes := sm2.Compress(pubKey)
	hash := sm3.Sm3Sum(pubBytes)
	return hex.EncodeToString(hash[:16])
}

func generateNodeIDFromKey(pubKey *sm2.PublicKey) string {
	return generateNodeID(pubKey)
}

func loadPrivateKey(keyHex string) (*sm2.PrivateKey, error) {
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, err
	}

	priv := new(sm2.PrivateKey)
	priv.Curve = sm2.P256Sm2()
	priv.D = new(big.Int).SetBytes(keyBytes)
	priv.PublicKey.X, priv.PublicKey.Y = priv.Curve.ScalarBaseMult(keyBytes)

	return priv, nil
}

func parsePublicKey(keyHex string) (*sm2.PublicKey, error) {
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, err
	}

	pubKey := sm2.Decompress(keyBytes)
	if pubKey == nil {
		return nil, errors.New("无效的公钥")
	}

	return pubKey, nil
}

func verifyGenesisSignature(genesis *GenesisInfo) error {
	pubKey, err := parsePublicKey(genesis.GenesisKey)
	if err != nil {
		return fmt.Errorf("解析创世公钥失败: %w", err)
	}

	signData := fmt.Sprintf("%s|%s|%d|%s|%s|%d|%d|%d|%d|%d",
		genesis.GenesisNodeID,
		genesis.GenesisKey,
		genesis.Timestamp,
		genesis.NetworkName,
		genesis.NetworkVersion,
		genesis.InitialReputation,
		genesis.MinInviterReputation,
		genesis.InvitationValidHours,
		genesis.MaxNeighbors,
		genesis.MinNeighbors,
	)

	sigBytes, err := hex.DecodeString(genesis.Signature)
	if err != nil {
		return fmt.Errorf("解析签名失败: %w", err)
	}

	hash := sm3.Sm3Sum([]byte(signData))
	if !pubKey.Verify(hash[:], sigBytes) {
		return ErrInvalidGenesisSignature
	}

	return nil
}

package protocol

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/config"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/crypto"
)

// Handler 协议处理器
type Handler struct {
	config       *config.Config
	signer       crypto.Signer
	protocolHash string
}

// NewHandler 创建协议处理器
func NewHandler(cfg *config.Config, signer crypto.Signer) *Handler {
	return &Handler{
		config: cfg,
		signer: signer,
	}
}

// Start 启动协议处理
func (h *Handler) Start(ctx context.Context) {
	// 同步协议
	if err := h.syncProtocol(); err != nil {
		fmt.Printf("同步协议失败: %v\n", err)
	}

	// TODO: 启动网络监听
	// TODO: 处理入站消息

	<-ctx.Done()
}

// syncProtocol 同步协议
func (h *Handler) syncProtocol() error {
	skillPath := filepath.Join(h.config.BaseDir, "SKILL.md")

	data, err := os.ReadFile(skillPath)
	if err != nil {
		return fmt.Errorf("读取 SKILL.md 失败: %w", err)
	}

	hash := sha256.Sum256(data)
	h.protocolHash = hex.EncodeToString(hash[:])

	fmt.Printf("协议哈希: %s\n", h.protocolHash)

	return nil
}

// GetProtocolHash 获取协议哈希
func (h *Handler) GetProtocolHash() string {
	return h.protocolHash
}

// VerifyMessage 验证消息
func (h *Handler) VerifyMessage(data, signature, publicKey []byte) (bool, error) {
	// TODO: 实现消息验证
	return true, nil
}

// SignMessage 签名消息
func (h *Handler) SignMessage(data []byte) ([]byte, error) {
	return h.signer.Sign(data)
}

package heartbeat

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/config"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/crypto"
)

func setupTestService(t *testing.T) (*Service, func()) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test.key")

	// 创建测试密钥
	testKey := []byte("test-private-key")
	if err := os.WriteFile(keyPath, testKey, 0600); err != nil {
		t.Fatalf("创建测试密钥失败: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.AgentID = "test-agent-id"
	cfg.Version = "1.0.0"

	signer, err := crypto.NewSM2Signer(keyPath)
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	service := NewService(cfg, signer)

	cleanup := func() {
		// 清理资源
	}

	return service, cleanup
}

func TestNewService(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	if service == nil {
		t.Fatal("服务为空")
	}
}

func TestService_GetStatus(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	status := service.GetStatus()
	if status != string(StatusIdle) {
		t.Errorf("初始状态错误: %s (应该是 idle)", status)
	}
}

func TestService_SetStatus(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	service.SetStatus(StatusWorking)
	if service.GetStatus() != string(StatusWorking) {
		t.Error("设置状态失败")
	}

	service.SetStatus(StatusBlocked)
	if service.GetStatus() != string(StatusBlocked) {
		t.Error("设置状态失败")
	}

	service.SetStatus(StatusIdle)
	if service.GetStatus() != string(StatusIdle) {
		t.Error("设置状态失败")
	}
}

func TestService_SetTask(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	task := "处理用户请求"
	service.SetTask(&task)

	// 通过创建心跳包来验证任务已设置
	packet := service.createPacket()
	if packet.CurrentTask == nil || *packet.CurrentTask != task {
		t.Error("设置任务失败")
	}

	// 清除任务
	service.SetTask(nil)
	packet = service.createPacket()
	if packet.CurrentTask != nil {
		t.Error("清除任务失败")
	}
}

func TestService_CreatePacket(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	packet := service.createPacket()

	if packet == nil {
		t.Fatal("心跳包为空")
	}

	if packet.Type != "heartbeat" {
		t.Errorf("类型错误: %s", packet.Type)
	}

	if packet.AgentID != "test-agent-id" {
		t.Errorf("AgentID 错误: %s", packet.AgentID)
	}

	if packet.Version != "1.0.0" {
		t.Errorf("版本错误: %s", packet.Version)
	}

	if packet.Status != StatusIdle {
		t.Errorf("状态错误: %s", packet.Status)
	}

	if packet.Timestamp == "" {
		t.Error("时间戳为空")
	}
}

func TestService_Send(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	// 发送心跳（当前只是打印，不应该报错）
	err := service.Send()
	if err != nil {
		t.Fatalf("发送心跳失败: %v", err)
	}
}

func TestService_Start(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// 在后台启动服务
	done := make(chan struct{})
	go func() {
		service.Start(ctx)
		close(done)
	}()

	// 等待服务启动并发送第一次心跳
	time.Sleep(100 * time.Millisecond)

	// 取消上下文
	cancel()

	// 等待服务退出
	select {
	case <-done:
		// 正常退出
	case <-time.After(2 * time.Second):
		t.Error("服务未能正常退出")
	}
}

func TestStatus_Constants(t *testing.T) {
	if StatusIdle != "idle" {
		t.Error("StatusIdle 常量错误")
	}
	if StatusWorking != "working" {
		t.Error("StatusWorking 常量错误")
	}
	if StatusBlocked != "blocked" {
		t.Error("StatusBlocked 常量错误")
	}
}

func TestContributions_Default(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	packet := service.createPacket()

	if packet.Contributions.PRsMerged != 0 {
		t.Error("PRsMerged 默认值错误")
	}
	if packet.Contributions.PRsReviewed != 0 {
		t.Error("PRsReviewed 默认值错误")
	}
	if packet.Contributions.IssuesClosed != 0 {
		t.Error("IssuesClosed 默认值错误")
	}
	if packet.Contributions.Discussions != 0 {
		t.Error("Discussions 默认值错误")
	}
}

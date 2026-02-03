package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()
	config := &Config{
		DataDir:       tmpDir,
		SyncInterval:  time.Second,
		BackupEnabled: true,
		MaxBackups:    3,
	}

	s, err := New(config)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	// 验证目录创建
	dirs := []string{"neighbors", "tasks", "reputation", "accusations", "messages", "backup"}
	for _, dir := range dirs {
		path := filepath.Join(tmpDir, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("目录未创建: %s", dir)
		}
	}
}

func TestNeighborStore(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewNeighborStore(filepath.Join(tmpDir, "neighbors.json"))

	// 添加邻居
	n := &Neighbor{
		NodeID:     "node-001",
		IP:         "192.168.1.1",
		Port:       9000,
		Reputation: 10.0,
		Status:     "online",
	}
	store.Add(n)

	// 获取邻居
	got, ok := store.Get("node-001")
	if !ok {
		t.Fatal("Get() 未找到邻居")
	}
	if got.IP != "192.168.1.1" {
		t.Errorf("IP = %v, want %v", got.IP, "192.168.1.1")
	}

	// 列表
	list := store.List()
	if len(list) != 1 {
		t.Errorf("List() 长度 = %d, want 1", len(list))
	}

	// 保存和加载
	if err := store.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	store2 := NewNeighborStore(filepath.Join(tmpDir, "neighbors.json"))
	if err := store2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if store2.Count() != 1 {
		t.Errorf("Load后 Count() = %d, want 1", store2.Count())
	}

	// 删除
	store.Remove("node-001")
	if store.Count() != 0 {
		t.Errorf("Remove后 Count() = %d, want 0", store.Count())
	}
}

func TestTaskStore(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewTaskStore(filepath.Join(tmpDir, "tasks.json"))

	// 添加任务
	task := &Task{
		TaskID:  "task-001",
		Creator: "node-001",
		Target:  "node-002",
		Type:    "compute",
		Status:  "created",
	}
	store.Add(task)

	// 获取
	got, ok := store.Get("task-001")
	if !ok {
		t.Fatal("Get() 未找到任务")
	}
	if got.Status != "created" {
		t.Errorf("Status = %v, want created", got.Status)
	}

	// 更新
	store.Update("task-001", "completed", "result-hash")
	got, _ = store.Get("task-001")
	if got.Status != "completed" {
		t.Errorf("Update后 Status = %v, want completed", got.Status)
	}

	// 按状态列表
	completed := store.ListByStatus("completed")
	if len(completed) != 1 {
		t.Errorf("ListByStatus() 长度 = %d, want 1", len(completed))
	}

	// 保存和加载
	if err := store.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	store2 := NewTaskStore(filepath.Join(tmpDir, "tasks.json"))
	if err := store2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if store2.Count() != 1 {
		t.Errorf("Load后 Count() = %d, want 1", store2.Count())
	}
}

func TestReputationStore(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewReputationStore(filepath.Join(tmpDir, "reputation.json"))

	// 默认分数
	score := store.GetScore("node-001")
	if score != 10.0 {
		t.Errorf("默认 GetScore() = %v, want 10.0", score)
	}

	// 更新分数
	store.UpdateScore("node-001", 5.0, "任务完成", "node-002")
	score = store.GetScore("node-001")
	if score != 15.0 {
		t.Errorf("UpdateScore后 = %v, want 15.0", score)
	}

	// 获取详情
	rep, ok := store.Get("node-001")
	if !ok {
		t.Fatal("Get() 未找到声誉")
	}
	if len(rep.History) != 1 {
		t.Errorf("History 长度 = %d, want 1", len(rep.History))
	}

	// 保存和加载
	if err := store.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	store2 := NewReputationStore(filepath.Join(tmpDir, "reputation.json"))
	if err := store2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if store2.GetScore("node-001") != 15.0 {
		t.Errorf("Load后 GetScore() = %v, want 15.0", store2.GetScore("node-001"))
	}
}

func TestAccusationStore(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewAccusationStore(filepath.Join(tmpDir, "accusations.json"))

	// 添加指责
	acc := &Accusation{
		ID:        "acc-001",
		AccuserID: "node-001",
		AccusedID: "node-002",
		Reason:    "未完成任务",
		Status:    "pending",
	}
	store.Add(acc)

	// 获取
	got, ok := store.Get("acc-001")
	if !ok {
		t.Fatal("Get() 未找到指责")
	}
	if got.Status != "pending" {
		t.Errorf("Status = %v, want pending", got.Status)
	}

	// 更新
	store.Update("acc-001", "verified")
	got, _ = store.Get("acc-001")
	if got.Status != "verified" {
		t.Errorf("Update后 Status = %v, want verified", got.Status)
	}

	// 按被指责者列表
	list := store.ListByAccused("node-002")
	if len(list) != 1 {
		t.Errorf("ListByAccused() 长度 = %d, want 1", len(list))
	}

	// 保存和加载
	if err := store.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	store2 := NewAccusationStore(filepath.Join(tmpDir, "accusations.json"))
	if err := store2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(store2.List()) != 1 {
		t.Errorf("Load后 List() 长度 = %d, want 1", len(store2.List()))
	}
}

func TestMessageStore(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewMessageStore(filepath.Join(tmpDir, "messages"))

	// 保存消息
	msg := &Message{
		ID:      "msg-001",
		From:    "node-001",
		To:      "node-002",
		Subject: "测试",
		Content: "Hello World",
		Type:    "normal",
	}
	if err := store.Save(msg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// 获取消息
	got, err := store.Get("node-002", "msg-001")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Content != "Hello World" {
		t.Errorf("Content = %v, want Hello World", got.Content)
	}

	// 列表
	list, err := store.List("node-002")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Errorf("List() 长度 = %d, want 1", len(list))
	}

	// 标记已读
	if err := store.MarkRead("node-002", "msg-001"); err != nil {
		t.Fatalf("MarkRead() error = %v", err)
	}
	got, _ = store.Get("node-002", "msg-001")
	if !got.Read {
		t.Error("MarkRead后 Read = false, want true")
	}

	// 删除
	if err := store.Delete("node-002", "msg-001"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	list, _ = store.List("node-002")
	if len(list) != 0 {
		t.Errorf("Delete后 List() 长度 = %d, want 0", len(list))
	}
}

func TestNodeConfigStore(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewNodeConfigStore(filepath.Join(tmpDir, "config.json"))

	// 获取默认配置
	cfg := store.Get()
	if cfg.Version != "0.1.0" {
		t.Errorf("Version = %v, want 0.1.0", cfg.Version)
	}

	// 设置配置
	cfg.NodeID = "test-node"
	store.Set(cfg)

	// 保存
	if err := store.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// 加载
	store2 := NewNodeConfigStore(filepath.Join(tmpDir, "config.json"))
	if err := store2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	cfg2 := store2.Get()
	if cfg2.NodeID != "test-node" {
		t.Errorf("Load后 NodeID = %v, want test-node", cfg2.NodeID)
	}
}

func TestStorageBackup(t *testing.T) {
	tmpDir := t.TempDir()
	config := &Config{
		DataDir:       tmpDir,
		SyncInterval:  time.Second,
		BackupEnabled: true,
		MaxBackups:    2,
	}

	s, err := New(config)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	// 添加一些数据
	s.Neighbors().Add(&Neighbor{NodeID: "node-001", IP: "1.1.1.1", Port: 9000})
	s.Save()

	// 创建备份
	if err := s.Backup(); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	// 验证备份目录
	backupDir := filepath.Join(tmpDir, "backup")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("读取备份目录失败: %v", err)
	}
	if len(entries) == 0 {
		t.Error("备份目录为空")
	}

	// 多次备份测试清理
	time.Sleep(time.Second)
	s.Backup()
	time.Sleep(time.Second)
	s.Backup()
	time.Sleep(time.Second)
	s.Backup()

	entries, _ = os.ReadDir(backupDir)
	if len(entries) > 2 {
		t.Errorf("备份清理失败，期望最多2个，实际 %d 个", len(entries))
	}
}

func TestStorageIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	config := &Config{
		DataDir:       tmpDir,
		SyncInterval:  time.Second,
		BackupEnabled: false,
	}

	s, err := New(config)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// 添加各种数据
	s.Neighbors().Add(&Neighbor{NodeID: "n1", IP: "1.1.1.1", Port: 9000, Reputation: 10})
	s.Neighbors().Add(&Neighbor{NodeID: "n2", IP: "2.2.2.2", Port: 9001, Reputation: 8})

	s.Tasks().Add(&Task{TaskID: "t1", Creator: "n1", Target: "n2", Status: "created"})
	s.Tasks().Update("t1", "completed", "result")

	s.Reputation().UpdateScore("n2", 5.0, "完成任务", "n1")

	s.Accusations().Add(&Accusation{ID: "a1", AccuserID: "n1", AccusedID: "n3", Reason: "作弊"})

	s.Messages().Save(&Message{ID: "m1", From: "n1", To: "n2", Content: "Hello"})

	// 保存
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// 关闭
	s.Close()

	// 重新加载
	s2, err := New(config)
	if err != nil {
		t.Fatalf("重新打开 error = %v", err)
	}
	defer s2.Close()

	// 验证数据
	if s2.Neighbors().Count() != 2 {
		t.Errorf("Neighbors.Count() = %d, want 2", s2.Neighbors().Count())
	}
	if s2.Tasks().Count() != 1 {
		t.Errorf("Tasks.Count() = %d, want 1", s2.Tasks().Count())
	}
	if s2.Reputation().GetScore("n2") != 15.0 {
		t.Errorf("Reputation.GetScore(n2) = %v, want 15.0", s2.Reputation().GetScore("n2"))
	}
	if len(s2.Accusations().List()) != 1 {
		t.Errorf("Accusations.List() 长度 = %d, want 1", len(s2.Accusations().List()))
	}

	msgs, _ := s2.Messages().List("n2")
	if len(msgs) != 1 {
		t.Errorf("Messages.List(n2) 长度 = %d, want 1", len(msgs))
	}
}

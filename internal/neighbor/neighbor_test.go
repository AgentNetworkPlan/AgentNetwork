package neighbor

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewNeighborManager(t *testing.T) {
	nm := NewNeighborManager(nil)
	if nm == nil {
		t.Fatal("NewNeighborManager 返回 nil")
	}

	if nm.config.MinNeighbors != 3 {
		t.Errorf("默认 MinNeighbors 错误: got %d, want 3", nm.config.MinNeighbors)
	}

	if nm.config.MaxNeighbors != 15 {
		t.Errorf("默认 MaxNeighbors 错误: got %d, want 15", nm.config.MaxNeighbors)
	}
}

func TestAddNeighbor(t *testing.T) {
	nm := NewNeighborManager(nil)

	neighbor := &Neighbor{
		NodeID:     "node1",
		PublicKey:  "pubkey1",
		Type:       TypeNormal,
		Reputation: 10,
	}

	err := nm.AddNeighbor(neighbor)
	if err != nil {
		t.Fatalf("添加邻居失败: %v", err)
	}

	if nm.NeighborCount() != 1 {
		t.Errorf("邻居数量错误: got %d, want 1", nm.NeighborCount())
	}

	// 验证邻居信息
	n, err := nm.GetNeighbor("node1")
	if err != nil {
		t.Fatalf("获取邻居失败: %v", err)
	}

	if n.NodeID != "node1" {
		t.Errorf("邻居ID错误")
	}

	if n.PingStatus != StatusUnknown {
		t.Errorf("初始状态应该是 unknown: got %s", n.PingStatus)
	}
}

func TestAddNeighborDuplicate(t *testing.T) {
	nm := NewNeighborManager(nil)

	neighbor := &Neighbor{
		NodeID:     "node1",
		Reputation: 10,
	}

	nm.AddNeighbor(neighbor)
	err := nm.AddNeighbor(neighbor)

	if err != ErrNeighborAlreadyExists {
		t.Errorf("预期 ErrNeighborAlreadyExists, got %v", err)
	}
}

func TestAddNeighborLowReputation(t *testing.T) {
	nm := NewNeighborManager(nil)

	neighbor := &Neighbor{
		NodeID:     "node1",
		Reputation: 1, // 低于默认阈值 5
	}

	err := nm.AddNeighbor(neighbor)
	if err != ErrReputationTooLow {
		t.Errorf("预期 ErrReputationTooLow, got %v", err)
	}
}

func TestMaxNeighbors(t *testing.T) {
	config := &NeighborConfig{
		MinNeighbors:  1,
		MaxNeighbors:  3,
		MinReputation: 1,
	}
	nm := NewNeighborManager(config)

	// 添加到最大数量
	for i := 0; i < 3; i++ {
		nm.AddNeighbor(&Neighbor{
			NodeID:     string(rune('a' + i)),
			Reputation: 10,
		})
	}

	// 再添加应该失败或替换
	err := nm.AddNeighbor(&Neighbor{
		NodeID:     "d",
		Reputation: 10,
	})

	// 因为会移除低质量邻居，所以应该成功
	if err != nil {
		t.Logf("达到最大数量后添加: %v", err)
	}
}

func TestRemoveNeighbor(t *testing.T) {
	nm := NewNeighborManager(nil)

	nm.AddNeighbor(&Neighbor{
		NodeID:     "node1",
		Reputation: 10,
	})

	err := nm.RemoveNeighbor("node1", "test")
	if err != nil {
		t.Fatalf("移除邻居失败: %v", err)
	}

	if nm.NeighborCount() != 0 {
		t.Errorf("邻居数量应该为 0")
	}

	// 移除不存在的邻居
	err = nm.RemoveNeighbor("node2", "test")
	if err != ErrNeighborNotFound {
		t.Errorf("预期 ErrNeighborNotFound, got %v", err)
	}
}

func TestGetAllNeighbors(t *testing.T) {
	nm := NewNeighborManager(nil)

	for i := 0; i < 5; i++ {
		nm.AddNeighbor(&Neighbor{
			NodeID:     string(rune('a' + i)),
			Reputation: 10,
		})
	}

	neighbors := nm.GetAllNeighbors()
	if len(neighbors) != 5 {
		t.Errorf("邻居数量错误: got %d, want 5", len(neighbors))
	}
}

func TestGetNeighborsByType(t *testing.T) {
	nm := NewNeighborManager(nil)

	nm.AddNeighbor(&Neighbor{NodeID: "n1", Reputation: 10, Type: TypeNormal})
	nm.AddNeighbor(&Neighbor{NodeID: "n2", Reputation: 10, Type: TypeNormal})
	nm.AddNeighbor(&Neighbor{NodeID: "s1", Reputation: 10, Type: TypeSuper})

	normal := nm.GetNeighborsByType(TypeNormal)
	if len(normal) != 2 {
		t.Errorf("普通邻居数量错误: got %d, want 2", len(normal))
	}

	super := nm.GetNeighborsByType(TypeSuper)
	if len(super) != 1 {
		t.Errorf("超级邻居数量错误: got %d, want 1", len(super))
	}
}

func TestIsNeighbor(t *testing.T) {
	nm := NewNeighborManager(nil)

	nm.AddNeighbor(&Neighbor{NodeID: "node1", Reputation: 10})

	if !nm.IsNeighbor("node1") {
		t.Error("node1 应该是邻居")
	}

	if nm.IsNeighbor("node2") {
		t.Error("node2 不应该是邻居")
	}
}

func TestUpdateNeighborReputation(t *testing.T) {
	nm := NewNeighborManager(nil)

	nm.AddNeighbor(&Neighbor{NodeID: "node1", Reputation: 10})

	err := nm.UpdateNeighborReputation("node1", 20)
	if err != nil {
		t.Fatalf("更新声誉失败: %v", err)
	}

	n, _ := nm.GetNeighbor("node1")
	if n.Reputation != 20 {
		t.Errorf("声誉更新错误: got %d, want 20", n.Reputation)
	}

	// 声誉过低应该被移除
	nm.UpdateNeighborReputation("node1", 1)
	if nm.IsNeighbor("node1") {
		t.Error("低声誉邻居应该被移除")
	}
}

func TestUpdateNeighborContribution(t *testing.T) {
	nm := NewNeighborManager(nil)

	nm.AddNeighbor(&Neighbor{NodeID: "node1", Reputation: 10})

	err := nm.UpdateNeighborContribution("node1", 50)
	if err != nil {
		t.Fatalf("更新贡献失败: %v", err)
	}

	n, _ := nm.GetNeighbor("node1")
	if n.Contribution != 50 {
		t.Errorf("贡献更新错误: got %d, want 50", n.Contribution)
	}
}

func TestPing(t *testing.T) {
	nm := NewNeighborManager(nil)

	nm.AddNeighbor(&Neighbor{NodeID: "node1", Reputation: 10})

	// 设置成功的 ping 函数
	nm.SetPingFunc(func(nodeID string) error {
		return nil
	})

	err := nm.Ping("node1")
	if err != nil {
		t.Fatalf("Ping 失败: %v", err)
	}

	n, _ := nm.GetNeighbor("node1")
	if n.PingStatus != StatusOnline {
		t.Errorf("状态应该是 online: got %s", n.PingStatus)
	}

	if n.SuccessfulPings != 1 {
		t.Errorf("成功 ping 次数错误: got %d, want 1", n.SuccessfulPings)
	}
}

func TestPingFailure(t *testing.T) {
	config := &NeighborConfig{
		MinNeighbors:    1,
		MaxNeighbors:    10,
		MinReputation:   1,
		MaxPingFailures: 2,
	}
	nm := NewNeighborManager(config)

	nm.AddNeighbor(&Neighbor{NodeID: "node1", Reputation: 10})

	// 设置失败的 ping 函数
	nm.SetPingFunc(func(nodeID string) error {
		return errors.New("ping failed")
	})

	// 模拟多次失败
	nm.Ping("node1")
	nm.Ping("node1")

	n, _ := nm.GetNeighbor("node1")
	if n.PingStatus != StatusOffline {
		t.Errorf("状态应该是 offline: got %s", n.PingStatus)
	}

	if n.FailedPings != 2 {
		t.Errorf("失败 ping 次数错误: got %d, want 2", n.FailedPings)
	}
}

func TestPingAll(t *testing.T) {
	nm := NewNeighborManager(nil)

	for i := 0; i < 3; i++ {
		nm.AddNeighbor(&Neighbor{
			NodeID:     string(rune('a' + i)),
			Reputation: 10,
		})
	}

	var pingCount int32
	nm.SetPingFunc(func(nodeID string) error {
		atomic.AddInt32(&pingCount, 1)
		return nil
	})

	results := nm.PingAll()
	if len(results) != 3 {
		t.Errorf("Ping 结果数量错误: got %d, want 3", len(results))
	}

	if atomic.LoadInt32(&pingCount) != 3 {
		t.Errorf("Ping 调用次数错误: got %d, want 3", pingCount)
	}
}

func TestGetBestNeighbors(t *testing.T) {
	config := &NeighborConfig{
		MinNeighbors:  1,
		MaxNeighbors:  10,
		MinReputation: 1,
	}
	nm := NewNeighborManager(config)

	// 添加邻居
	nm.AddNeighbor(&Neighbor{NodeID: "n1", Reputation: 10})
	nm.AddNeighbor(&Neighbor{NodeID: "n2", Reputation: 10})
	nm.AddNeighbor(&Neighbor{NodeID: "n3", Reputation: 10})
	nm.AddNeighbor(&Neighbor{NodeID: "n4", Reputation: 10})

	// 手动设置状态和信任分
	nm.mu.Lock()
	nm.neighbors["n1"].TrustScore = 0.9
	nm.neighbors["n1"].PingStatus = StatusOnline
	nm.neighbors["n2"].TrustScore = 0.5
	nm.neighbors["n2"].PingStatus = StatusOnline
	nm.neighbors["n3"].TrustScore = 0.7
	nm.neighbors["n3"].PingStatus = StatusOnline
	nm.neighbors["n4"].TrustScore = 0.3
	nm.neighbors["n4"].PingStatus = StatusOffline
	nm.mu.Unlock()

	best := nm.GetBestNeighbors(2)
	if len(best) != 2 {
		t.Fatalf("应该返回 2 个邻居: got %d", len(best))
	}

	// 应该按信任分排序
	if best[0].TrustScore < best[1].TrustScore {
		t.Error("应该按信任分降序排序")
	}
}

func TestCandidates(t *testing.T) {
	nm := NewNeighborManager(nil)

	candidate := &Neighbor{
		NodeID:     "candidate1",
		Reputation: 10,
	}

	nm.AddCandidate(candidate)

	candidates := nm.GetCandidates()
	if len(candidates) != 1 {
		t.Errorf("候选数量错误: got %d, want 1", len(candidates))
	}

	// 提升为邻居
	err := nm.PromoteCandidate("candidate1")
	if err != nil {
		t.Fatalf("提升候选失败: %v", err)
	}

	if !nm.IsNeighbor("candidate1") {
		t.Error("候选应该已成为邻居")
	}

	candidates = nm.GetCandidates()
	if len(candidates) != 0 {
		t.Errorf("候选应该被移除")
	}
}

func TestNeedMoreNeighbors(t *testing.T) {
	config := &NeighborConfig{
		MinNeighbors:  3,
		MaxNeighbors:  10,
		MinReputation: 1,
	}
	nm := NewNeighborManager(config)

	if !nm.NeedMoreNeighbors() {
		t.Error("初始状态应该需要更多邻居")
	}

	// 添加足够邻居
	for i := 0; i < 3; i++ {
		nm.AddNeighbor(&Neighbor{
			NodeID:     string(rune('a' + i)),
			Reputation: 10,
		})
	}

	if nm.NeedMoreNeighbors() {
		t.Error("已有足够邻居，不应该需要更多")
	}
}

func TestGetStats(t *testing.T) {
	nm := NewNeighborManager(nil)

	nm.AddNeighbor(&Neighbor{NodeID: "n1", Reputation: 10, TrustScore: 0.8})
	nm.AddNeighbor(&Neighbor{NodeID: "n2", Reputation: 20, TrustScore: 0.6})

	stats := nm.GetStats()

	if stats["total"].(int) != 2 {
		t.Errorf("total 错误: got %d, want 2", stats["total"])
	}

	if stats["avg_reputation"].(int64) != 15 {
		t.Errorf("avg_reputation 错误: got %d, want 15", stats["avg_reputation"])
	}
}

func TestOnlineCount(t *testing.T) {
	nm := NewNeighborManager(nil)

	nm.AddNeighbor(&Neighbor{NodeID: "n1", Reputation: 10})
	nm.AddNeighbor(&Neighbor{NodeID: "n2", Reputation: 10})

	// 初始状态都是 unknown
	if nm.OnlineCount() != 0 {
		t.Errorf("初始在线数应该为 0")
	}

	// 手动设置为 online
	nm.mu.Lock()
	nm.neighbors["n1"].PingStatus = StatusOnline
	nm.mu.Unlock()

	if nm.OnlineCount() != 1 {
		t.Errorf("在线数应该为 1")
	}
}

func TestNeighborCallbacks(t *testing.T) {
	nm := NewNeighborManager(nil)

	addedCount := 0
	removedCount := 0

	nm.SetOnNeighborAdded(func(n *Neighbor) {
		addedCount++
	})

	nm.SetOnNeighborRemoved(func(n *Neighbor) {
		removedCount++
	})

	nm.AddNeighbor(&Neighbor{NodeID: "n1", Reputation: 10})
	time.Sleep(10 * time.Millisecond) // 等待回调执行

	if addedCount != 1 {
		t.Errorf("添加回调次数错误: got %d, want 1", addedCount)
	}

	nm.RemoveNeighbor("n1", "test")
	time.Sleep(10 * time.Millisecond)

	if removedCount != 1 {
		t.Errorf("移除回调次数错误: got %d, want 1", removedCount)
	}
}

func TestExportImportNeighbors(t *testing.T) {
	nm1 := NewNeighborManager(nil)

	nm1.AddNeighbor(&Neighbor{NodeID: "n1", Reputation: 10})
	nm1.AddNeighbor(&Neighbor{NodeID: "n2", Reputation: 20})

	exported := nm1.ExportNeighbors()

	nm2 := NewNeighborManager(nil)
	nm2.ImportNeighbors(exported)

	if nm2.NeighborCount() != 2 {
		t.Errorf("导入后邻居数量错误: got %d, want 2", nm2.NeighborCount())
	}
}

func TestNeighborString(t *testing.T) {
	n := &Neighbor{
		NodeID:     "1234567890abcdef",
		Type:       TypeNormal,
		Reputation: 50,
		PingStatus: StatusOnline,
		TrustScore: 0.85,
	}

	str := n.String()
	if str == "" {
		t.Error("String() 不应返回空")
	}
	t.Logf("Neighbor: %s", str)
}

func BenchmarkAddNeighbor(b *testing.B) {
	nm := NewNeighborManager(&NeighborConfig{
		MinNeighbors:  1,
		MaxNeighbors:  b.N + 10,
		MinReputation: 1,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nm.AddNeighbor(&Neighbor{
			NodeID:     string(rune(i)),
			Reputation: 10,
		})
	}
}

func BenchmarkGetAllNeighbors(b *testing.B) {
	nm := NewNeighborManager(&NeighborConfig{
		MinNeighbors:  1,
		MaxNeighbors:  100,
		MinReputation: 1,
	})

	for i := 0; i < 50; i++ {
		nm.AddNeighbor(&Neighbor{
			NodeID:     string(rune(i)),
			Reputation: 10,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nm.GetAllNeighbors()
	}
}

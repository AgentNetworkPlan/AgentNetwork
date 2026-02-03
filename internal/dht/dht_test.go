package dht

import (
	"testing"
)

func TestNewTable(t *testing.T) {
	table := NewTable("local-node-id")

	if table == nil {
		t.Fatal("路由表为空")
	}

	if table.localID != "local-node-id" {
		t.Errorf("本地 ID 错误: %s", table.localID)
	}
}

func TestTable_AddNode(t *testing.T) {
	table := NewTable("local-node")

	node := &Node{
		ID:       "node-1",
		Address:  "192.168.1.100",
		Port:     8080,
		LastSeen: 1234567890,
	}

	table.AddNode(node)

	// 验证节点已添加
	found := table.GetNode("node-1")
	if found == nil {
		t.Fatal("节点未找到")
	}

	if found.Address != "192.168.1.100" {
		t.Errorf("地址错误: %s", found.Address)
	}

	if found.Port != 8080 {
		t.Errorf("端口错误: %d", found.Port)
	}
}

func TestTable_AddMultipleNodes(t *testing.T) {
	table := NewTable("local-node")

	for i := 0; i < 10; i++ {
		node := &Node{
			ID:       "node-" + string(rune('A'+i)),
			Address:  "192.168.1." + string(rune('0'+i)),
			Port:     8080 + i,
			LastSeen: int64(i),
		}
		table.AddNode(node)
	}

	// 验证所有节点都能找到
	for i := 0; i < 10; i++ {
		found := table.GetNode("node-" + string(rune('A'+i)))
		if found == nil {
			t.Errorf("节点 %d 未找到", i)
		}
	}
}

func TestTable_UpdateNode(t *testing.T) {
	table := NewTable("local-node")

	// 添加节点
	node1 := &Node{
		ID:       "node-1",
		Address:  "192.168.1.100",
		Port:     8080,
		LastSeen: 1000,
	}
	table.AddNode(node1)

	// 更新节点
	node2 := &Node{
		ID:       "node-1",
		Address:  "192.168.1.200",
		Port:     9090,
		LastSeen: 2000,
	}
	table.AddNode(node2)

	// 验证更新
	found := table.GetNode("node-1")
	if found.Address != "192.168.1.200" {
		t.Errorf("地址未更新: %s", found.Address)
	}
	if found.Port != 9090 {
		t.Errorf("端口未更新: %d", found.Port)
	}
	if found.LastSeen != 2000 {
		t.Errorf("LastSeen 未更新: %d", found.LastSeen)
	}
}

func TestTable_RemoveNode(t *testing.T) {
	table := NewTable("local-node")

	node := &Node{
		ID:      "node-1",
		Address: "192.168.1.100",
		Port:    8080,
	}
	table.AddNode(node)

	// 确认节点存在
	if table.GetNode("node-1") == nil {
		t.Fatal("节点应该存在")
	}

	// 移除节点
	table.RemoveNode("node-1")

	// 确认节点已移除
	if table.GetNode("node-1") != nil {
		t.Error("节点应该已被移除")
	}
}

func TestTable_GetNode_NotFound(t *testing.T) {
	table := NewTable("local-node")

	found := table.GetNode("nonexistent")
	if found != nil {
		t.Error("不存在的节点应该返回 nil")
	}
}

func TestTable_FindClosestNodes(t *testing.T) {
	table := NewTable("local-node")

	// 添加多个节点
	for i := 0; i < 20; i++ {
		node := &Node{
			ID:      "node-" + string(rune('A'+i)),
			Address: "192.168.1.100",
			Port:    8080 + i,
		}
		table.AddNode(node)
	}

	// 查找最近的 5 个节点
	closest := table.FindClosestNodes("target-id", 5)

	if len(closest) > 5 {
		t.Errorf("返回节点数量超过限制: %d", len(closest))
	}

	t.Logf("找到 %d 个最近节点", len(closest))
}

func TestTable_FindClosestNodes_Empty(t *testing.T) {
	table := NewTable("local-node")

	closest := table.FindClosestNodes("target-id", 5)

	if len(closest) != 0 {
		t.Errorf("空表应该返回空列表: %d", len(closest))
	}
}

func TestHashKey(t *testing.T) {
	key1 := HashKey("test-key")
	key2 := HashKey("test-key")
	key3 := HashKey("different-key")

	if key1 != key2 {
		t.Error("相同输入应该产生相同哈希")
	}

	if key1 == key3 {
		t.Error("不同输入应该产生不同哈希")
	}

	if len(key1) != 64 { // SHA256 产生 32 字节 = 64 个十六进制字符
		t.Errorf("哈希长度错误: %d", len(key1))
	}

	t.Logf("哈希值: %s", key1)
}

func TestTable_Distance(t *testing.T) {
	table := NewTable("local")

	// 相同 ID 的距离应该是全 0
	dist := table.distance("same", "same")
	allZero := true
	for _, b := range dist {
		if b != 0 {
			allZero = false
			break
		}
	}
	if !allZero {
		t.Error("相同 ID 的距离应该是 0")
	}

	// 不同 ID 的距离应该非零
	dist2 := table.distance("id1", "id2")
	hasNonZero := false
	for _, b := range dist2 {
		if b != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Error("不同 ID 的距离应该非零")
	}
}

func TestTable_GetBucketIndex(t *testing.T) {
	table := NewTable("local")

	// 测试不同的距离值
	tests := []struct {
		distance []byte
		expected int
	}{
		{[]byte{0x80, 0, 0, 0}, 0},  // 最高位为 1
		{[]byte{0x40, 0, 0, 0}, 1},  // 第二高位为 1
		{[]byte{0x01, 0, 0, 0}, 7},  // 第 8 位为 1
		{[]byte{0, 0x80, 0, 0}, 8},  // 第 9 位为 1
		{[]byte{0, 0, 0, 0}, 255},   // 全 0
	}

	for _, tt := range tests {
		// 填充到 32 字节
		dist := make([]byte, 32)
		copy(dist, tt.distance)

		idx := table.getBucketIndex(dist)
		if idx != tt.expected {
			t.Errorf("距离 %v 的 bucket 索引错误: %d (期望 %d)",
				tt.distance, idx, tt.expected)
		}
	}
}

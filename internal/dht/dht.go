package dht

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// Node DHT 节点
type Node struct {
	ID       string
	Address  string
	Port     int
	LastSeen int64
}

// Table DHT 路由表
type Table struct {
	localID string
	buckets [256][]*Node // K-Bucket
	mu      sync.RWMutex
}

// NewTable 创建 DHT 路由表
func NewTable(localID string) *Table {
	return &Table{
		localID: localID,
	}
}

// AddNode 添加节点
func (t *Table) AddNode(node *Node) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 计算与本地节点的距离
	distance := t.distance(t.localID, node.ID)
	bucketIndex := t.getBucketIndex(distance)

	// 添加到对应的 bucket
	bucket := t.buckets[bucketIndex]
	
	// 检查是否已存在
	for i, n := range bucket {
		if n.ID == node.ID {
			// 更新节点信息
			t.buckets[bucketIndex][i] = node
			return
		}
	}

	// 添加新节点
	t.buckets[bucketIndex] = append(bucket, node)
}

// RemoveNode 移除节点
func (t *Table) RemoveNode(nodeID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	distance := t.distance(t.localID, nodeID)
	bucketIndex := t.getBucketIndex(distance)

	bucket := t.buckets[bucketIndex]
	for i, n := range bucket {
		if n.ID == nodeID {
			t.buckets[bucketIndex] = append(bucket[:i], bucket[i+1:]...)
			return
		}
	}
}

// FindClosestNodes 查找最近的节点
func (t *Table) FindClosestNodes(targetID string, count int) []*Node {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var allNodes []*Node
	for _, bucket := range t.buckets {
		allNodes = append(allNodes, bucket...)
	}

	// 按距离排序
	// TODO: 实现真正的排序
	if len(allNodes) > count {
		return allNodes[:count]
	}

	return allNodes
}

// GetNode 获取节点
func (t *Table) GetNode(nodeID string) *Node {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, bucket := range t.buckets {
		for _, n := range bucket {
			if n.ID == nodeID {
				return n
			}
		}
	}

	return nil
}

// distance 计算两个节点 ID 的 XOR 距离
func (t *Table) distance(id1, id2 string) []byte {
	hash1 := sha256.Sum256([]byte(id1))
	hash2 := sha256.Sum256([]byte(id2))

	result := make([]byte, 32)
	for i := 0; i < 32; i++ {
		result[i] = hash1[i] ^ hash2[i]
	}

	return result
}

// getBucketIndex 根据距离获取 bucket 索引
func (t *Table) getBucketIndex(distance []byte) int {
	for i := 0; i < len(distance); i++ {
		for j := 7; j >= 0; j-- {
			if (distance[i]>>j)&1 == 1 {
				return i*8 + (7 - j)
			}
		}
	}
	return 255
}

// HashKey 计算键的哈希
func HashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

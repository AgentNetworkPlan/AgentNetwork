package discovery

import (
	"testing"
	"time"
)

func TestDiscoveryConstants(t *testing.T) {
	if DiscoveryNamespace != "/daan/1.0.0" {
		t.Errorf("DiscoveryNamespace 错误: %s", DiscoveryNamespace)
	}

	if DiscoveryInterval != 5*time.Minute {
		t.Errorf("DiscoveryInterval 错误: %v", DiscoveryInterval)
	}
}

// 注意：更完整的 discovery 测试需要真实的 libp2p host 和 DHT
// 这些测试在 node_test.go 中通过集成测试覆盖

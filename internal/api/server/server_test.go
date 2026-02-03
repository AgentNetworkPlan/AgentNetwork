package server

import (
	"context"
	"testing"
	"time"
)

func TestNodeStatus_Constants(t *testing.T) {
	if StatusOnline != "online" {
		t.Error("StatusOnline 常量错误")
	}
	if StatusOffline != "offline" {
		t.Error("StatusOffline 常量错误")
	}
	if StatusBusy != "busy" {
		t.Error("StatusBusy 常量错误")
	}
}

func TestNodeFilter(t *testing.T) {
	filter := &NodeFilter{
		Capabilities: []string{"compute", "storage"},
		Status:       "online",
		Region:       "asia",
		Limit:        10,
	}

	if len(filter.Capabilities) != 2 {
		t.Error("Capabilities 错误")
	}

	if filter.Status != "online" {
		t.Error("Status 错误")
	}

	if filter.Limit != 10 {
		t.Error("Limit 错误")
	}
}

func TestNodeInfo(t *testing.T) {
	info := &NodeInfo{
		NodeId:       "node-123",
		PeerId:       "peer-456",
		Addresses:    []string{"/ip4/127.0.0.1/tcp/8080"},
		Status:       string(StatusOnline),
		Capabilities: []string{"compute"},
		ConnectedAt:  time.Now().Unix(),
		LastSeen:     time.Now().Unix(),
	}

	if info.NodeId != "node-123" {
		t.Error("NodeId 错误")
	}

	if len(info.Addresses) != 1 {
		t.Error("Addresses 错误")
	}
}

func TestTaskRequest(t *testing.T) {
	req := &TaskRequest{
		TaskId:     "task-001",
		TaskType:   "compute",
		Payload:    []byte("test payload"),
		TargetNode: "node-123",
		TimeoutMs:  5000,
	}

	if req.TaskId != "task-001" {
		t.Error("TaskId 错误")
	}

	if req.TimeoutMs != 5000 {
		t.Error("TimeoutMs 错误")
	}

	if string(req.Payload) != "test payload" {
		t.Error("Payload 错误")
	}
}

func TestTaskResponse(t *testing.T) {
	resp := &TaskResponse{
		TaskId:     "task-001",
		Success:    true,
		Result:     []byte("result data"),
		ExecutedBy: "node-123",
		DurationMs: 100,
	}

	if !resp.Success {
		t.Error("Success 应该为 true")
	}

	if resp.DurationMs != 100 {
		t.Error("DurationMs 错误")
	}
}

func TestDataRequest(t *testing.T) {
	req := &DataRequest{
		Key:        "data-key",
		Value:      []byte("data-value"),
		TtlSeconds: 3600,
	}

	if req.Key != "data-key" {
		t.Error("Key 错误")
	}

	if req.TtlSeconds != 3600 {
		t.Error("TtlSeconds 错误")
	}
}

func TestHeartbeatRequest(t *testing.T) {
	req := &HeartbeatRequest{
		NodeId:       "node-123",
		Status:       "online",
		Capabilities: []string{"compute", "storage"},
		Timestamp:    time.Now().Unix(),
	}

	if req.NodeId != "node-123" {
		t.Error("NodeId 错误")
	}

	if len(req.Capabilities) != 2 {
		t.Error("Capabilities 数量错误")
	}
}

func TestHeartbeatResponse(t *testing.T) {
	resp := &HeartbeatResponse{
		Success:    true,
		ServerTime: time.Now().Unix(),
	}

	if !resp.Success {
		t.Error("Success 应该为 true")
	}

	if resp.ServerTime == 0 {
		t.Error("ServerTime 不应该为 0")
	}
}

func TestUnimplementedToolNetworkServer(t *testing.T) {
	server := UnimplementedToolNetworkServer{}
	ctx := context.Background()

	// 测试所有方法都返回 nil
	nodeList, _ := server.GetNodeList(ctx, &NodeFilter{})
	if nodeList != nil {
		t.Error("GetNodeList 应该返回 nil")
	}

	nodeInfo, _ := server.GetNodeInfo(ctx, &NodeInfoRequest{})
	if nodeInfo != nil {
		t.Error("GetNodeInfo 应该返回 nil")
	}

	taskResp, _ := server.SendTask(ctx, &TaskRequest{})
	if taskResp != nil {
		t.Error("SendTask 应该返回 nil")
	}

	storeResp, _ := server.StoreData(ctx, &DataRequest{})
	if storeResp != nil {
		t.Error("StoreData 应该返回 nil")
	}

	fetchResp, _ := server.FetchData(ctx, &FetchRequest{})
	if fetchResp != nil {
		t.Error("FetchData 应该返回 nil")
	}

	hbResp, _ := server.Heartbeat(ctx, &HeartbeatRequest{})
	if hbResp != nil {
		t.Error("Heartbeat 应该返回 nil")
	}
}

package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := NewServer(nil)
		if err != ErrNilConfig {
			t.Errorf("expected ErrNilConfig, got %v", err)
		}
	})
	
	t.Run("empty node ID", func(t *testing.T) {
		config := &Config{}
		_, err := NewServer(config)
		if err != ErrEmptyNodeID {
			t.Errorf("expected ErrEmptyNodeID, got %v", err)
		}
	})
	
	t.Run("valid config", func(t *testing.T) {
		config := DefaultConfig("node1")
		s, err := NewServer(config)
		if err != nil {
			t.Fatalf("failed to create server: %v", err)
		}
		if s == nil {
			t.Fatal("server is nil")
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig("node1")
	
	if config.NodeID != "node1" {
		t.Errorf("expected NodeID 'node1', got %s", config.NodeID)
	}
	if config.ListenAddr != ":18345" {
		t.Errorf("expected ListenAddr ':18345', got %s", config.ListenAddr)
	}
	if config.ReadTimeout != 30*time.Second {
		t.Errorf("expected ReadTimeout 30s, got %v", config.ReadTimeout)
	}
	if !config.EnableCORS {
		t.Error("expected EnableCORS true")
	}
}

func TestStartStop(t *testing.T) {
	config := DefaultConfig("node1")
	config.ListenAddr = ":0" // 随机端口
	
	s, _ := NewServer(config)
	
	err := s.Start()
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	
	time.Sleep(50 * time.Millisecond)
	
	if !s.IsRunning() {
		t.Error("expected server to be running")
	}
	
	// 再次启动不应有问题
	err = s.Start()
	if err != nil {
		t.Errorf("second start failed: %v", err)
	}
	
	err = s.Stop()
	if err != nil {
		t.Fatalf("failed to stop server: %v", err)
	}
	
	time.Sleep(50 * time.Millisecond)
	
	// 再次停止不应有问题
	err = s.Stop()
	if err != nil {
		t.Errorf("second stop failed: %v", err)
	}
}

func createTestServer() *Server {
	config := DefaultConfig("test-node")
	s, _ := NewServer(config)
	return s
}

func TestHandleHealth(t *testing.T) {
	s := createTestServer()
	
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	
	s.handleHealth(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	
	if !resp.Success {
		t.Error("expected success=true")
	}
	
	data := resp.Data.(map[string]interface{})
	if data["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", data["status"])
	}
}

func TestHandleStatus(t *testing.T) {
	s := createTestServer()
	
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	
	s.handleStatus(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	
	if !resp.Success {
		t.Error("expected success=true")
	}
	
	data := resp.Data.(map[string]interface{})
	if data["node_id"] != "test-node" {
		t.Errorf("expected node_id 'test-node', got %v", data["node_id"])
	}
}

func TestHandleNodeInfo(t *testing.T) {
	s := createTestServer()
	
	t.Run("GET request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/node/info", nil)
		w := httptest.NewRecorder()
		
		s.handleNodeInfo(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
	
	t.Run("POST request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/node/info", nil)
		w := httptest.NewRecorder()
		
		s.handleNodeInfo(w, req)
		
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})
}

func TestHandlePeers(t *testing.T) {
	s := createTestServer()
	
	t.Run("no peers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/node/peers", nil)
		w := httptest.NewRecorder()
		
		s.handlePeers(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
		
		var resp Response
		json.Unmarshal(w.Body.Bytes(), &resp)
		
		data := resp.Data.(map[string]interface{})
		if data["count"].(float64) != 0 {
			t.Errorf("expected count 0, got %v", data["count"])
		}
	})
	
	t.Run("with peers", func(t *testing.T) {
		s.GetPeersFunc = func() []*PeerInfo {
			return []*PeerInfo{
				{NodeID: "peer1", Status: "online"},
				{NodeID: "peer2", Status: "online"},
			}
		}
		
		req := httptest.NewRequest(http.MethodGet, "/api/v1/node/peers", nil)
		w := httptest.NewRecorder()
		
		s.handlePeers(w, req)
		
		var resp Response
		json.Unmarshal(w.Body.Bytes(), &resp)
		
		data := resp.Data.(map[string]interface{})
		if data["count"].(float64) != 2 {
			t.Errorf("expected count 2, got %v", data["count"])
		}
	})
}

func TestHandleSendMessage(t *testing.T) {
	s := createTestServer()
	
	t.Run("valid message", func(t *testing.T) {
		msg := MessageRequest{
			To:      "recipient1",
			Type:    "text",
			Content: "Hello",
		}
		body, _ := json.Marshal(msg)
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/message/send", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleSendMessage(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
	
	t.Run("missing recipient", func(t *testing.T) {
		msg := MessageRequest{
			Type:    "text",
			Content: "Hello",
		}
		body, _ := json.Marshal(msg)
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/message/send", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleSendMessage(w, req)
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
	
	t.Run("invalid body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/message/send", bytes.NewReader([]byte("invalid")))
		w := httptest.NewRecorder()
		
		s.handleSendMessage(w, req)
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
	
	t.Run("wrong method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/message/send", nil)
		w := httptest.NewRecorder()
		
		s.handleSendMessage(w, req)
		
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})
}

func TestHandleReceiveMessage(t *testing.T) {
	s := createTestServer()
	
	var receivedMsg *MessageRequest
	s.OnMessageReceived = func(from string, msg *MessageRequest) {
		receivedMsg = msg
	}
	
	msg := MessageRequest{
		Type:    "text",
		Content: "Test message",
	}
	body, _ := json.Marshal(msg)
	
	req := httptest.NewRequest(http.MethodPost, "/api/v1/message/receive", bytes.NewReader(body))
	req.Header.Set("X-NodeID", "sender1")
	w := httptest.NewRecorder()
	
	s.handleReceiveMessage(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	
	if receivedMsg == nil {
		t.Error("expected message to be received")
	}
}

func TestHandleCreateTask(t *testing.T) {
	s := createTestServer()
	
	t.Run("valid task", func(t *testing.T) {
		task := TaskRequest{
			TaskID:      "task123",
			Type:        "compute",
			Description: "Test task",
		}
		body, _ := json.Marshal(task)
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/task/create", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleCreateTask(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
	
	t.Run("with create func", func(t *testing.T) {
		s.CreateTaskFunc = func(task *TaskRequest) (string, error) {
			return "generated-task-id", nil
		}
		
		task := TaskRequest{
			Type:        "compute",
			Description: "Test task",
		}
		body, _ := json.Marshal(task)
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/task/create", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleCreateTask(w, req)
		
		var resp Response
		json.Unmarshal(w.Body.Bytes(), &resp)
		
		data := resp.Data.(map[string]interface{})
		if data["task_id"] != "generated-task-id" {
			t.Errorf("expected task_id 'generated-task-id', got %v", data["task_id"])
		}
	})
}

func TestHandleTaskStatus(t *testing.T) {
	s := createTestServer()
	
	t.Run("with task_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/task/status?task_id=task123", nil)
		w := httptest.NewRecorder()
		
		s.handleTaskStatus(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
	
	t.Run("missing task_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/task/status", nil)
		w := httptest.NewRecorder()
		
		s.handleTaskStatus(w, req)
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestHandleReputationQuery(t *testing.T) {
	s := createTestServer()
	
	t.Run("default node", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reputation/query", nil)
		w := httptest.NewRecorder()
		
		s.handleReputationQuery(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
		
		var resp Response
		json.Unmarshal(w.Body.Bytes(), &resp)
		
		data := resp.Data.(map[string]interface{})
		if data["node_id"] != "test-node" {
			t.Errorf("expected node_id 'test-node', got %v", data["node_id"])
		}
	})
	
	t.Run("specific node", func(t *testing.T) {
		s.GetReputationFunc = func(nodeID string) float64 {
			return 75.0
		}
		
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reputation/query?node_id=node2", nil)
		w := httptest.NewRecorder()
		
		s.handleReputationQuery(w, req)
		
		var resp Response
		json.Unmarshal(w.Body.Bytes(), &resp)
		
		data := resp.Data.(map[string]interface{})
		if data["reputation"].(float64) != 75.0 {
			t.Errorf("expected reputation 75.0, got %v", data["reputation"])
		}
	})
}

func TestHandleReputationUpdate(t *testing.T) {
	s := createTestServer()
	
	req := ReputationRequest{
		NodeID: "node2",
		Delta:  5.0,
		Reason: "good behavior",
	}
	body, _ := json.Marshal(req)
	
	r := httptest.NewRequest(http.MethodPost, "/api/v1/reputation/update", bytes.NewReader(body))
	w := httptest.NewRecorder()
	
	s.handleReputationUpdate(w, r)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleAccusationCreate(t *testing.T) {
	s := createTestServer()
	
	t.Run("valid accusation", func(t *testing.T) {
		acc := AccusationRequest{
			Accused: "bad-node",
			Type:    "spam",
			Reason:  "spamming messages",
		}
		body, _ := json.Marshal(acc)
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accusation/create", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleAccusationCreate(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
	
	t.Run("missing accused", func(t *testing.T) {
		acc := AccusationRequest{
			Type:   "spam",
			Reason: "test",
		}
		body, _ := json.Marshal(acc)
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accusation/create", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleAccusationCreate(w, req)
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
	
	t.Run("with callback", func(t *testing.T) {
		var createdAcc *AccusationRequest
		s.OnAccusationCreate = func(from string, acc *AccusationRequest) {
			createdAcc = acc
		}
		
		acc := AccusationRequest{
			Accused: "bad-node",
			Type:    "spam",
			Reason:  "test",
		}
		body, _ := json.Marshal(acc)
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accusation/create", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleAccusationCreate(w, req)
		
		if createdAcc == nil {
			t.Error("expected callback to be called")
		}
	})
}

func TestHandleAccusationList(t *testing.T) {
	s := createTestServer()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/accusation/list", nil)
	w := httptest.NewRecorder()
	
	s.handleAccusationList(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleLogSubmit(t *testing.T) {
	s := createTestServer()
	
	logEntry := map[string]interface{}{
		"event_type": "task_complete",
		"task_id":    "task123",
	}
	body, _ := json.Marshal(logEntry)
	
	req := httptest.NewRequest(http.MethodPost, "/api/v1/log/submit", bytes.NewReader(body))
	w := httptest.NewRecorder()
	
	s.handleLogSubmit(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleLogQuery(t *testing.T) {
	s := createTestServer()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/log/query?node_id=node1&limit=50", nil)
	w := httptest.NewRecorder()
	
	s.handleLogQuery(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestRegisterHandler(t *testing.T) {
	s := createTestServer()
	
	s.RegisterHandler("/custom", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	if _, exists := s.handlers["/custom"]; !exists {
		t.Error("expected handler to be registered")
	}
}

func TestMiddleware(t *testing.T) {
	s := createTestServer()
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	wrapped := s.middleware(handler)
	
	t.Run("CORS headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		
		wrapped.ServeHTTP(w, req)
		
		if w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Error("expected CORS header")
		}
	})
	
	t.Run("OPTIONS request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		w := httptest.NewRecorder()
		
		wrapped.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Run("getQueryParam", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test?key=value", nil)
		
		v := getQueryParam(req, "key", "default")
		if v != "value" {
			t.Errorf("expected 'value', got %s", v)
		}
		
		v = getQueryParam(req, "missing", "default")
		if v != "default" {
			t.Errorf("expected 'default', got %s", v)
		}
	})
	
	t.Run("getIntQueryParam", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test?num=42&invalid=abc", nil)
		
		v := getIntQueryParam(req, "num", 0)
		if v != 42 {
			t.Errorf("expected 42, got %d", v)
		}
		
		v = getIntQueryParam(req, "invalid", 10)
		if v != 10 {
			t.Errorf("expected 10, got %d", v)
		}
		
		v = getIntQueryParam(req, "missing", 5)
		if v != 5 {
			t.Errorf("expected 5, got %d", v)
		}
	})
	
	t.Run("extractNodeID", func(t *testing.T) {
		// From header
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-NodeID", "node1")
		
		id := extractNodeID(req)
		if id != "node1" {
			t.Errorf("expected 'node1', got %s", id)
		}
		
		// From query
		req = httptest.NewRequest(http.MethodGet, "/test?node_id=node2", nil)
		
		id = extractNodeID(req)
		if id != "node2" {
			t.Errorf("expected 'node2', got %s", id)
		}
	})
}

func TestValidateSignature(t *testing.T) {
	s := createTestServer()
	
	t.Run("no verify func", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		
		if !s.validateSignature(req, []byte("data")) {
			t.Error("expected validation to pass without verify func")
		}
	})
	
	t.Run("with verify func", func(t *testing.T) {
		s.config.VerifyFunc = func(nodeID string, data []byte, signature string) bool {
			return signature == "valid"
		}
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-NodeID", "node1")
		req.Header.Set("X-Signature", "valid")
		
		if !s.validateSignature(req, []byte("data")) {
			t.Error("expected validation to pass")
		}
		
		req.Header.Set("X-Signature", "invalid")
		if s.validateSignature(req, []byte("data")) {
			t.Error("expected validation to fail")
		}
	})
}

func TestGetListenAddr(t *testing.T) {
	config := DefaultConfig("node1")
	config.ListenAddr = ":9999"
	
	s, _ := NewServer(config)
	
	if s.GetListenAddr() != ":9999" {
		t.Errorf("expected ':9999', got %s", s.GetListenAddr())
	}
}

// ============== 新接口测试 ==============

func TestHandleNeighborList(t *testing.T) {
	s := createTestServer()
	
	t.Run("no neighbors", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/neighbor/list", nil)
		w := httptest.NewRecorder()
		
		s.handleNeighborList(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
	
	t.Run("with neighbors", func(t *testing.T) {
		s.GetNeighborsFunc = func(limit int) []*PeerInfo {
			return []*PeerInfo{
				{NodeID: "peer1", Status: "online"},
			}
		}
		
		req := httptest.NewRequest(http.MethodGet, "/api/v1/neighbor/list?limit=5", nil)
		w := httptest.NewRecorder()
		
		s.handleNeighborList(w, req)
		
		var resp Response
		json.Unmarshal(w.Body.Bytes(), &resp)
		
		data := resp.Data.(map[string]interface{})
		if data["count"].(float64) != 1 {
			t.Errorf("expected count 1, got %v", data["count"])
		}
	})
}

func TestHandleNeighborAdd(t *testing.T) {
	s := createTestServer()
	
	t.Run("valid request", func(t *testing.T) {
		body, _ := json.Marshal(NeighborRequest{
			NodeID:    "peer1",
			Addresses: []string{"/ip4/127.0.0.1/tcp/18345"},
		})
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/neighbor/add", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleNeighborAdd(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
	
	t.Run("missing node_id", func(t *testing.T) {
		body, _ := json.Marshal(NeighborRequest{})
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/neighbor/add", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleNeighborAdd(w, req)
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestHandleMailboxSend(t *testing.T) {
	s := createTestServer()
	
	t.Run("valid request", func(t *testing.T) {
		body, _ := json.Marshal(MailboxSendRequest{
			To:      "recipient1",
			Subject: "Test",
			Content: "Hello",
		})
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/mailbox/send", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleMailboxSend(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
	
	t.Run("missing recipient", func(t *testing.T) {
		body, _ := json.Marshal(MailboxSendRequest{
			Subject: "Test",
		})
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/mailbox/send", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleMailboxSend(w, req)
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestHandleMailboxInbox(t *testing.T) {
	s := createTestServer()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mailbox/inbox?limit=10", nil)
	w := httptest.NewRecorder()
	
	s.handleMailboxInbox(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleBulletinPublish(t *testing.T) {
	s := createTestServer()
	
	t.Run("valid request", func(t *testing.T) {
		body, _ := json.Marshal(BulletinPublishRequest{
			Topic:   "tasks",
			Content: "New task available",
			TTL:     3600,
		})
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bulletin/publish", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleBulletinPublish(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
	
	t.Run("missing content", func(t *testing.T) {
		body, _ := json.Marshal(BulletinPublishRequest{
			Topic: "tasks",
		})
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bulletin/publish", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleBulletinPublish(w, req)
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestHandleBulletinByTopic(t *testing.T) {
	s := createTestServer()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bulletin/topic/tasks?limit=10", nil)
	w := httptest.NewRecorder()
	
	s.handleBulletinByTopic(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleBulletinSearch(t *testing.T) {
	s := createTestServer()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bulletin/search?keyword=task&limit=10", nil)
	w := httptest.NewRecorder()
	
	s.handleBulletinSearch(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleVotingCreate(t *testing.T) {
	s := createTestServer()
	
	t.Run("valid request", func(t *testing.T) {
		body, _ := json.Marshal(ProposalRequest{
			Title: "Kick bad node",
			Type:  "kick",
		})
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/voting/proposal/create", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleVotingCreate(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
	
	t.Run("missing title", func(t *testing.T) {
		body, _ := json.Marshal(ProposalRequest{
			Type: "kick",
		})
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/voting/proposal/create", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleVotingCreate(w, req)
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestHandleVotingVote(t *testing.T) {
	s := createTestServer()
	
	t.Run("valid vote", func(t *testing.T) {
		body, _ := json.Marshal(VoteRequest{
			ProposalID: "prop123",
			Vote:       "yes",
		})
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/voting/vote", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleVotingVote(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
	
	t.Run("missing proposal_id", func(t *testing.T) {
		body, _ := json.Marshal(VoteRequest{
			Vote: "yes",
		})
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/voting/vote", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleVotingVote(w, req)
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestHandleSuperNodeList(t *testing.T) {
	s := createTestServer()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/supernode/list", nil)
	w := httptest.NewRecorder()
	
	s.handleSuperNodeList(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleSuperNodeApply(t *testing.T) {
	s := createTestServer()
	
	body, _ := json.Marshal(SuperNodeApplyRequest{
		Stake: 1000,
	})
	
	req := httptest.NewRequest(http.MethodPost, "/api/v1/supernode/apply", bytes.NewReader(body))
	w := httptest.NewRecorder()
	
	s.handleSuperNodeApply(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleSuperNodeVote(t *testing.T) {
	s := createTestServer()
	
	t.Run("valid vote", func(t *testing.T) {
		body, _ := json.Marshal(SuperNodeVoteRequest{
			Candidate: "candidate1",
		})
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/supernode/vote", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleSuperNodeVote(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
	
	t.Run("missing candidate", func(t *testing.T) {
		body, _ := json.Marshal(SuperNodeVoteRequest{})
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/supernode/vote", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleSuperNodeVote(w, req)
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestHandleGenesisInfo(t *testing.T) {
	s := createTestServer()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/genesis/info", nil)
	w := httptest.NewRecorder()
	
	s.handleGenesisInfo(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleGenesisJoin(t *testing.T) {
	s := createTestServer()
	
	t.Run("valid request", func(t *testing.T) {
		body, _ := json.Marshal(GenesisJoinRequest{
			Invitation: "inv123",
			Pubkey:     "pubkey123",
		})
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/genesis/join", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleGenesisJoin(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
	
	t.Run("missing fields", func(t *testing.T) {
		body, _ := json.Marshal(GenesisJoinRequest{
			Invitation: "inv123",
		})
		
		req := httptest.NewRequest(http.MethodPost, "/api/v1/genesis/join", bytes.NewReader(body))
		w := httptest.NewRecorder()
		
		s.handleGenesisJoin(w, req)
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestHandleIncentiveAward(t *testing.T) {
	s := createTestServer()
	
	body, _ := json.Marshal(IncentiveAwardRequest{
		NodeID:   "node1",
		TaskType: "relay",
	})
	
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incentive/award", bytes.NewReader(body))
	w := httptest.NewRecorder()
	
	s.handleIncentiveAward(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleIncentiveTolerance(t *testing.T) {
	s := createTestServer()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/incentive/tolerance?node_id=node1", nil)
	w := httptest.NewRecorder()
	
	s.handleIncentiveTolerance(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleReputationRanking(t *testing.T) {
	s := createTestServer()
	
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reputation/ranking?limit=10", nil)
	w := httptest.NewRecorder()
	
	s.handleReputationRanking(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleAccusationAnalyze(t *testing.T) {
	s := createTestServer()
	
	t.Run("with node_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accusation/analyze?node_id=node1", nil)
		w := httptest.NewRecorder()
		
		s.handleAccusationAnalyze(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
	
	t.Run("missing node_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accusation/analyze", nil)
		w := httptest.NewRecorder()
		
		s.handleAccusationAnalyze(w, req)
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestExtractPathParam(t *testing.T) {
	t.Run("valid prefix", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bulletin/message/msg123", nil)
		
		param := extractPathParam(req, "/api/v1/bulletin/message/")
		if param != "msg123" {
			t.Errorf("expected 'msg123', got %s", param)
		}
	})
	
	t.Run("no match", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/other/path", nil)
		
		param := extractPathParam(req, "/api/v1/bulletin/message/")
		if param != "" {
			t.Errorf("expected empty string, got %s", param)
		}
	})
}

package webadmin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockNodeInfo implements NodeInfoProvider for testing.
type mockNodeInfo struct{}

func (m *mockNodeInfo) GetNodeID() string {
	return "12D3KooWTest123"
}

func (m *mockNodeInfo) GetPeerCount() int {
	return 5
}

func (m *mockNodeInfo) GetPeers() []string {
	return []string{"peer1", "peer2", "peer3", "peer4", "peer5"}
}

func (m *mockNodeInfo) GetNodeStatus() *NodeStatus {
	return &NodeStatus{
		NodeID:      "12D3KooWTest123",
		PublicKey:   "test-public-key",
		StartTime:   time.Now().Add(-time.Hour),
		Uptime:      "1h0m0s",
		Version:     "1.0.0",
		P2PPort:     9000,
		HTTPPort:    18345,
		GRPCPort:    50051,
		AdminPort:   18080,
		IsGenesis:   false,
		IsSupernode: false,
		Reputation:  100.0,
		TokenCount:  1000,
	}
}

func (m *mockNodeInfo) GetHTTPAPIEndpoints() []APIEndpoint {
	return []APIEndpoint{
		{Method: "GET", Path: "/api/v1/node/info", Description: "Get node info", Category: "Node"},
		{Method: "GET", Path: "/api/v1/peers", Description: "List peers", Category: "Network"},
	}
}

func (m *mockNodeInfo) GetRecentLogs(limit int) []LogEntry {
	return []LogEntry{
		{Timestamp: time.Now(), Level: "INFO", Module: "p2p", Message: "Node started"},
		{Timestamp: time.Now(), Level: "INFO", Module: "dht", Message: "DHT ready"},
	}
}

func (m *mockNodeInfo) GetNetworkStats() *NetworkStats {
	return &NetworkStats{
		TotalPeers:       10,
		ActivePeers:      5,
		MessagesSent:     1000,
		MessagesReceived: 900,
		BytesSent:        1024000,
		BytesReceived:    512000,
		AvgLatency:       50.5,
	}
}

func newTestServer() *Server {
	config := &Config{
		ListenAddr:      "127.0.0.1:0",
		AdminToken:      "test-token-12345",
		SessionDuration: time.Hour,
		EnableCORS:      true,
	}
	return New(config, &mockNodeInfo{})
}

// TestHealthEndpoint tests the health check endpoint.
func TestHealthEndpoint(t *testing.T) {
	server := newTestServer()

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%v'", resp["status"])
	}
}

// TestLoginEndpoint tests the login endpoint.
func TestLoginEndpoint(t *testing.T) {
	server := newTestServer()

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantError  bool
	}{
		{
			name:       "valid token",
			body:       `{"token": "test-token-12345"}`,
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "invalid token",
			body:       `{"token": "wrong-token"}`,
			wantStatus: http.StatusUnauthorized,
			wantError:  true,
		},
		{
			name:       "empty token",
			body:       `{"token": ""}`,
			wantStatus: http.StatusUnauthorized,
			wantError:  true,
		},
		{
			name:       "missing token",
			body:       `{}`,
			wantStatus: http.StatusUnauthorized,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.mux.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}

			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)

			if tt.wantError {
				if _, ok := resp["error"]; !ok {
					t.Error("Expected error in response")
				}
			} else {
				if _, ok := resp["success"]; !ok {
					t.Error("Expected success in response")
				}
			}
		})
	}
}

// TestProtectedEndpointsWithoutAuth tests that protected endpoints require auth.
func TestProtectedEndpointsWithoutAuth(t *testing.T) {
	server := newTestServer()

	endpoints := []string{
		"/api/node/status",
		"/api/node/peers",
		"/api/topology",
		"/api/endpoints",
		"/api/logs",
		"/api/stats",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			req := httptest.NewRequest("GET", endpoint, nil)
			w := httptest.NewRecorder()

			server.mux.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401 for %s without auth, got %d", endpoint, w.Code)
			}
		})
	}
}

// TestProtectedEndpointsWithTokenParam tests authentication via URL token.
func TestProtectedEndpointsWithTokenParam(t *testing.T) {
	server := newTestServer()

	endpoints := []string{
		"/api/node/status",
		"/api/node/peers",
		"/api/stats",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			req := httptest.NewRequest("GET", endpoint+"?token=test-token-12345", nil)
			w := httptest.NewRecorder()

			server.mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200 for %s with token, got %d", endpoint, w.Code)
			}
		})
	}
}

// TestProtectedEndpointsWithBearerToken tests authentication via Bearer token.
func TestProtectedEndpointsWithBearerToken(t *testing.T) {
	server := newTestServer()

	req := httptest.NewRequest("GET", "/api/node/status", nil)
	req.Header.Set("Authorization", "Bearer test-token-12345")
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 with Bearer token, got %d", w.Code)
	}
}

// TestSPAFallback tests that SPA routes return index.html.
func TestSPAFallback(t *testing.T) {
	server := newTestServer()

	spaRoutes := []string{
		"/login",
		"/dashboard",
		"/topology",
		"/endpoints",
		"/logs",
		"/about",
		"/some/unknown/route",
	}

	for _, route := range spaRoutes {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest("GET", route, nil)
			w := httptest.NewRecorder()

			server.mux.ServeHTTP(w, req)

			// Should return 200 (SPA fallback to index.html)
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200 for SPA route %s, got %d", route, w.Code)
			}

			contentType := w.Header().Get("Content-Type")
			if !strings.Contains(contentType, "text/html") {
				t.Errorf("Expected Content-Type text/html for %s, got %s", route, contentType)
			}
		})
	}
}

// TestAPIRoutesNotFallback tests that API routes don't fallback to index.html.
func TestAPIRoutesNotFallback(t *testing.T) {
	server := newTestServer()

	// Non-existent API routes should return 404, not index.html
	req := httptest.NewRequest("GET", "/api/nonexistent", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent API route, got %d", w.Code)
	}
}

// TestCORSHeaders tests that CORS headers are set when enabled.
func TestCORSHeaders(t *testing.T) {
	server := newTestServer()

	req := httptest.NewRequest("OPTIONS", "/api/health", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for OPTIONS, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS Allow-Origin header")
	}

	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("Expected CORS Allow-Methods header")
	}
}

// TestNodeStatusEndpoint tests the node status API response.
func TestNodeStatusEndpoint(t *testing.T) {
	server := newTestServer()

	req := httptest.NewRequest("GET", "/api/node/status?token=test-token-12345", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var status NodeStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if status.NodeID != "12D3KooWTest123" {
		t.Errorf("Expected NodeID '12D3KooWTest123', got '%s'", status.NodeID)
	}

	if status.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got '%s'", status.Version)
	}
}

// TestPeersEndpoint tests the peers API response.
func TestPeersEndpoint(t *testing.T) {
	server := newTestServer()

	req := httptest.NewRequest("GET", "/api/node/peers?token=test-token-12345", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	count := int(resp["count"].(float64))
	if count != 5 {
		t.Errorf("Expected 5 peers, got %d", count)
	}
}

// TestStatsEndpoint tests the stats API response.
func TestStatsEndpoint(t *testing.T) {
	server := newTestServer()

	req := httptest.NewRequest("GET", "/api/stats?token=test-token-12345", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var stats NetworkStats
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if stats.TotalPeers != 10 {
		t.Errorf("Expected 10 total peers, got %d", stats.TotalPeers)
	}

	if stats.ActivePeers != 5 {
		t.Errorf("Expected 5 active peers, got %d", stats.ActivePeers)
	}
}

// TestEndpointsAPI tests the endpoints listing API.
func TestEndpointsAPI(t *testing.T) {
	server := newTestServer()

	req := httptest.NewRequest("GET", "/api/endpoints?token=test-token-12345", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check that response contains endpoints
	if endpoints, ok := resp["endpoints"]; ok {
		if arr, ok := endpoints.([]interface{}); ok {
			if len(arr) != 2 {
				t.Errorf("Expected 2 endpoints, got %d", len(arr))
			}
		}
	}
}

// TestLogsEndpoint tests the logs API response.
func TestLogsEndpoint(t *testing.T) {
	server := newTestServer()

	req := httptest.NewRequest("GET", "/api/logs?token=test-token-12345", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check that response contains logs
	if logs, ok := resp["logs"]; ok {
		if arr, ok := logs.([]interface{}); ok {
			if len(arr) != 2 {
				t.Errorf("Expected 2 log entries, got %d", len(arr))
			}
		}
	}
}

// TestLogout tests the logout endpoint.
func TestLogout(t *testing.T) {
	server := newTestServer()

	// First login to get a session
	loginReq := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"token": "test-token-12345"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	server.mux.ServeHTTP(loginW, loginReq)

	if loginW.Code != http.StatusOK {
		t.Fatalf("Login failed with status %d", loginW.Code)
	}

	// Get session cookie
	cookies := loginW.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == SessionCookieName {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("No session cookie returned")
	}

	// Now logout
	logoutReq := httptest.NewRequest("POST", "/api/auth/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutW := httptest.NewRecorder()
	server.mux.ServeHTTP(logoutW, logoutReq)

	if logoutW.Code != http.StatusOK {
		t.Errorf("Expected logout status 200, got %d", logoutW.Code)
	}

	// Verify session is invalidated - trying to access protected endpoint with old cookie should fail
	statusReq := httptest.NewRequest("GET", "/api/node/status", nil)
	statusReq.AddCookie(sessionCookie)
	statusW := httptest.NewRecorder()
	server.mux.ServeHTTP(statusW, statusReq)

	if statusW.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 after logout, got %d", statusW.Code)
	}
}

// TestServerStartStop tests server lifecycle.
func TestServerStartStop(t *testing.T) {
	config := &Config{
		ListenAddr:      "127.0.0.1:0", // Use random port
		AdminToken:      "test-token",
		SessionDuration: time.Hour,
	}
	server := New(config, &mockNodeInfo{})

	// Test initial state
	if server.IsRunning() {
		t.Error("Server should not be running initially")
	}

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	if !server.IsRunning() {
		t.Error("Server should be running after Start()")
	}

	// Double start should fail
	if err := server.Start(); err == nil {
		t.Error("Double start should return error")
	}

	// Stop server
	if err := server.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	if server.IsRunning() {
		t.Error("Server should not be running after Stop()")
	}

	// Double stop should be safe
	if err := server.Stop(); err != nil {
		t.Error("Double stop should not return error")
	}
}

// BenchmarkHealthEndpoint benchmarks the health endpoint.
func BenchmarkHealthEndpoint(b *testing.B) {
	server := newTestServer()
	req := httptest.NewRequest("GET", "/api/health", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
	}
}

// BenchmarkAuthenticatedEndpoint benchmarks an authenticated endpoint.
func BenchmarkAuthenticatedEndpoint(b *testing.B) {
	server := newTestServer()
	req := httptest.NewRequest("GET", "/api/node/status?token=test-token-12345", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
	}
}

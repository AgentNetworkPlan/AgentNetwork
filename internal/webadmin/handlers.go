package webadmin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// Handlers contains all HTTP request handlers.
type Handlers struct {
	server *Server
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(server *Server) *Handlers {
	return &Handlers{server: server}
}

// LoginRequest represents a login request.
type LoginRequest struct {
	Token string `json:"token"`
}

// LoginResponse represents a login response.
type LoginResponse struct {
	Success   bool   `json:"success"`
	SessionID string `json:"session_id,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	Error     string `json:"error,omitempty"`
}

// HandleLogin handles login requests.
func (h *Handlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, LoginResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// Validate token and create session
	session, err := h.server.auth.CreateSession(req.Token, r.RemoteAddr, r.UserAgent())
	if err != nil {
		WriteJSON(w, http.StatusUnauthorized, LoginResponse{
			Success: false,
			Error:   "Invalid token",
		})
		return
	}

	// Set session cookie (using session ID, not the token)
	http.SetCookie(w, &http.Cookie{
		Name:     TokenCookieName,
		Value:    session.ID,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	WriteJSON(w, http.StatusOK, LoginResponse{
		Success:   true,
		SessionID: session.ID,
		ExpiresAt: session.ExpiresAt.Format(time.RFC3339),
	})
}

// HandleLogout handles logout requests.
func (h *Handlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// Get and invalidate session
	cookie, err := r.Cookie(TokenCookieName)
	if err == nil && cookie.Value != "" {
		h.server.auth.DeleteSession(cookie.Value)
	}

	// Clear the token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     TokenCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// HandleTokenRefresh handles token refresh requests.
func (h *Handlers) HandleTokenRefresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(TokenCookieName)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "No token found")
		return
	}

	// Validate current token
	if h.server.auth.ValidateToken(cookie.Value) {
		// Extend cookie expiration
		expiresAt := time.Now().Add(24 * time.Hour)
		http.SetCookie(w, &http.Cookie{
			Name:     TokenCookieName,
			Value:    cookie.Value,
			Path:     "/",
			Expires:  expiresAt,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"success":    true,
			"expires_at": expiresAt.Format(time.RFC3339),
		})
		return
	}

	WriteError(w, http.StatusUnauthorized, "Token expired or invalid")
}

// HandleHealth handles health check requests.
func (h *Handlers) HandleHealth(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "1.0.0",
	})
}

// HandleNodeStatus handles node status requests.
func (h *Handlers) HandleNodeStatus(w http.ResponseWriter, r *http.Request) {
	if h.server.nodeInfo == nil {
		WriteError(w, http.StatusServiceUnavailable, "Node info not available")
		return
	}

	status := h.server.nodeInfo.GetNodeStatus()
	if status == nil {
		WriteError(w, http.StatusServiceUnavailable, "Unable to get node status")
		return
	}

	WriteJSON(w, http.StatusOK, status)
}

// HandlePeers handles peer list requests.
func (h *Handlers) HandlePeers(w http.ResponseWriter, r *http.Request) {
	if h.server.nodeInfo == nil {
		WriteError(w, http.StatusServiceUnavailable, "Node info not available")
		return
	}

	peers := h.server.nodeInfo.GetPeers()
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"count": len(peers),
		"peers": peers,
	})
}

// HandleConfig handles config requests.
func (h *Handlers) HandleConfig(w http.ResponseWriter, r *http.Request) {
	// Return sanitized config (no secrets)
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"listen_addr": h.server.config.ListenAddr,
		"cors":        h.server.config.EnableCORS,
	})
}

// HandleTopology handles network topology requests.
func (h *Handlers) HandleTopology(w http.ResponseWriter, r *http.Request) {
	topology := h.server.topology.GetTopology()
	WriteJSON(w, http.StatusOK, topology)
}

// HandleEndpoints handles API endpoints list requests.
func (h *Handlers) HandleEndpoints(w http.ResponseWriter, r *http.Request) {
	if h.server.nodeInfo == nil {
		WriteError(w, http.StatusServiceUnavailable, "Node info not available")
		return
	}

	endpoints := h.server.nodeInfo.GetHTTPAPIEndpoints()
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"count":     len(endpoints),
		"endpoints": endpoints,
	})
}

// HandleLogs handles log retrieval requests.
func (h *Handlers) HandleLogs(w http.ResponseWriter, r *http.Request) {
	if h.server.nodeInfo == nil {
		WriteError(w, http.StatusServiceUnavailable, "Node info not available")
		return
	}

	// Parse limit parameter
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	logs := h.server.nodeInfo.GetRecentLogs(limit)
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"count": len(logs),
		"logs":  logs,
	})
}

// HandleStats handles network statistics requests.
func (h *Handlers) HandleStats(w http.ResponseWriter, r *http.Request) {
	if h.server.nodeInfo == nil {
		WriteError(w, http.StatusServiceUnavailable, "Node info not available")
		return
	}

	stats := h.server.nodeInfo.GetNetworkStats()
	if stats == nil {
		WriteError(w, http.StatusServiceUnavailable, "Unable to get network stats")
		return
	}

	WriteJSON(w, http.StatusOK, stats)
}

// HandleIndex serves the main index page (fallback when no static files).
func (h *Handlers) HandleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>DAAN Admin</title>
    <style>
        body { font-family: system-ui, -apple-system, sans-serif; background: #1a1a2e; color: #eee; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; }
        .container { text-align: center; }
        h1 { color: #4fc3f7; }
        p { color: #999; }
        a { color: #4fc3f7; text-decoration: none; }
        a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🌐 DAAN Admin Panel</h1>
        <p>Frontend not yet built. Run <code>make build-admin</code> to build the Vue.js frontend.</p>
        <p>API available at <a href="/api/health">/api/health</a></p>
    </div>
</body>
</html>`))
}

// WebSocket handlers

// HandleWSTopology handles WebSocket connections for topology updates.
func (h *Handlers) HandleWSTopology(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &WSClient{
		conn:     conn,
		send:     make(chan []byte, 256),
		channel:  "topology",
		hub:      h.server.wsHub,
	}

	h.server.wsHub.register <- client

	go client.writePump()
	go client.readPump()
}

// HandleWSLogs handles WebSocket connections for log streaming.
func (h *Handlers) HandleWSLogs(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &WSClient{
		conn:     conn,
		send:     make(chan []byte, 256),
		channel:  "logs",
		hub:      h.server.wsHub,
	}

	h.server.wsHub.register <- client

	go client.writePump()
	go client.readPump()
}

// HandleWSStats handles WebSocket connections for stats updates.
func (h *Handlers) HandleWSStats(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &WSClient{
		conn:     conn,
		send:     make(chan []byte, 256),
		channel:  "stats",
		hub:      h.server.wsHub,
	}

	h.server.wsHub.register <- client

	go client.writePump()
	go client.readPump()
}

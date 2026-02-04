package webadmin

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// TokenCookieName is the name of the token cookie.
const TokenCookieName = "daan_token"

// GenerateToken generates a new random admin token.
func GenerateToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based token if random fails
		return hex.EncodeToString([]byte(time.Now().String()))[:32]
	}
	return hex.EncodeToString(b)
}

// Session represents an authenticated session.
type Session struct {
	ID        string    `json:"id"`
	Token     string    `json:"-"` // The token used to create this session
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
}

// IsExpired returns whether the session has expired.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// AuthManager manages authentication and sessions.
type AuthManager struct {
	adminToken      string
	sessionDuration time.Duration

	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewAuthManager creates a new authentication manager.
func NewAuthManager(adminToken string, sessionDuration time.Duration) *AuthManager {
	am := &AuthManager{
		adminToken:      adminToken,
		sessionDuration: sessionDuration,
		sessions:        make(map[string]*Session),
	}

	// Start session cleanup goroutine
	go am.cleanupExpiredSessions()

	return am
}

// ValidateToken validates the admin token.
func (am *AuthManager) ValidateToken(token string) bool {
	if am.adminToken == "" {
		return false
	}
	return token == am.adminToken
}

// CreateSession creates a new session for a valid token.
func (am *AuthManager) CreateSession(token, ipAddress, userAgent string) (*Session, error) {
	if !am.ValidateToken(token) {
		return nil, ErrInvalidToken
	}

	sessionID, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	session := &Session{
		ID:        sessionID,
		Token:     token,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(am.sessionDuration),
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}

	am.mu.Lock()
	am.sessions[sessionID] = session
	am.mu.Unlock()

	return session, nil
}

// ValidateSession validates a session ID.
func (am *AuthManager) ValidateSession(sessionID string) bool {
	fmt.Printf("[DEBUG] ValidateSession called with sessionID: %s\n", sessionID)
	am.mu.RLock()
	session, exists := am.sessions[sessionID]
	am.mu.RUnlock()

	fmt.Printf("[DEBUG] Session exists: %v\n", exists)
	if !exists {
		fmt.Printf("[DEBUG] Available sessions: %+v\n", am.sessions)
		return false
	}

	if session.IsExpired() {
		fmt.Printf("[DEBUG] Session expired at %v\n", session.ExpiresAt)
		am.mu.Lock()
		delete(am.sessions, sessionID)
		am.mu.Unlock()
		return false
	}

	fmt.Printf("[DEBUG] Session is valid\n")
	return true
}

// GetSession returns a session by ID.
func (am *AuthManager) GetSession(sessionID string) *Session {
	am.mu.RLock()
	defer am.mu.RUnlock()

	session, exists := am.sessions[sessionID]
	if !exists || session.IsExpired() {
		return nil
	}

	return session
}

// DeleteSession deletes a session (logout).
func (am *AuthManager) DeleteSession(sessionID string) {
	am.mu.Lock()
	delete(am.sessions, sessionID)
	am.mu.Unlock()
}

// RefreshSession extends a session's expiration time.
func (am *AuthManager) RefreshSession(sessionID string) bool {
	am.mu.Lock()
	defer am.mu.Unlock()

	session, exists := am.sessions[sessionID]
	if !exists || session.IsExpired() {
		return false
	}

	session.ExpiresAt = time.Now().Add(am.sessionDuration)
	return true
}

// GetActiveSessions returns all active (non-expired) sessions.
func (am *AuthManager) GetActiveSessions() []*Session {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var sessions []*Session
	for _, session := range am.sessions {
		if !session.IsExpired() {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// UpdateToken updates the admin token and invalidates all existing sessions.
func (am *AuthManager) UpdateToken(newToken string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.adminToken = newToken
	am.sessions = make(map[string]*Session) // Clear all sessions
}

// GetToken returns the current admin token.
func (am *AuthManager) GetToken() string {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.adminToken
}

// cleanupExpiredSessions periodically removes expired sessions.
func (am *AuthManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		am.mu.Lock()
		for id, session := range am.sessions {
			if session.IsExpired() {
				delete(am.sessions, id)
			}
		}
		am.mu.Unlock()
	}
}

// generateSessionID generates a random session ID.
func generateSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateAdminToken generates a new random admin token.
func GenerateAdminToken() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

package webadmin

import (
	"testing"
	"time"
)

func TestAuthManager_ValidateToken(t *testing.T) {
	am := NewAuthManager("test-token-123", 24*time.Hour)

	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{"valid token", "test-token-123", true},
		{"invalid token", "wrong-token", false},
		{"empty token", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := am.ValidateToken(tt.token); got != tt.want {
				t.Errorf("ValidateToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthManager_CreateSession(t *testing.T) {
	am := NewAuthManager("test-token", 1*time.Hour)

	// Test valid token
	session, err := am.CreateSession("test-token", "127.0.0.1", "Test Agent")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if session == nil {
		t.Fatal("CreateSession() returned nil session")
	}
	if session.ID == "" {
		t.Error("Session ID should not be empty")
	}
	if session.IPAddress != "127.0.0.1" {
		t.Errorf("Session IPAddress = %v, want %v", session.IPAddress, "127.0.0.1")
	}

	// Test invalid token
	_, err = am.CreateSession("invalid-token", "127.0.0.1", "Test Agent")
	if err != ErrInvalidToken {
		t.Errorf("CreateSession() with invalid token should return ErrInvalidToken, got %v", err)
	}
}

func TestAuthManager_ValidateSession(t *testing.T) {
	am := NewAuthManager("test-token", 1*time.Hour)

	session, _ := am.CreateSession("test-token", "127.0.0.1", "Test Agent")

	// Test valid session
	if !am.ValidateSession(session.ID) {
		t.Error("ValidateSession() should return true for valid session")
	}

	// Test invalid session
	if am.ValidateSession("invalid-session-id") {
		t.Error("ValidateSession() should return false for invalid session")
	}
}

func TestAuthManager_DeleteSession(t *testing.T) {
	am := NewAuthManager("test-token", 1*time.Hour)

	session, _ := am.CreateSession("test-token", "127.0.0.1", "Test Agent")

	// Verify session exists
	if !am.ValidateSession(session.ID) {
		t.Fatal("Session should exist before deletion")
	}

	// Delete session
	am.DeleteSession(session.ID)

	// Verify session is deleted
	if am.ValidateSession(session.ID) {
		t.Error("Session should not exist after deletion")
	}
}

func TestAuthManager_RefreshSession(t *testing.T) {
	am := NewAuthManager("test-token", 1*time.Hour)

	session, _ := am.CreateSession("test-token", "127.0.0.1", "Test Agent")
	originalExpiry := session.ExpiresAt

	time.Sleep(10 * time.Millisecond)

	// Refresh session
	if !am.RefreshSession(session.ID) {
		t.Error("RefreshSession() should return true for valid session")
	}

	// Verify expiry was updated
	refreshed := am.GetSession(session.ID)
	if !refreshed.ExpiresAt.After(originalExpiry) {
		t.Error("Session expiry should be extended after refresh")
	}

	// Test refresh with invalid session
	if am.RefreshSession("invalid-session-id") {
		t.Error("RefreshSession() should return false for invalid session")
	}
}

func TestAuthManager_UpdateToken(t *testing.T) {
	am := NewAuthManager("old-token", 1*time.Hour)

	// Create a session with old token
	session, _ := am.CreateSession("old-token", "127.0.0.1", "Test Agent")

	// Update token
	am.UpdateToken("new-token")

	// Old token should no longer work
	if am.ValidateToken("old-token") {
		t.Error("Old token should not work after update")
	}

	// New token should work
	if !am.ValidateToken("new-token") {
		t.Error("New token should work after update")
	}

	// Old session should be invalidated
	if am.ValidateSession(session.ID) {
		t.Error("Old sessions should be invalidated after token update")
	}
}

func TestGenerateAdminToken(t *testing.T) {
	token1, err := GenerateAdminToken()
	if err != nil {
		t.Fatalf("GenerateAdminToken() error = %v", err)
	}

	if len(token1) != 48 { // 24 bytes = 48 hex chars
		t.Errorf("Token length = %d, want 48", len(token1))
	}

	// Generate another token, should be different
	token2, _ := GenerateAdminToken()
	if token1 == token2 {
		t.Error("Generated tokens should be unique")
	}
}

func TestSession_IsExpired(t *testing.T) {
	// Non-expired session
	session := &Session{
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	if session.IsExpired() {
		t.Error("Session should not be expired")
	}

	// Expired session
	session.ExpiresAt = time.Now().Add(-1 * time.Hour)
	if !session.IsExpired() {
		t.Error("Session should be expired")
	}
}

func TestAuthManager_GetActiveSessions(t *testing.T) {
	am := NewAuthManager("test-token", 1*time.Hour)

	// Create multiple sessions
	am.CreateSession("test-token", "127.0.0.1", "Agent 1")
	am.CreateSession("test-token", "127.0.0.2", "Agent 2")

	sessions := am.GetActiveSessions()
	if len(sessions) != 2 {
		t.Errorf("GetActiveSessions() returned %d sessions, want 2", len(sessions))
	}
}

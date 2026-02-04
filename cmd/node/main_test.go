package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractPort(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		expected int
	}{
		{"port only", ":8080", 8080},
		{"host and port", "localhost:8080", 8080},
		{"full address", "127.0.0.1:18345", 18345},
		{"empty", "", 0},
		{"invalid", "invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPort(tt.addr)
			if result != tt.expected {
				t.Errorf("extractPort(%q) = %d, expected %d", tt.addr, result, tt.expected)
			}
		})
	}
}

func TestBoolToStatus(t *testing.T) {
	if boolToStatus(true) != "✅" {
		t.Error("boolToStatus(true) should return ✅")
	}
	if boolToStatus(false) != "❌" {
		t.Error("boolToStatus(false) should return ❌")
	}
}

func TestLoadOrGenerateToken(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "daan-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// First call should generate a new token
	token1 := loadOrGenerateToken(tmpDir)
	if token1 == "" {
		t.Error("Expected non-empty token")
	}
	if len(token1) != 32 {
		t.Errorf("Expected token length 32, got %d", len(token1))
	}

	// Second call should return the same token
	token2 := loadOrGenerateToken(tmpDir)
	if token2 != token1 {
		t.Errorf("Expected same token, got %q vs %q", token1, token2)
	}

	// Verify token file was created
	tokenPath := filepath.Join(tmpDir, "admin_token")
	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		t.Error("Token file should have been created")
	}
}

func TestGenerateAndSaveToken(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "daan-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate token
	token1 := generateAndSaveToken(tmpDir)
	if token1 == "" {
		t.Error("Expected non-empty token")
	}

	// Generate a new token (should be different)
	token2 := generateAndSaveToken(tmpDir)
	if token2 == "" {
		t.Error("Expected non-empty token")
	}
	if token2 == token1 {
		t.Log("Warning: tokens might be the same by chance, but very unlikely")
	}

	// Verify the file contains the latest token
	data, err := os.ReadFile(filepath.Join(tmpDir, "admin_token"))
	if err != nil {
		t.Fatalf("Failed to read token file: %v", err)
	}
	if string(data) != token2 {
		t.Errorf("Token file should contain latest token")
	}
}

func TestGetASCIILogo(t *testing.T) {
	logo := getASCIILogo()
	if logo == "" {
		t.Error("Expected non-empty logo")
	}
	// Check for key elements
	if len(logo) < 100 {
		t.Error("Logo seems too short")
	}
}

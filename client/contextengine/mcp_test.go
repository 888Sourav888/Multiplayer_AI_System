package contextengine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadDBPathFromRules(t *testing.T) {
	// Create a temporary workspace directory
	tmpDir, err := os.MkdirTemp("", "mpai_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", tmpDir)
	}
	defer os.RemoveAll(tmpDir)

	// Create .cursor/rules directory
	rulesDir := filepath.Join(tmpDir, ".cursor", "rules")
	err = os.MkdirAll(rulesDir, 0755)
	if err != nil {
		t.Fatalf("failed to create .cursor/rules: %v", err)
	}

	// Write mock multiplayer.mdc file
	mdcContent := `# Multiplayer AI Rules
# This project is part of a live multiplayer session: test-session.
# Session ID: mock-session-123
# Folder Hash: mock-hash-abc
# DB Path: C:/Users/test/.mpai/shared context/mock-session-123_mock-hash-abc/multiplayer_ai.db

# You MUST consult the multiplayer-ai MCP server before starting tasks or proposing files changes.
`
	err = os.WriteFile(filepath.Join(rulesDir, "multiplayer.mdc"), []byte(mdcContent), 0644)
	if err != nil {
		t.Fatalf("failed to write mdc file: %v", err)
	}

	// Test readDBPathFromRules
	dbPath, sessionID, err := readDBPathFromRules(tmpDir)
	if err != nil {
		t.Fatalf("readDBPathFromRules failed: %v", err)
	}

	expectedDBPath := "C:/Users/test/.mpai/shared context/mock-session-123_mock-hash-abc/multiplayer_ai.db"
	expectedSessionID := "mock-session-123"

	if dbPath != expectedDBPath {
		t.Errorf("expected DB path %q, got %q", expectedDBPath, dbPath)
	}
	if sessionID != expectedSessionID {
		t.Errorf("expected Session ID %q, got %q", expectedSessionID, sessionID)
	}
}

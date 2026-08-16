package contextengine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type MCPGlobalConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// RegisterMCPServer auto-registers the current executable in the Antigravity IDE global config
func RegisterMCPServer() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	geminiConfigDir := filepath.Join(home, ".gemini", "config")
	if err := os.MkdirAll(geminiConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", geminiConfigDir, err)
	}

	configPath := filepath.Join(geminiConfigDir, "mcp_config.json")
	var mcpConfig MCPGlobalConfig
	mcpConfig.MCPServers = make(map[string]MCPServerConfig)

	// Check if config file exists and read it
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err == nil && len(data) > 0 {
			// Try to unmarshal. If it fails, we will just overwrite it.
			_ = json.Unmarshal(data, &mcpConfig)
		}
	}

	if mcpConfig.MCPServers == nil {
		mcpConfig.MCPServers = make(map[string]MCPServerConfig)
	}

	// Get our own absolute executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable path: %w", err)
	}
	execPathAbs, err := filepath.Abs(execPath)
	if err != nil {
		execPathAbs = execPath
	}

	// Use forward slashes for Antigravity compatibility on Windows
	execPathClean := filepath.ToSlash(execPathAbs)

	// Add or update the multiplayer-ai server config
	mcpConfig.MCPServers["multiplayer-ai"] = MCPServerConfig{
		Command: execPathClean,
		Args:    []string{"mcp"},
		Env:     make(map[string]string),
	}

	// Write back the updated JSON configuration
	newData, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize MCP config: %w", err)
	}

	err = os.WriteFile(configPath, newData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write MCP config file %s: %w", configPath, err)
	}

	fmt.Printf("[Auto-Config] Registered Multiplayer-AI MCP server in Antigravity IDE global config:\n  %s\n", configPath)
	return nil
}

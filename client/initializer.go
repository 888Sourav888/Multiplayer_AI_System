package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds the configuration values for the client application.
type Config struct {
	LowerEnvBackendURL string `json:"lowerEnvBackendURL"`
	ProdBackendURL     string `json:"prodBackendURL"`
}

// InitializeApp loads the configuration from config.json and returns the Config dictionary/struct.
func InitializeApp() (*Config, error) {
	const configPath = "config.json"
	
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %s: %w", configPath, err)
	}
	defer file.Close()

	var cfg Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	return &cfg, nil
}

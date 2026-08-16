package main

import (
	"flag"
	"fmt"
	"os"

	"multiplayer_ai_client/contextengine"
	"multiplayer_ai_client/menu"
	"multiplayer_ai_client/ui"
)

func main() {
	// Enable ANSI/VT100 on Windows
	ui.InitTerminal()

	// Intercept MCP server execution before parsing normal flags
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		contextengine.RunMCPServer()
		return
	}

	// Print branded header
	ui.Header("v1.0.0")

	// Auto-register MCP server configuration in Antigravity
	spin := ui.NewSpinner("Registering MCP server…")
	if err := contextengine.RegisterMCPServer(); err != nil {
		spin.StopError(fmt.Sprintf("Failed to auto-register MCP server: %v", err))
	} else {
		spin.Stop()
	}

	userIDFlag := flag.String("user", "", "User ID to run the client for")
	flag.Parse()

	userID := *userIDFlag
	if userID == "" {
		// Fallback to positional argument for backward compatibility
		if flag.NArg() > 0 {
			userID = flag.Arg(0)
		}
	}

	if userID == "" {
		ui.Error("Missing User ID.")
		fmt.Println()
		fmt.Println("  " + ui.Bold + "Usage:" + ui.Reset)
		fmt.Println("    go run . --user <user_id>")
		fmt.Println("    go run . <user_id>")
		fmt.Println()
		os.Exit(1)
	}

	// Load configuration dictionary
	cfgSpin := ui.NewSpinner("Loading configuration…")
	cfg, err := InitializeApp()
	if err != nil {
		cfgSpin.StopError(fmt.Sprintf("Initialization error: %v", err))
		os.Exit(1)
	}
	cfgSpin.Stop()

	// Determine backend URL from config
	backendURL := cfg.ProdBackendURL
	if backendURL == "" {
		backendURL = cfg.LowerEnvBackendURL
	}

	if backendURL == "" {
		ui.Error("No backend URL configured in config.json")
		os.Exit(1)
	}

	ui.StartupInfo(userID, backendURL)

	// Initialize the modular components
	apiClient := menu.NewAPIClient(backendURL)
	menuService := menu.NewMenuService(apiClient)

	// Display menu
	menu.ShowMenu(userID, menuService)
}

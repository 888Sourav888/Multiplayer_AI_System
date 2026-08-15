package main

import (
	"flag"
	"fmt"
	"os"

	"multiplayer_ai_client/menu"
)

func main() {
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
		fmt.Println("Error: Missing User ID.")
		fmt.Println("Usage:")
		fmt.Println("  go run . --user <user_id>")
		fmt.Println("  go run . <user_id>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Load configuration dictionary
	cfg, err := InitializeApp()
	if err != nil {
		fmt.Printf("Initialization error: %v\n", err)
		os.Exit(1)
	}

	// Determine backend URL from config
	backendURL := cfg.ProdBackendURL
	if backendURL == "" {
		backendURL = cfg.LowerEnvBackendURL
	}

	if backendURL == "" {
		fmt.Println("Error: No backend URL configured in config.json")
		os.Exit(1)
	}

	fmt.Printf("Starting client for User ID: %s (Backend: %s)\n\n", userID, backendURL)

	// Initialize the modular components
	apiClient := menu.NewAPIClient(backendURL)
	menuService := menu.NewMenuService(apiClient)

	// Display menu
	menu.ShowMenu(userID, menuService)
}

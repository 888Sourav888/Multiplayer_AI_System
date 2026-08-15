package session

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// RunSessionFlow starts the live interactive multiplayer session CLI.
func RunSessionFlow(userID string, sessionID string, baseHTTPURL string) {
	reader := bufio.NewReader(os.Stdin)

	backend := NewSessionBackend(baseHTTPURL)
	service := NewSessionService(backend)
	defer service.Close()

	fmt.Printf("\nConnecting to live multiplayer session %s...\n", sessionID)
	
	// Fetch initial session info
	sInfo, err := service.GetSessionInfo(sessionID)
	if err != nil {
		fmt.Printf("✗ Failed to get session info: %v\n\n", err)
		return
	}

	// Connect to WS
	msgChan, err := service.ConnectSession(sessionID)
	if err != nil {
		fmt.Printf("✗ Connection failed: %v\n\n", err)
		return
	}

	fmt.Println("✓ Successfully connected to live session room!")
	fmt.Printf("Active Session Name: %s\n\n", sInfo.Name)

	// Goroutine to monitor incoming broadcasts asynchronously
	stopPrintChan := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopPrintChan:
				return
			case msg, ok := <-msgChan:
				if !ok {
					fmt.Println("\n[Live Session Alert] Connection closed by remote server.")
					return
				}
				// Format incoming broadcast alerts nicely
				if msg.Type == "PATCH_BROADCAST" || msg.Status == "PATCH_BROADCAST" {
					fmt.Printf("\n[Incoming Broadcast] Patch received from sender: %s\n", msg.SenderID)
					if msg.Message != "" {
						fmt.Printf("  Message: %s\n", msg.Message)
					}
					fmt.Printf("  Patches payload: %v\n\n", msg.Patches)
					fmt.Print("Select: ") // Reprint the select prompt after printing message
				} else if msg.Status != "" {
					fmt.Printf("\n[Session Event] Status update: %s - %s\n\n", msg.Status, msg.Message)
					fmt.Print("Select: ")
				}
			}
		}
	}()

	// Loop for session menu actions
	for {
		fmt.Printf("=== Active Session: %s ===\n", sInfo.Name)
		fmt.Println("1. Show session details")
		fmt.Println("2. Send simulated file patch")
		fmt.Println("3. Leave session")
		fmt.Print("Select: ")

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			break
		}

		choice := strings.TrimSpace(input)
		switch choice {
		case "1":
			// Refresh info and print
			sInfo, err = service.GetSessionInfo(sessionID)
			if err != nil {
				fmt.Printf("Error refreshing session: %v\n\n", err)
			} else {
				fmt.Println("\n--- Live Session Details ---")
				fmt.Println(service.FormatSessionInfo(sInfo))
				fmt.Println()
			}
		case "2":
			fmt.Print("\nEnter file path (e.g. main.go): ")
			pathInput, _ := reader.ReadString('\n')
			filePath := strings.TrimSpace(pathInput)

			fmt.Print("Enter patch content (e.g. + func main()): ")
			contentInput, _ := reader.ReadString('\n')
			content := strings.TrimSpace(contentInput)

			if filePath == "" || content == "" {
				fmt.Println("Cancelled. Empty inputs not allowed.\n")
				continue
			}

			err := service.SendSimulatedPatch(sessionID, userID, filePath, content)
			if err != nil {
				fmt.Printf("✗ Failed to send patch: %v\n\n", err)
			} else {
				fmt.Println("✓ Patch sent successfully!\n")
			}
		case "3":
			fmt.Println("Leaving session...")
			close(stopPrintChan)
			return
		default:
			fmt.Println("Invalid option.\n")
		}
	}
}

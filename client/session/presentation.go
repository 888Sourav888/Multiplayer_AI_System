package session

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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

	var watcher *HighPrecisionWatcher

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
					if msg.SenderID != userID {
						fmt.Printf("\n[Incoming Sync] Applying remote patch from sender: %s\n", msg.SenderID)
						for _, patch := range msg.Patches {
							absPath, _ := filepath.Abs(patch.FilePathFromRoot)
							if watcher != nil {
								watcher.IgnorePath(absPath)
							}
							err := ApplyPatch(".", patch.FilePathFromRoot, patch.Operation, patch.IsWholeFile, patch.ContentDelta)
							if err != nil {
								fmt.Printf("   ✗ Failed to apply patch for %s: %v\n", patch.FilePathFromRoot, err)
							} else {
								fmt.Printf("   ✓ Successfully applied patch for %s\n", patch.FilePathFromRoot)
							}
						}
						fmt.Print("Select: ")
					}
				} else if msg.Status != "" {
					fmt.Printf("\n[Session Event] Status update: %s - %s\n\n", msg.Status, msg.Message)
					fmt.Print("Select: ")
				}
			}
		}
	}()

	// Start File System Watcher
	callback := func(filePathFromRoot string, fileName string, fileExtension string, operation string, sizeBytes int64, modifier string, isAiEdit bool, isRevert bool, isWholeFile bool, contentDeltaJSON string) {
		patch := FilePatchItem{
			FilePathFromRoot: filePathFromRoot,
			FileName:         fileName,
			FileExtension:    fileExtension,
			Operation:        operation,
			SizeBytes:        sizeBytes,
			Modifier:         modifier,
			IsAiEdit:         isAiEdit,
			IsRevert:         isRevert,
			IsWholeFile:      isWholeFile,
			ContentDelta:     contentDeltaJSON,
		}

		msg := WSMessage{
			Type:      "PATCH_TRANSFER",
			SessionID: sessionID,
			SenderID:  userID,
			Message:   fmt.Sprintf("Sync change for %s", filePathFromRoot),
			Patches:   []FilePatchItem{patch},
			Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
		}

		_ = service.SendWSMessage(msg)
	}

	w, wErr := NewHighPrecisionWatcher(".", callback)
	if wErr != nil {
		fmt.Printf("✗ Warning: Failed to initialize file watcher: %v\n", wErr)
	} else {
		watcher = w
		if err := watcher.AddRecursive("."); err != nil {
			fmt.Printf("✗ Warning: File watcher AddRecursive failed: %v\n", err)
		}

		watcherCtx, watcherCancel := context.WithCancel(context.Background())
		defer watcherCancel()
		go func() {
			if err := watcher.Start(watcherCtx); err != nil {
				// Watcher exited
			}
		}()
		fmt.Println("✓ Real-time directory file synchronization watcher active!")
	}

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

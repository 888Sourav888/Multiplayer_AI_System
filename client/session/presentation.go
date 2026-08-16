package session

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func createWorkspaceRules(sessionName string) {
	cursorRulesContent := fmt.Sprintf(`# Multiplayer AI Rules
# This project is part of a live multiplayer session: %s.

# You MUST consult the multiplayer-ai MCP server before starting tasks or proposing files changes.
# Call the "get_session_messages" tool to synchronize your context with other participants.
# Never skip checking the shared context.
`, sessionName)

	_ = os.WriteFile(".cursorrules", []byte(cursorRulesContent), 0644)
	_ = os.WriteFile(".clinerules", []byte(cursorRulesContent), 0644)
	_ = os.MkdirAll(".cursor/rules", 0755)
	_ = os.WriteFile(".cursor/rules/multiplayer.mdc", []byte(cursorRulesContent), 0644)
}

func removeWorkspaceRules() {
	_ = os.Remove(".cursorrules")
	_ = os.Remove(".clinerules")
	_ = os.Remove(".cursor/rules/multiplayer.mdc")
	fmt.Println("\n✓ Local workspace AI rules cleaned up.")
}

// RunSessionFlow starts the live interactive multiplayer session CLI.
func RunSessionFlow(userID string, sessionID string, baseHTTPURL string, engine ContextEngine) {
	reader := bufio.NewReader(os.Stdin)

	// Set up deferred cleanup of rules
	defer removeWorkspaceRules()

	// Capture interrupt signals to clean up rule files on abrupt exits (like Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		removeWorkspaceRules()
		os.Exit(0)
	}()

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

	// Initialize active session context if engine is provided
	if engine != nil {
		localPath, _ := filepath.Abs(".")
		err = engine.SetActiveSession(sInfo, userID, localPath)
		if err != nil {
			fmt.Printf("✗ Warning: Failed to set active session in SQLite: %v\n", err)
		}
	}

	// Connect to WS
	msgChan, err := service.ConnectSession(sessionID, userID)
	if err != nil {
		fmt.Printf("✗ Connection failed: %v\n\n", err)
		return
	}

	fmt.Println("✓ Successfully connected to live session room!")
	fmt.Printf("Active Session Name: %s\n\n", sInfo.Name)

	// Send CONTEXT_REQUEST to get session history from peers
	reqMsg := WSMessage{
		Type:      "CONTEXT_REQUEST",
		SessionID: sessionID,
		SenderID:  userID,
		Message:   "Requesting shared AI context space history",
		Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
	}
	_ = service.SendWSMessage(reqMsg)

	// Generate .cursorrules and .clinerules files
	createWorkspaceRules(sInfo.Name)
	fmt.Println("✓ Local workspace AI rules generated (.cursorrules, .clinerules, .cursor/rules/multiplayer.mdc)")

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
				if msg.Type == "CONTEXT_REQUEST" {
					// If we are the owner of the session, respond with our SQLite history
					if sInfo.OwnerID == userID && engine != nil {
						msgs, err := engine.GetSessionMessages(sessionID, 100)
						if err == nil {
							bytes, _ := json.Marshal(msgs)
							respMsg := WSMessage{
								Type:      "CONTEXT_RESPONSE",
								SessionID: sessionID,
								SenderID:  userID,
								Message:   string(bytes),
								Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
							}
							_ = service.SendWSMessage(respMsg)
						}
					}
				} else if msg.Type == "CONTEXT_RESPONSE" {
					// Parse the messages and save them locally
					if msg.SenderID != userID && engine != nil {
						var msgs []AIMessage
						if err := json.Unmarshal([]byte(msg.Message), &msgs); err == nil {
							for _, m := range msgs {
								_ = engine.SaveAIMessage(m.ID, m.SessionID, m.SenderID, m.Modifier, m.Content, m.StepIndex, m.CreatedAt)
							}
							fmt.Printf("\n[Incoming Sync] Synchronized %d AI messages from session owner.\nSelect: ", len(msgs))
						}
					}
				} else if msg.Type == "PATCH_BROADCAST" || msg.Status == "PATCH_BROADCAST" {
					if msg.SenderID != userID {
						// Check if the patches contain an AI message
						isAiMsg := false
						for _, patch := range msg.Patches {
							if patch.Operation == "AI_MESSAGE" {
								isAiMsg = true
								if engine != nil {
									msgID := fmt.Sprintf("broadcast-%s-%d", msg.SenderID, time.Now().UnixNano())
									_ = engine.SaveAIMessage(msgID, sessionID, msg.SenderID, patch.Modifier, patch.ContentDelta, -1, time.Now())
								}
								fmt.Printf("\n--- AI Broadcast Message from %s (via %s) ---\n%s\n---------------------------------------------\n",
									msg.SenderID, patch.Modifier, patch.ContentDelta)
							}
						}

						if !isAiMsg {
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
									if engine != nil {
										changeID := fmt.Sprintf("fc-in-%s-%d", msg.SenderID, time.Now().UnixNano())
										_ = engine.SaveFileChange(changeID, sessionID, msg.SenderID, patch.FilePathFromRoot, patch.Operation, patch.Modifier, patch.IsAiEdit, patch.ContentDelta, time.Now())
									}
								}
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

		if engine != nil {
			changeID := fmt.Sprintf("fc-out-%d", time.Now().UnixNano())
			_ = engine.SaveFileChange(changeID, sessionID, userID, filePathFromRoot, operation, modifier, isAiEdit, contentDeltaJSON, time.Now())
		}

		_ = service.SendWSMessage(msg)
	}

	w, wErr := NewHighPrecisionWatcher(".", callback)
	if wErr != nil {
		if engine != nil {
			engine.LogError("Failed to initialize file watcher: %v", wErr)
		}
	} else {
		watcher = w
		if err := watcher.AddRecursive("."); err != nil {
			if engine != nil {
				engine.LogError("File watcher AddRecursive failed: %v", err)
			}
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

	// Start AI Transcript Poller
	broadcastFn := func(modifier string, content string, stepIndex int) {
		patchItem := FilePatchItem{
			FilePathFromRoot: "ai_transcript.jsonl",
			FileName:         "transcript.jsonl",
			FileExtension:    ".jsonl",
			Operation:        "AI_MESSAGE",
			SizeBytes:        int64(len(content)),
			Modifier:         modifier,
			IsAiEdit:         false,
			IsWholeFile:      true,
			ContentDelta:     content,
			FileChanges:      content,
		}
		msg := WSMessage{
			Type:      "PATCH_TRANSFER",
			SessionID: sessionID,
			SenderID:  userID,
			Message:   fmt.Sprintf("AI step %d", stepIndex),
			Patches:   []FilePatchItem{patchItem},
			Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
		}
		_ = service.SendWSMessage(msg)
	}

	var pollerCancel context.CancelFunc
	if engine != nil {
		pollerCancel = engine.StartPoller(context.Background(), sessionID, userID, ".", broadcastFn)
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
			if pollerCancel != nil {
				pollerCancel()
			}
			return
		default:
			fmt.Println("Invalid option.\n")
		}
	}
}

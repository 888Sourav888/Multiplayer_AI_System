package session

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"multiplayer_ai_client/ui"
)

func getWDHash(wd string) string {
	h := md5.New()
	abs, err := filepath.Abs(wd)
	if err != nil {
		abs = wd
	}
	_, _ = io.WriteString(h, strings.ToLower(filepath.ToSlash(abs)))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

func createWorkspaceHooks(sessionID, wd string) {
	_ = os.MkdirAll(".agents", 0755)

	// Determine the path to HooksModule relative to the running client executable
	hooksSrcDir := ""
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		candidate := filepath.Join(execDir, "..", "HooksModule")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			hooksSrcDir = candidate
		}
	}

	// Fallback to checking relative to the current working directory
	if hooksSrcDir == "" {
		if info, err := os.Stat("HooksModule"); err == nil && info.IsDir() {
			hooksSrcDir = "HooksModule"
		} else if info, err := os.Stat("../HooksModule"); err == nil && info.IsDir() {
			hooksSrcDir = "../HooksModule"
		}
	}

	if hooksSrcDir == "" {
		ui.Warn("HooksModule source directory not found. Skipping hook setup.")
		return
	}

	absHooksSrcDir, _ := filepath.Abs(hooksSrcDir)
	hookExeName := "fetch_latest_hook.exe"
	hookExePath := filepath.Join(absHooksSrcDir, hookExeName)

	if _, err := os.Stat(hookExePath); os.IsNotExist(err) {
		ui.Info("Building hooks executable...")
		buildCmd := exec.Command("go", "build", "-o", hookExePath, ".")
		buildCmd.Dir = absHooksSrcDir
		if out, err := buildCmd.CombinedOutput(); err != nil {
			ui.Warnf("Failed to build hooks executable: %v\nOutput: %s", err, string(out))
			return
		}
		ui.Success("Hooks executable built successfully!")
	}

	exePathClean := filepath.ToSlash(hookExePath)
	hash := getWDHash(wd)

	hooksJSONContent := fmt.Sprintf(`{
  "fetch-latest-messages": {
    "PreInvocation": [
      {
        "type": "command",
        "command": "\"%s\" \"%s\" \"%s\""
      }
    ]
  }
}
`, exePathClean, sessionID, hash)

	hooksJSONPath := filepath.Join(".agents", "hooks.json")
	_ = os.WriteFile(hooksJSONPath, []byte(hooksJSONContent), 0644)
	ui.Success("Local workspace AI lifecycle hooks initialized (.agents/hooks.json)")
}

func createWorkspaceRules(sessionID, sessionName, wd string) {
	home, err := os.UserHomeDir()
	var dbPath string
	var hash string
	if err == nil {
		hash = getWDHash(wd)
		dbDir := filepath.Join(home, ".mpai", "shared context", fmt.Sprintf("%s_%s", sessionID, hash))
		dbPath = filepath.ToSlash(filepath.Join(dbDir, "multiplayer_ai.db"))
	}

	cursorRulesContent := fmt.Sprintf(`# Multiplayer AI Rules
# This project is part of a live multiplayer session: %s.
# Session ID: %s
# Folder Hash: %s
# DB Path: %s
`, sessionName, sessionID, hash, dbPath)

	_ = os.WriteFile(".cursorrules", []byte(cursorRulesContent), 0644)
	_ = os.WriteFile(".clinerules", []byte(cursorRulesContent), 0644)
	_ = os.MkdirAll(".cursor/rules", 0755)
	_ = os.WriteFile(".cursor/rules/multiplayer.mdc", []byte(cursorRulesContent), 0644)
	createWorkspaceHooks(sessionID, wd)
}

func removeWorkspaceRules() {
	_ = os.Remove(".cursorrules")
	_ = os.Remove(".clinerules")
	_ = os.Remove(".cursor/rules/multiplayer.mdc")
	_ = os.Remove(".agents/hooks.json")
	ui.Success("Local workspace AI rules and hooks cleaned up.")
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

	ui.ConnectingMsg(sessionID)

	// Fetch initial session info
	connSpin := ui.NewSpinner("Fetching session metadata…")
	sInfo, err := service.GetSessionInfo(sessionID)
	if err != nil {
		connSpin.StopError(fmt.Sprintf("Failed to get session info: %v", err))
		return
	}
	connSpin.Stop()

	// Initialize active session context if engine is provided
	if engine != nil {
		localPath, _ := filepath.Abs(".")
		err = engine.SetActiveSession(sInfo, userID, localPath)
		if err != nil {
			ui.Warnf("Failed to set active session in SQLite: %v", err)
		}
	}

	// Connect to WS
	wsSpin := ui.NewSpinner("Establishing WebSocket connection…")
	msgChan, err := service.ConnectSession(sessionID, userID)
	if err != nil {
		wsSpin.StopError(fmt.Sprintf("Connection failed: %v", err))
		return
	}
	wsSpin.StopSuccess("Successfully connected to live session room!")

	ui.Infof("Active Session: %s%s%s", ui.Bold+ui.BrightWhite, sInfo.Name, ui.Reset)
	fmt.Println()

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
	createWorkspaceRules(sessionID, sInfo.Name, ".")
	ui.Success("Local workspace AI rules generated (.cursorrules, .clinerules, .cursor/rules/multiplayer.mdc)")

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
					fmt.Println()
					ui.Warn("[Live Session Alert] Connection closed by remote server.")
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
							fmt.Printf("\n%s  ↓ Sync%s  Synchronized %d AI messages from session owner.\n%s  ›%s Select: ",
								ui.BrightCyan+ui.Bold, ui.Reset,
								len(msgs),
								ui.BrightCyan, ui.Reset)
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
								fmt.Printf("\n%s╭─ AI Broadcast%s  from %s%s%s  via %s%s%s\n%s│%s %s\n%s╰%s\n",
									ui.BrightMagenta+ui.Bold, ui.Reset,
									ui.BrightWhite+ui.Bold, msg.SenderID, ui.Reset,
									ui.Dim, patch.Modifier, ui.Reset,
									ui.BrightBlack, ui.Reset,
									patch.ContentDelta,
									ui.BrightBlack, ui.Reset,
								)
							}
						}

						if !isAiMsg {
							fmt.Printf("\n%s  ↓ Patch%s  Applying remote patch from %s%s%s\n",
								ui.BrightCyan+ui.Bold, ui.Reset,
								ui.BrightWhite, msg.SenderID, ui.Reset)
							for _, patch := range msg.Patches {
								absPath, _ := filepath.Abs(patch.FilePathFromRoot)
								if watcher != nil {
									watcher.IgnorePath(absPath)
								}
								err := ApplyPatch(".", patch.FilePathFromRoot, patch.Operation, patch.IsWholeFile, patch.ContentDelta)
								if err != nil {
									ui.Errorf("  Failed to apply patch for %s: %v", patch.FilePathFromRoot, err)
								} else {
									ui.Successf("  Applied patch for %s", patch.FilePathFromRoot)
									if engine != nil {
										changeID := fmt.Sprintf("fc-in-%s-%d", msg.SenderID, time.Now().UnixNano())
										_ = engine.SaveFileChange(changeID, sessionID, msg.SenderID, patch.FilePathFromRoot, patch.Operation, patch.Modifier, patch.IsAiEdit, patch.ContentDelta, time.Now())
									}
								}
							}
							fmt.Printf("%s  ›%s Select: ", ui.BrightCyan, ui.Reset)
						}
					}
				} else if msg.Status != "" {
					fmt.Printf("\n%s  ⚡ Event%s  Status update: %s%s%s — %s\n%s  ›%s Select: ",
						ui.BrightYellow+ui.Bold, ui.Reset,
						ui.BrightWhite, msg.Status, ui.Reset,
						msg.Message,
						ui.BrightCyan, ui.Reset)
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
		ui.Success("Real-time directory file synchronization watcher active!")
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

	fmt.Println()

	// Loop for session menu actions
	for {
		ui.ActiveSessionHeader(sInfo.Name)
		ui.SubMenuItem(1, "Show session details")
		ui.SubMenuItem(2, "Send simulated file patch")
		ui.SubMenuItem(3, "Leave session")
		fmt.Println()
		ui.Prompt("Select")

		input, err := reader.ReadString('\n')
		if err != nil {
			ui.Errorf("Error reading input: %v", err)
			break
		}

		choice := strings.TrimSpace(input)
		switch choice {
		case "1":
			// Refresh info and print
			refreshSpin := ui.NewSpinner("Refreshing session info…")
			sInfo, err = service.GetSessionInfo(sessionID)
			if err != nil {
				refreshSpin.StopError(fmt.Sprintf("Error refreshing session: %v", err))
			} else {
				refreshSpin.Stop()
				fmt.Println()
				fmt.Print(service.FormatSessionInfo(sInfo))
				fmt.Println()
			}
		case "2":
			fmt.Println()
			ui.Prompt("File path (e.g. main.go)")
			pathInput, _ := reader.ReadString('\n')
			filePath := strings.TrimSpace(pathInput)

			ui.Prompt("Patch content (e.g. + func main())")
			contentInput, _ := reader.ReadString('\n')
			content := strings.TrimSpace(contentInput)

			if filePath == "" || content == "" {
				ui.Warn("Cancelled. Empty inputs not allowed.")
				continue
			}

			patchSpin := ui.NewSpinner(fmt.Sprintf("Sending patch for %s…", filePath))
			err := service.SendSimulatedPatch(sessionID, userID, filePath, content)
			if err != nil {
				patchSpin.StopError(fmt.Sprintf("Failed to send patch: %v", err))
			} else {
				patchSpin.StopSuccess("Patch sent successfully!")
			}
			fmt.Println()
		case "3":
			ui.Info("Leaving session…")
			close(stopPrintChan)
			if pollerCancel != nil {
				pollerCancel()
			}
			return
		default:
			ui.Warn("Invalid option.")
		}
	}
}

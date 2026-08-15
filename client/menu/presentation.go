package menu

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"multiplayer_ai_client/session"
)

// ShowMenu displays the CLI menu for the client application and delegates
// action processing to the provided MenuService.
func ShowMenu(userID string, service *MenuService) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("=== Menu ===")
		fmt.Println("1. See my sessions")
		fmt.Println("2. Create session")
		fmt.Println("3. Join session")
		fmt.Println("4. Update session")
		fmt.Println("5. Delete session")
		fmt.Println("6. Exit")
		fmt.Print("Select option: ")

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		choice := strings.TrimSpace(input)
		switch choice {
		case "1":
			handleSeeSessions(reader, userID, service)
		case "2":
			handleCreateSession(reader, userID, service)
		case "3":
			handleJoinSession(reader, userID, service)
		case "4":
			handleUpdateSession(reader, userID, service)
		case "5":
			handleDeleteSession(reader, userID, service)
		case "6":
			fmt.Println("Goodbye!")
			return
		default:
			fmt.Println("Invalid choice, please select again.\n")
		}
	}
}

// handleSeeSessions displays the user's sessions.
func handleSeeSessions(reader *bufio.Reader, userID string, service *MenuService) {
	fmt.Println("\n--- My Sessions ---")
	sessions, err := service.GetUserSessions(userID)
	if err != nil {
		fmt.Printf("Error: %v\n\n", err)
		return
	}
	fmt.Printf("%s\n\n", service.FormatSessionList(sessions))
}

// handleCreateSession prompts for a session name and creates a new session.
func handleCreateSession(reader *bufio.Reader, userID string, service *MenuService) {
	fmt.Println("\n--- Create Session ---")
	fmt.Print("Enter session name (or press Enter to cancel): ")

	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		return
	}

	name := strings.TrimSpace(input)
	if name == "" {
		fmt.Println("Cancelled.\n")
		return
	}

	gitInfo, isGit := GetGitInfo()
	var created *Session

	if isGit {
		fmt.Printf("\nGit repository detected. Creating synchronized session using HEAD...\n")
		fmt.Printf("  URL:    %s\n  Branch: %s\n  Commit: %s\n\n", gitInfo.RepoURL, gitInfo.Branch, gitInfo.CommitSHA)

		created, err = service.CreateSession(userID, name, gitInfo.RepoURL, gitInfo.Branch, gitInfo.CommitSHA)
		if err != nil {
			fmt.Printf("✗ Creation failed: %v\n\n", err)
			return
		}
		fmt.Println("✓ Git-synchronized session created successfully!")
	} else {
		fmt.Println("\nNo Git repository detected. Generating initial workspace snapshot...")
		zipBytes, zErr := ZipDirectory(".")
		if zErr != nil {
			fmt.Printf("✗ Failed to compress workspace: %v\n\n", zErr)
			return
		}

		zipSizeMB := float64(len(zipBytes)) / (1024 * 1024)
		fmt.Printf("Local workspace archive size: %.2f MB\n", zipSizeMB)

		if len(zipBytes) > 5*1024*1024 {
			fmt.Printf("✗ Creation failed: Archive size (%.2f MB) exceeds the 5MB limit.\n\n", zipSizeMB)
			return
		}

		created, err = service.CreateSession(userID, name, "", "", "")
		if err != nil {
			fmt.Printf("✗ Creation failed: %v\n\n", err)
			return
		}

		fmt.Println("Uploading initial workspace snapshot...")
		_, upErr := service.UploadSnapshot(created.ID, zipBytes)
		if upErr != nil {
			fmt.Printf("✗ Warning: Session created but snapshot upload failed: %v\n\n", upErr)
			return
		}
		fmt.Println("✓ Session created and initial snapshot uploaded successfully!")
	}

	fmt.Println(service.FormatSessionDetail(created))
	fmt.Println()
}

// handleJoinSession runs the interactive join session flow.
func handleJoinSession(reader *bufio.Reader, userID string, service *MenuService) {
	fmt.Println("\n--- Join Session ---")
	fmt.Println("  1. Join an active session you own")
	fmt.Println("  2. Join any other session by Session ID")
	fmt.Println("  3. Back to menu")
	fmt.Print("Select: ")

	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading input: %v\n\n", err)
		return
	}

	choice := strings.TrimSpace(input)
	switch choice {
	case "1":
		// List owned sessions
		sessions, err := service.GetUserSessions(userID)
		if err != nil {
			fmt.Printf("✗ Failed to load sessions: %v\n\n", err)
			return
		}

		// Filter out terminated sessions
		var activeSessions []Session
		for _, s := range sessions {
			if s.Status == "ACTIVE" {
				activeSessions = append(activeSessions, s)
			}
		}

		if len(activeSessions) == 0 {
			fmt.Println("No active sessions found to join.\n")
			return
		}

		for i, s := range activeSessions {
			fmt.Printf("  %d. %s  (%s)\n", i+1, s.Name, s.ID)
		}
		fmt.Println()

		sessionIndex := promptForNumber(reader, "Select session to join (number): ", 1, len(activeSessions))
		if sessionIndex == -1 {
			return
		}
		selected := activeSessions[sessionIndex-1]

		joined, err := service.JoinSession(userID, selected.ID)
		if err != nil {
			fmt.Printf("✗ Join failed: %v\n\n", err)
			return
		}

		fmt.Println("✓ Successfully joined session!")
		if !syncOwnerSessionState(joined, userID, service) {
			return
		}
		if !syncMemberSessionState(joined, userID, service) {
			return
		}
		fmt.Println(service.FormatSessionDetail(joined))
		fmt.Println()

		// Start the live session interaction module (REST + WS)
		session.RunSessionFlow(userID, selected.ID, service.GetBaseURL())

	case "2":
		fmt.Print("Enter Session ID (UUID): ")
		inputID, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n\n", err)
			return
		}
		sessionID := strings.TrimSpace(inputID)
		if sessionID == "" {
			fmt.Println("Cancelled.\n")
			return
		}

		joined, err := service.JoinSession(userID, sessionID)
		if err != nil {
			fmt.Printf("✗ Join failed: %v\n\n", err)
			return
		}

		fmt.Println("✓ Successfully joined session!")
		if !syncOwnerSessionState(joined, userID, service) {
			return
		}
		if !syncMemberSessionState(joined, userID, service) {
			return
		}
		fmt.Println(service.FormatSessionDetail(joined))
		fmt.Println()

		// Start the live session interaction module (REST + WS)
		session.RunSessionFlow(userID, sessionID, service.GetBaseURL())

	case "3":
		fmt.Println()
		return
	default:
		fmt.Println("Invalid selection.\n")
	}
}

// syncOwnerSessionState checks if the current user is the owner, and if so,
// synchronizes their local workspace (Git info or Zip snapshot) to the backend.
func syncOwnerSessionState(joined *Session, userID string, service *MenuService) bool {
	if joined.OwnerID != userID {
		return true // Other users do not need to upload state
	}

	fmt.Println("Welcome, Session Owner! Synchronizing latest workspace state to backend...")
	gitInfo, isGit := GetGitInfo()

	if isGit {
		fmt.Printf("Updating Git reference (HEAD @ %s)...\n", gitInfo.CommitSHA[:8])
		updated, err := service.UpdateSessionGitInfo(joined.ID, gitInfo.RepoURL, gitInfo.Branch, gitInfo.CommitSHA)
		if err != nil {
			fmt.Printf("✗ Warning: Git synchronization failed: %v\n", err)
		} else {
			*joined = *updated
			fmt.Println("✓ Git metadata successfully updated on backend!")
		}
	} else {
		fmt.Println("Compressing local workspace code...")
		zipBytes, err := ZipDirectory(".")
		if err != nil {
			fmt.Printf("✗ Failed to compress workspace: %v\n\n", err)
			return false
		}

		zipSizeMB := float64(len(zipBytes)) / (1024 * 1024)
		if len(zipBytes) > 5*1024*1024 {
			fmt.Printf("✗ Cannot join session: Workspace snapshot size (%.2f MB) exceeds 5MB limit.\n\n", zipSizeMB)
			return false
		}

		fmt.Printf("Uploading latest workspace snapshot (%.2f MB)...\n", zipSizeMB)
		_, upErr := service.UploadSnapshot(joined.ID, zipBytes)
		if upErr != nil {
			fmt.Printf("✗ Warning: Snapshot synchronization failed: %v\n", upErr)
		} else {
			fmt.Println("✓ Workspace snapshot synchronized successfully!")
		}
	}
	return true
}

// syncMemberSessionState synchronizes a non-owner member's local workspace.
// If Git session: checks out the designated branch.
// If Zip session: cleans directory and downloads & extracts the latest zip archive.
func syncMemberSessionState(joined *Session, userID string, service *MenuService) bool {
	if joined.OwnerID == userID {
		return true // Owner state is updated in syncOwnerSessionState
	}

	if joined.GitRepoUrl != "" {
		fmt.Printf("Git session detected. Synchronizing codebase...\n")
		// Check if we are inside a local Git repo
		_, isGit := GetGitInfo()
		if !isGit {
			fmt.Println("✗ Error: This is a Git-tracked session, but your local folder is not a Git repository.")
			fmt.Println("Please run 'git init' or clone the project repository first.")
			return false
		}

		fmt.Printf("Switching local workspace branch to: %s\n", joined.GitBranch)
		cmd := exec.Command("git", "checkout", joined.GitBranch)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("✗ Failed to switch branch: %v\n\n", err)
			return false
		}
		fmt.Println("✓ Local workspace successfully switched to branch!")
	} else {
		fmt.Println("Non-Git session detected. Downloading latest workspace snapshot...")
		// If version is 1, maybe no snapshot is uploaded yet. Let's check current version.
		if joined.CurrentVersion < 2 {
			fmt.Println("No workspace snapshots are available for this session yet.")
			return true
		}

		// Download zip bytes
		zipBytes, err := service.DownloadSnapshot(joined.ID, joined.CurrentVersion)
		if err != nil {
			fmt.Printf("✗ Failed to download snapshot from server: %v\n\n", err)
			return false
		}

		fmt.Println("Cleaning local workspace (excluding client binary and config)...")
		if err := CleanDirectory("."); err != nil {
			fmt.Printf("✗ Failed to clean directory: %v\n\n", err)
			return false
		}

		fmt.Println("Extracting workspace snapshot files...")
		if err := UnzipBytes(zipBytes, "."); err != nil {
			fmt.Printf("✗ Failed to extract snapshot files: %v\n\n", err)
			return false
		}
		fmt.Println("✓ Workspace successfully synchronized to latest server snapshot!")
	}
	return true
}


// handleDeleteSession runs the interactive delete sub-flow.
func handleDeleteSession(reader *bufio.Reader, userID string, service *MenuService) {
	fmt.Println("\n--- Delete Session ---")

	// Step 1: Fetch and display sessions
	sessions, err := service.GetUserSessions(userID)
	if err != nil {
		fmt.Printf("Error fetching sessions: %v\n\n", err)
		return
	}
	if len(sessions) == 0 {
		fmt.Println("No sessions found.\n")
		return
	}

	for i, s := range sessions {
		fmt.Printf("  %d. %s  [%s]\n", i+1, s.Name, s.Status)
	}
	fmt.Println()

	// Step 2: Select a session
	sessionIndex := promptForNumber(reader, "Select session to delete (number): ", 1, len(sessions))
	if sessionIndex == -1 {
		return
	}
	selected := sessions[sessionIndex-1]

	// Step 3: Confirm deletion
	fmt.Printf("\n⚠ You are about to delete: %s (%s)\n", selected.Name, selected.ID)
	fmt.Print("This action is permanent. Continue? (y/N): ")

	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		return
	}

	confirm := strings.TrimSpace(strings.ToLower(input))
	if confirm != "y" && confirm != "yes" {
		fmt.Println("Cancelled.\n")
		return
	}

	// Step 4: Delete
	err = service.DeleteSession(selected.ID)
	if err != nil {
		fmt.Printf("✗ Delete failed: %v\n\n", err)
		return
	}

	fmt.Println("✓ Session deleted successfully!")
	fmt.Println()
}

// handleUpdateSession runs the interactive update sub-flow.
func handleUpdateSession(reader *bufio.Reader, userID string, service *MenuService) {
	fmt.Println("\n--- Update Session ---")

	// Step 1: Fetch and display sessions
	sessions, err := service.GetUserSessions(userID)
	if err != nil {
		fmt.Printf("Error fetching sessions: %v\n\n", err)
		return
	}
	if len(sessions) == 0 {
		fmt.Println("No sessions found.\n")
		return
	}

	// Display sessions in a compact selectable list
	for i, s := range sessions {
		fmt.Printf("  %d. %s  [%s]\n", i+1, s.Name, s.Status)
	}
	fmt.Println()

	// Step 2: Select a session
	sessionIndex := promptForNumber(reader, "Select session (number): ", 1, len(sessions))
	if sessionIndex == -1 {
		return
	}
	selected := sessions[sessionIndex-1]

	// Step 3: Show update options
	for {
		fmt.Printf("\nEditing: %s\n", selected.Name)
		fmt.Println("  1. Rename session")
		fmt.Println("  2. Change status (ACTIVE / ARCHIVED)")
		fmt.Println("  3. Back to menu")
		fmt.Print("Select: ")

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			return
		}

		action := strings.TrimSpace(input)
		switch action {
		case "1":
			handleRename(reader, &selected, service)
		case "2":
			handleStatusChange(reader, &selected, service)
		case "3":
			fmt.Println()
			return
		default:
			fmt.Println("Invalid choice.")
		}
	}
}

// handleRename prompts for a new name and applies the rename.
func handleRename(reader *bufio.Reader, session *Session, service *MenuService) {
	fmt.Printf("\nCurrent name: %s\n", session.Name)
	fmt.Print("Enter new name (or press Enter to cancel): ")

	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		return
	}

	newName := strings.TrimSpace(input)
	if newName == "" {
		fmt.Println("Cancelled.")
		return
	}

	updated, err := service.RenameSession(session.ID, newName)
	if err != nil {
		fmt.Printf("✗ Update failed: %v\n", err)
		return
	}

	// Sync the local session state so further edits reflect the change
	*session = *updated
	fmt.Println("✓ Session renamed successfully!")
	fmt.Println(service.FormatSessionDetail(updated))
}

// handleStatusChange prompts the user to pick a new status and applies it.
func handleStatusChange(reader *bufio.Reader, session *Session, service *MenuService) {
	fmt.Printf("\nCurrent status: %s\n", session.Status)
	fmt.Println("Available statuses:")
	fmt.Println("  1. ACTIVE")
	fmt.Println("  2. ARCHIVED")
	fmt.Print("Select new status (or press Enter to cancel): ")

	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		return
	}

	picked := strings.TrimSpace(input)
	var newStatus string
	switch picked {
	case "1":
		newStatus = "ACTIVE"
	case "2":
		newStatus = "ARCHIVED"
	case "":
		fmt.Println("Cancelled.")
		return
	default:
		fmt.Println("Invalid selection.")
		return
	}

	if newStatus == session.Status {
		fmt.Printf("Session is already %s.\n", session.Status)
		return
	}

	updated, err := service.ChangeSessionStatus(session.ID, newStatus)
	if err != nil {
		fmt.Printf("✗ Update failed: %v\n", err)
		return
	}

	*session = *updated
	fmt.Println("✓ Status updated successfully!")
	fmt.Println(service.FormatSessionDetail(updated))
}

// promptForNumber reads a number from stdin within [min, max] range.
// Returns -1 if the user enters invalid input.
func promptForNumber(reader *bufio.Reader, prompt string, min, max int) int {
	fmt.Print(prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading input: %v\n\n", err)
		return -1
	}

	num, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || num < min || num > max {
		fmt.Printf("Invalid selection. Enter a number between %d and %d.\n\n", min, max)
		return -1
	}
	return num
}


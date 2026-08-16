package menu

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"multiplayer_ai_client/contextengine"
	"multiplayer_ai_client/session"
	"multiplayer_ai_client/ui"
)

// ShowMenu displays the CLI menu for the client application and delegates
// action processing to the provided MenuService.
func ShowMenu(userID string, service *MenuService) {
	reader := bufio.NewReader(os.Stdin)

	for {
		ui.Banner("Main Menu")
		ui.MenuItem(1, "📋", "See my sessions")
		ui.MenuItem(2, "✚", "Create session")
		ui.MenuItem(3, "→", "Join session")
		ui.MenuItem(4, "✎", "Update session")
		ui.MenuItem(5, "✗", "Delete session")
		ui.MenuItem(6, "⏻", "Exit")
		fmt.Println()
		ui.Prompt("Select option")

		input, err := reader.ReadString('\n')
		if err != nil {
			ui.Errorf("Error reading input: %v", err)
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
			ui.LeaveMsg()
			return
		default:
			ui.Warn("Invalid choice, please select again.")
		}
	}
}

// handleSeeSessions displays the user's sessions.
func handleSeeSessions(reader *bufio.Reader, userID string, service *MenuService) {
	ui.Banner("My Sessions")

	spin := ui.NewSpinner("Fetching sessions…")
	sessions, err := service.GetUserSessions(userID)
	if err != nil {
		spin.StopError(fmt.Sprintf("%v", err))
		return
	}
	spin.Stop()

	fmt.Print(service.FormatSessionList(sessions))
	fmt.Println()
}

// handleCreateSession prompts for a session name and creates a new session.
func handleCreateSession(reader *bufio.Reader, userID string, service *MenuService) {
	ui.Banner("Create Session")
	ui.Prompt("Session name (Enter to cancel)")

	input, err := reader.ReadString('\n')
	if err != nil {
		ui.Errorf("Error reading input: %v", err)
		return
	}

	name := strings.TrimSpace(input)
	if name == "" {
		ui.Info("Cancelled.")
		return
	}

	gitInfo, isGit := GetGitInfo()
	var created *Session

	if isGit {
		ui.GitInfoBox(gitInfo.RepoURL, gitInfo.Branch, gitInfo.CommitSHA)
		spin := ui.NewSpinner("Creating git-synchronized session…")
		created, err = service.CreateSession(userID, name, gitInfo.RepoURL, gitInfo.Branch, gitInfo.CommitSHA)
		if err != nil {
			spin.StopError(fmt.Sprintf("Creation failed: %v", err))
			return
		}
		spin.StopSuccess("Git-synchronized session created successfully!")
	} else {
		ui.Info("No Git repository detected. Generating initial workspace snapshot…")

		zipSpin := ui.NewSpinner("Compressing workspace…")
		zipBytes, zErr := ZipDirectory(".")
		if zErr != nil {
			zipSpin.StopError(fmt.Sprintf("Failed to compress workspace: %v", zErr))
			return
		}
		zipSpin.Stop()

		zipSizeMB := float64(len(zipBytes)) / (1024 * 1024)
		ui.Detailf("Archive size: %.2f MB", zipSizeMB)

		if len(zipBytes) > 5*1024*1024 {
			ui.Errorf("Archive size (%.2f MB) exceeds the 5 MB limit.", zipSizeMB)
			return
		}

		crSpin := ui.NewSpinner("Creating session…")
		created, err = service.CreateSession(userID, name, "", "", "")
		if err != nil {
			crSpin.StopError(fmt.Sprintf("Creation failed: %v", err))
			return
		}
		crSpin.Stop()

		upSpin := ui.NewSpinner("Uploading initial workspace snapshot…")
		_, upErr := service.UploadSnapshot(created.ID, zipBytes)
		if upErr != nil {
			upSpin.StopError(fmt.Sprintf("Snapshot upload failed: %v", upErr))
			return
		}
		upSpin.StopSuccess("Session created and initial snapshot uploaded successfully!")
	}

	fmt.Println()
	fmt.Print(service.FormatSessionDetail(created))
	fmt.Println()
}

// handleJoinSession runs the interactive join session flow.
func handleJoinSession(reader *bufio.Reader, userID string, service *MenuService) {
	ui.Banner("Join Session")
	ui.SubMenuItem(1, "Join an active session you own")
	ui.SubMenuItem(2, "Join any other session by Session ID")
	ui.SubMenuItem(3, "Back to menu")
	fmt.Println()
	ui.Prompt("Select")

	input, err := reader.ReadString('\n')
	if err != nil {
		ui.Errorf("Error reading input: %v", err)
		return
	}

	choice := strings.TrimSpace(input)
	switch choice {
	case "1":
		// List owned sessions
		spin := ui.NewSpinner("Fetching your sessions…")
		sessions, err := service.GetUserSessions(userID)
		if err != nil {
			spin.StopError(fmt.Sprintf("Failed to load sessions: %v", err))
			return
		}
		spin.Stop()

		// Filter out terminated sessions
		var activeSessions []Session
		for _, s := range sessions {
			if s.Status == "ACTIVE" {
				activeSessions = append(activeSessions, s)
			}
		}

		if len(activeSessions) == 0 {
			ui.Warn("No active sessions found to join.")
			return
		}

		fmt.Println()
		for i, s := range activeSessions {
			ui.SubMenuItem(i+1, fmt.Sprintf("%s  %s%s%s", s.Name, ui.BrightBlack+ui.Dim, s.ID, ui.Reset))
		}
		fmt.Println()

		sessionIndex := promptForNumber(reader, "Select session to join (number): ", 1, len(activeSessions))
		if sessionIndex == -1 {
			return
		}
		selected := activeSessions[sessionIndex-1]

		joinSpin := ui.NewSpinner(fmt.Sprintf("Joining session '%s'…", selected.Name))
		joined, err := service.JoinSession(userID, selected.ID)
		if err != nil {
			joinSpin.StopError(fmt.Sprintf("Join failed: %v", err))
			return
		}
		joinSpin.StopSuccess("Successfully joined session!")

		if !syncOwnerSessionState(joined, userID, service) {
			return
		}
		if !syncMemberSessionState(joined, userID, service) {
			return
		}
		fmt.Print(service.FormatSessionDetail(joined))
		fmt.Println()

		// Dynamically open SQLite DB folder named: session + workingDirectoryBasedHash
		dbSpin := ui.NewSpinner("Initializing local context database…")
		db, err := contextengine.InitSessionDB(selected.ID, ".")
		if err != nil {
			dbSpin.StopError(fmt.Sprintf("Failed to initialize session SQLite DB: %v", err))
			return
		}
		dbSpin.StopSuccess("Context database initialized.")
		engine := contextengine.NewSqliteContextEngine(db)
		engine.InitLogger(selected.ID, ".")
		defer engine.Close()
		defer db.Close()

		// Start the live session interaction module (REST + WS)
		session.RunSessionFlow(userID, selected.ID, service.GetBaseURL(), engine)

	case "2":
		ui.Prompt("Session ID (UUID)")
		inputID, err := reader.ReadString('\n')
		if err != nil {
			ui.Errorf("Error reading input: %v", err)
			return
		}
		sessionID := strings.TrimSpace(inputID)
		if sessionID == "" {
			ui.Info("Cancelled.")
			return
		}

		joinSpin := ui.NewSpinner("Joining session…")
		joined, err := service.JoinSession(userID, sessionID)
		if err != nil {
			joinSpin.StopError(fmt.Sprintf("Join failed: %v", err))
			return
		}
		joinSpin.StopSuccess("Successfully joined session!")

		if !syncOwnerSessionState(joined, userID, service) {
			return
		}
		if !syncMemberSessionState(joined, userID, service) {
			return
		}
		fmt.Print(service.FormatSessionDetail(joined))
		fmt.Println()

		// Dynamically open SQLite DB folder named: session + workingDirectoryBasedHash
		dbSpin := ui.NewSpinner("Initializing local context database…")
		db, err := contextengine.InitSessionDB(sessionID, ".")
		if err != nil {
			dbSpin.StopError(fmt.Sprintf("Failed to initialize session SQLite DB: %v", err))
			return
		}
		dbSpin.StopSuccess("Context database initialized.")
		engine := contextengine.NewSqliteContextEngine(db)
		engine.InitLogger(sessionID, ".")
		defer engine.Close()
		defer db.Close()

		// Start the live session interaction module (REST + WS)
		session.RunSessionFlow(userID, sessionID, service.GetBaseURL(), engine)

	case "3":
		fmt.Println()
		return
	default:
		ui.Warn("Invalid selection.")
	}
}

// syncOwnerSessionState checks if the current user is the owner, and if so,
// synchronizes their local workspace (Git info or Zip snapshot) to the backend.
func syncOwnerSessionState(joined *Session, userID string, service *MenuService) bool {
	if joined.OwnerID != userID {
		return true // Other users do not need to upload state
	}

	ui.Info("Welcome, Session Owner! Synchronizing latest workspace state to backend…")
	gitInfo, isGit := GetGitInfo()

	if isGit {
		spin := ui.NewSpinner(fmt.Sprintf("Updating Git reference (HEAD @ %s)…", gitInfo.CommitSHA[:8]))
		updated, err := service.UpdateSessionGitInfo(joined.ID, gitInfo.RepoURL, gitInfo.Branch, gitInfo.CommitSHA)
		if err != nil {
			spin.StopError(fmt.Sprintf("Git synchronization failed: %v", err))
		} else {
			*joined = *updated
			spin.StopSuccess("Git metadata successfully updated on backend!")
		}
	} else {
		spin := ui.NewSpinner("Compressing local workspace code…")
		zipBytes, err := ZipDirectory(".")
		if err != nil {
			spin.StopError(fmt.Sprintf("Failed to compress workspace: %v", err))
			return false
		}
		spin.Stop()

		zipSizeMB := float64(len(zipBytes)) / (1024 * 1024)
		if len(zipBytes) > 5*1024*1024 {
			ui.Errorf("Cannot join session: Workspace snapshot size (%.2f MB) exceeds 5 MB limit.", zipSizeMB)
			return false
		}

		upSpin := ui.NewSpinner(fmt.Sprintf("Uploading latest workspace snapshot (%.2f MB)…", zipSizeMB))
		_, upErr := service.UploadSnapshot(joined.ID, zipBytes)
		if upErr != nil {
			upSpin.StopError(fmt.Sprintf("Snapshot synchronization failed: %v", upErr))
		} else {
			upSpin.StopSuccess("Workspace snapshot synchronized successfully!")
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
		ui.Info("Git session detected. Synchronizing codebase…")
		// Check if we are inside a local Git repo
		_, isGit := GetGitInfo()
		if !isGit {
			ui.Error("This is a Git-tracked session, but your local folder is not a Git repository.")
			ui.Detail("Please run 'git init' or clone the project repository first.")
			return false
		}

		spin := ui.NewSpinner(fmt.Sprintf("Switching local workspace branch to: %s…", joined.GitBranch))
		cmd := exec.Command("git", "checkout", joined.GitBranch)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			spin.StopError(fmt.Sprintf("Failed to switch branch: %v", err))
			return false
		}
		spin.StopSuccess("Local workspace successfully switched to branch!")
	} else {
		ui.Info("Non-Git session detected. Downloading latest workspace snapshot…")
		// If version is 1, maybe no snapshot is uploaded yet. Let's check current version.
		if joined.CurrentVersion < 2 {
			ui.Warn("No workspace snapshots are available for this session yet.")
			return true
		}

		dlSpin := ui.NewSpinner("Downloading workspace snapshot from server…")
		zipBytes, err := service.DownloadSnapshot(joined.ID, joined.CurrentVersion)
		if err != nil {
			dlSpin.StopError(fmt.Sprintf("Failed to download snapshot from server: %v", err))
			return false
		}
		dlSpin.Stop()

		clSpin := ui.NewSpinner("Cleaning local workspace (excluding client binary and config)…")
		if err := CleanDirectory("."); err != nil {
			clSpin.StopError(fmt.Sprintf("Failed to clean directory: %v", err))
			return false
		}
		clSpin.Stop()

		exSpin := ui.NewSpinner("Extracting workspace snapshot files…")
		if err := UnzipBytes(zipBytes, "."); err != nil {
			exSpin.StopError(fmt.Sprintf("Failed to extract snapshot files: %v", err))
			return false
		}
		exSpin.StopSuccess("Workspace successfully synchronized to latest server snapshot!")
	}
	return true
}

// handleDeleteSession runs the interactive delete sub-flow.
func handleDeleteSession(reader *bufio.Reader, userID string, service *MenuService) {
	ui.Banner("Delete Session")

	// Step 1: Fetch and display sessions
	spin := ui.NewSpinner("Fetching sessions…")
	sessions, err := service.GetUserSessions(userID)
	if err != nil {
		spin.StopError(fmt.Sprintf("Error fetching sessions: %v", err))
		return
	}
	spin.Stop()

	if len(sessions) == 0 {
		ui.Warn("No sessions found.")
		return
	}

	fmt.Println()
	for i, s := range sessions {
		ui.SubMenuItem(i+1, fmt.Sprintf("%-28s  %s", s.Name, ui.StatusBadge(s.Status)))
	}
	fmt.Println()

	// Step 2: Select a session
	sessionIndex := promptForNumber(reader, "Select session to delete (number): ", 1, len(sessions))
	if sessionIndex == -1 {
		return
	}
	selected := sessions[sessionIndex-1]

	// Step 3: Confirm deletion
	fmt.Println()
	ui.Warnf("You are about to delete: %s%s%s  %s(%s)%s", ui.Bold+ui.BrightWhite, selected.Name, ui.Reset, ui.Dim, selected.ID, ui.Reset)
	ui.Prompt("This action is permanent. Continue? (y/N)")

	input, err := reader.ReadString('\n')
	if err != nil {
		ui.Errorf("Error reading input: %v", err)
		return
	}

	confirm := strings.TrimSpace(strings.ToLower(input))
	if confirm != "y" && confirm != "yes" {
		ui.Info("Cancelled.")
		return
	}

	// Step 4: Delete
	delSpin := ui.NewSpinner(fmt.Sprintf("Deleting session '%s'…", selected.Name))
	err = service.DeleteSession(selected.ID)
	if err != nil {
		delSpin.StopError(fmt.Sprintf("Delete failed: %v", err))
		return
	}
	delSpin.StopSuccess("Session deleted successfully!")
	fmt.Println()
}

// handleUpdateSession runs the interactive update sub-flow.
func handleUpdateSession(reader *bufio.Reader, userID string, service *MenuService) {
	ui.Banner("Update Session")

	// Step 1: Fetch and display sessions
	spin := ui.NewSpinner("Fetching sessions…")
	sessions, err := service.GetUserSessions(userID)
	if err != nil {
		spin.StopError(fmt.Sprintf("Error fetching sessions: %v", err))
		return
	}
	spin.Stop()

	if len(sessions) == 0 {
		ui.Warn("No sessions found.")
		return
	}

	// Display sessions in a compact selectable list
	fmt.Println()
	for i, s := range sessions {
		ui.SubMenuItem(i+1, fmt.Sprintf("%-28s  %s", s.Name, ui.StatusBadge(s.Status)))
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
		ui.Banner(fmt.Sprintf("Editing: %s", selected.Name))
		ui.SubMenuItem(1, "Rename session")
		ui.SubMenuItem(2, "Change status  (ACTIVE / ARCHIVED)")
		ui.SubMenuItem(3, "Back to menu")
		fmt.Println()
		ui.Prompt("Select")

		input, err := reader.ReadString('\n')
		if err != nil {
			ui.Errorf("Error reading input: %v", err)
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
			ui.Warn("Invalid choice.")
		}
	}
}

// handleRename prompts for a new name and applies the rename.
func handleRename(reader *bufio.Reader, sess *Session, service *MenuService) {
	fmt.Println()
	ui.Detailf("Current name: %s", sess.Name)
	ui.Prompt("New name (Enter to cancel)")

	input, err := reader.ReadString('\n')
	if err != nil {
		ui.Errorf("Error reading input: %v", err)
		return
	}

	newName := strings.TrimSpace(input)
	if newName == "" {
		ui.Info("Cancelled.")
		return
	}

	spin := ui.NewSpinner("Renaming session…")
	updated, err := service.RenameSession(sess.ID, newName)
	if err != nil {
		spin.StopError(fmt.Sprintf("Update failed: %v", err))
		return
	}

	// Sync the local session state so further edits reflect the change
	*sess = *updated
	spin.StopSuccess("Session renamed successfully!")
	fmt.Print(service.FormatSessionDetail(updated))
	fmt.Println()
}

// handleStatusChange prompts the user to pick a new status and applies it.
func handleStatusChange(reader *bufio.Reader, sess *Session, service *MenuService) {
	fmt.Println()
	ui.Detailf("Current status: %s", ui.StatusBadge(sess.Status))
	ui.SubMenuItem(1, "ACTIVE")
	ui.SubMenuItem(2, "ARCHIVED")
	fmt.Println()
	ui.Prompt("Select new status (Enter to cancel)")

	input, err := reader.ReadString('\n')
	if err != nil {
		ui.Errorf("Error reading input: %v", err)
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
		ui.Info("Cancelled.")
		return
	default:
		ui.Warn("Invalid selection.")
		return
	}

	if newStatus == sess.Status {
		ui.Infof("Session is already %s.", sess.Status)
		return
	}

	spin := ui.NewSpinner(fmt.Sprintf("Changing status to %s…", newStatus))
	updated, err := service.ChangeSessionStatus(sess.ID, newStatus)
	if err != nil {
		spin.StopError(fmt.Sprintf("Update failed: %v", err))
		return
	}

	*sess = *updated
	spin.StopSuccess("Status updated successfully!")
	fmt.Print(service.FormatSessionDetail(updated))
	fmt.Println()
}

// promptForNumber reads a number from stdin within [min, max] range.
// Returns -1 if the user enters invalid input.
func promptForNumber(reader *bufio.Reader, prompt string, min, max int) int {
	ui.Prompt(prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		ui.Errorf("Error reading input: %v", err)
		return -1
	}

	num, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || num < min || num > max {
		ui.Warnf("Invalid selection. Enter a number between %d and %d.", min, max)
		return -1
	}
	return num
}

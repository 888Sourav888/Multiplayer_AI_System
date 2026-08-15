package menu

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ShowMenu displays the CLI menu for the client application and delegates
// action processing to the provided MenuService.
func ShowMenu(userID string, service *MenuService) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("=== Menu ===")
		fmt.Println("1. See my sessions")
		fmt.Println("2. Create session")
		fmt.Println("3. Update session")
		fmt.Println("4. Delete session")
		fmt.Println("5. Exit")
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
			handleUpdateSession(reader, userID, service)
		case "4":
			handleDeleteSession(reader, userID, service)
		case "5":
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

	created, err := service.CreateSession(userID, name)
	if err != nil {
		fmt.Printf("✗ Creation failed: %v\n\n", err)
		return
	}

	fmt.Println("✓ Session created successfully!")
	fmt.Println(service.FormatSessionDetail(created))
	fmt.Println()
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
	deleted, err := service.DeleteSession(selected.ID)
	if err != nil {
		fmt.Printf("✗ Delete failed: %v\n\n", err)
		return
	}

	fmt.Println("✓ Session deleted successfully!")
	fmt.Println(service.FormatSessionDetail(deleted))
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


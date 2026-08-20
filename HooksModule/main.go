package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Step struct {
	EphemeralMessage string `json:"ephemeralMessage,omitempty"`
}

type Output struct {
	InjectSteps []Step `json:"injectSteps"`
}

type Message struct {
	SenderID  string
	Modifier  string
	Content   string
	StepIndex sql.NullInt64
	CreatedAt string
}

func main() {
	// We must write valid JSON to stdout under all circumstances.
	// Diagnostical errors are sent to stderr.
	writeEmptyResponse := func(reason string, err error) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "[fetch-latest-hook] Error: %s: %v\n", reason, err)
		} else if reason != "" {
			fmt.Fprintf(os.Stderr, "[fetch-latest-hook] Info: %s\n", reason)
		}
		fmt.Println(`{"injectSteps": []}`)
	}

	// Read stdin to prevent blocking the parent process pipe
	_, _ = io.ReadAll(os.Stdin)

	if len(os.Args) < 3 {
		writeEmptyResponse("Missing arguments: usage is <session_id> <folder_hash>", nil)
		return
	}

	sessionID := os.Args[1]
	folderHash := os.Args[2]

	home, err := os.UserHomeDir()
	if err != nil {
		writeEmptyResponse("Failed to determine user home directory", err)
		return
	}

	dirName := fmt.Sprintf("%s_%s", sessionID, folderHash)
	dbDir := filepath.Join(home, ".mpai", "shared context", dirName)
	dbPath := filepath.Join(dbDir, "multiplayer_ai.db")

	// SQLite lock contention: Set WAL mode and busy timeout via query params
	dsn := dbPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		writeEmptyResponse("Failed to open SQLite database", err)
		return
	}
	defer db.Close()

	// Current user filtering: Fetch active_user_id from sessions table
	var activeUserID string
	err = db.QueryRow("SELECT active_user_id FROM sessions WHERE id = ? LIMIT 1", sessionID).Scan(&activeUserID)
	if err != nil && err != sql.ErrNoRows {
		writeEmptyResponse("Failed to query active_user_id from sessions", err)
		return
	}

	// Watermark tracking: Read watermark timestamp from file
	lastSeenPath := filepath.Join(dbDir, ".last_seen_time")
	lastSeenTime := "1970-01-01T00:00:00Z"
	if data, err := os.ReadFile(lastSeenPath); err == nil {
		trimmed := string(data)
		if trimmed != "" {
			lastSeenTime = trimmed
		}
	}

	// Fetch latest 5 messages that are:
	// - part of the current session
	// - not sent by the active user (activeUserID)
	// - newer than the last seen watermark
	query := `SELECT sender_id, modifier, content, step_index, created_at 
		FROM ai_messages 
		WHERE session_id = ? AND sender_id != ? AND created_at > ? 
		ORDER BY created_at DESC 
		LIMIT 5`

	rows, err := db.Query(query, sessionID, activeUserID, lastSeenTime)
	if err != nil {
		writeEmptyResponse("Failed to query ai_messages", err)
		return
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.SenderID, &m.Modifier, &m.Content, &m.StepIndex, &m.CreatedAt); err != nil {
			continue
		}
		messages = append(messages, m)
	}

	if err = rows.Err(); err != nil {
		writeEmptyResponse("Row iteration error", err)
		return
	}

	if len(messages) == 0 {
		writeEmptyResponse("", nil)
		return
	}

	// Ordering: reverse the messages so they are printed in chronological order (oldest -> newest)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	// Construct message text
	var messagesText string
	for _, m := range messages {
		stepStr := "N/A"
		if m.StepIndex.Valid {
			stepStr = fmt.Sprintf("%d", m.StepIndex.Int64)
		}
		messagesText += fmt.Sprintf("[%s] %s (%s):\n%s\n---\n", stepStr, m.SenderID, m.Modifier, m.Content)
	}

	// Update the watermark with the timestamp of the newest message (last element after reversal)
	newestTime := messages[len(messages)-1].CreatedAt
	_ = os.WriteFile(lastSeenPath, []byte(newestTime), 0644)

	header := fmt.Sprintf("System Notice: Here are the latest %d messages from other participants in this live multiplayer session. Review them to keep your local context aligned:\n\n", len(messages))
	fullText := header + messagesText

	out := Output{
		InjectSteps: []Step{
			{
				EphemeralMessage: fullText,
			},
		},
	}

	jsonData, err := json.Marshal(out)
	if err != nil {
		writeEmptyResponse("Failed to marshal JSON output", err)
		return
	}

	fmt.Println(string(jsonData))
}

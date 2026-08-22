package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
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

func extractSessionInfo() (string, string, error) {
	// 1. Try to read from hooks.json in the current directory
	if data, err := os.ReadFile("hooks.json"); err == nil {
		var config struct {
			PostInvocationHandler struct {
				PostInvocation []struct {
					Command string `json:"command"`
				} `json:"PostInvocation"`
			} `json:"post-invocation-handler"`
		}
		if err := json.Unmarshal(data, &config); err == nil {
			for _, h := range config.PostInvocationHandler.PostInvocation {
				parts := strings.Fields(h.Command)
				for i := 0; i < len(parts)-2; i++ {
					if parts[i] == "post" {
						return parts[i+1], parts[i+2], nil
					}
				}
			}
		}
	}

	// 2. Try to read from .cursorrules or cursorrules in parent directory
	rulePaths := []string{
		".cursorrules",
		"../.cursorrules",
		".clinerules",
		"../.clinerules",
		".cursor/rules/multiplayer.mdc",
		"../.cursor/rules/multiplayer.mdc",
	}
	for _, p := range rulePaths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var sessionID, folderHash string
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "# Session ID:") {
				sessionID = strings.TrimSpace(strings.TrimPrefix(line, "# Session ID:"))
			} else if strings.HasPrefix(line, "# Folder Hash:") {
				folderHash = strings.TrimSpace(strings.TrimPrefix(line, "# Folder Hash:"))
			}
		}
		if sessionID != "" && folderHash != "" {
			return sessionID, folderHash, nil
		}
	}

	return "", "", fmt.Errorf("could not find session ID and folder hash in hooks.json or rules files")
}

func main() {
	home, homeErr := os.UserHomeDir()

	// Write hook start debug log
	if homeErr == nil {
		debugLogPath := filepath.Join(home, "hook_debug.log")
		debugFile, err := os.OpenFile(debugLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			_, _ = fmt.Fprintf(debugFile, "[%s] Hook started. Args: %q\n", time.Now().Format("2006-01-02 15:04:05"), os.Args)
			debugFile.Close()
		}
	}

	// We must write valid JSON to stdout under all circumstances.
	// Diagnostical errors are sent to stderr.
	writeEmptyResponse := func(reason string, err error) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "[fetch-latest-hook] Error: %s: %v\n", reason, err)
		} else if reason != "" {
			fmt.Fprintf(os.Stderr, "[fetch-latest-hook] Info: %s\n", reason)
		}

		if homeErr == nil {
			debugLogPath := filepath.Join(home, "hook_debug.log")
			debugFile, logErr := os.OpenFile(debugLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if logErr == nil {
				if err != nil {
					_, _ = fmt.Fprintf(debugFile, "  Empty response written. Reason: %s, Error: %v\n", reason, err)
				} else {
					_, _ = fmt.Fprintf(debugFile, "  Empty response written. Reason: %s\n", reason)
				}
				debugFile.Close()
			}
		}
		fmt.Println(`{"injectSteps": []}`)
	}

	var mode string
	var sessionID string
	var folderHash string

	if len(os.Args) >= 4 {
		mode = os.Args[1]
		sessionID = os.Args[2]
		folderHash = os.Args[3]
	} else if len(os.Args) == 3 {
		mode = "pre"
		sessionID = os.Args[1]
		folderHash = os.Args[2]
	} else {
		var err error
		sessionID, folderHash, err = extractSessionInfo()
		if err != nil {
			writeEmptyResponse(fmt.Sprintf("Missing arguments and fallback failed: %v", err), nil)
			return
		}
		mode = "pre"
	}

	if mode == "post" {
		err := handlePostInvocation(sessionID, folderHash)
		if err != nil {
			writeEmptyResponse("PostInvocation error", err)
		} else {
			if homeErr == nil {
				debugLogPath := filepath.Join(home, "hook_debug.log")
				debugFile, logErr := os.OpenFile(debugLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
				if logErr == nil {
					_, _ = fmt.Fprintf(debugFile, "  PostInvocation completed successfully\n")
					debugFile.Close()
				}
			}
			fmt.Println(`{"injectSteps": []}`)
		}
		return
	}

	// Read stdin safely without waiting for EOF by using json.NewDecoder
	var temp interface{}
	_ = json.NewDecoder(os.Stdin).Decode(&temp)

	home, err := os.UserHomeDir()
	if err != nil {
		writeEmptyResponse("Failed to determine user home directory", err)
		return
	}

	dirName := fmt.Sprintf("%s_%s", sessionID, folderHash)
	dbDir := filepath.Join(home, ".mpai", "shared context", dirName)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		writeEmptyResponse("Failed to create dbDir "+dbDir, err)
		return
	}
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

	// Log pre-invocation details
	logPath := filepath.Join(dbDir, "pre_invocation.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		logMsg := fmt.Sprintf("[%s] PreInvocation triggered\n", time.Now().Format("2006-01-02 15:04:05"))
		logMsg += fmt.Sprintf("  Active User ID: %s\n", activeUserID)
		logMsg += fmt.Sprintf("  Last Seen Watermark: %s\n", lastSeenTime)
		logMsg += fmt.Sprintf("  Retrieved %d new messages from other participants\n", len(messages))
		if len(messages) > 0 {
			logMsg += "  Messages (newest to oldest):\n"
			for i, m := range messages {
				stepStr := "N/A"
				if m.StepIndex.Valid {
					stepStr = fmt.Sprintf("%d", m.StepIndex.Int64)
				}
				logMsg += fmt.Sprintf("    [%d] Step: %s | Sender: %s (%s) | CreatedAt: %s\n", i+1, stepStr, m.SenderID, m.Modifier, m.CreatedAt)
				logMsg += fmt.Sprintf("        Content: %s\n", m.Content)
			}
		}
		logMsg += "--------------------------------------------------\n"
		_, _ = logFile.WriteString(logMsg)
		logFile.Close()
	} else {
		if homeErr == nil {
			debugLogPath := filepath.Join(home, "hook_debug.log")
			debugFile, logErr := os.OpenFile(debugLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if logErr == nil {
				_, _ = fmt.Fprintf(debugFile, "  Failed to open pre_invocation.log: %v\n", err)
				debugFile.Close()
			}
		}
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

func handlePostInvocation(sessionID, folderHash string) error {
	// 1. Read stdin safely using json.NewDecoder to avoid blocking on EOF
	var input struct {
		TranscriptPath string `json:"transcriptPath"`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		return fmt.Errorf("failed to parse stdin JSON: %w", err)
	}

	if input.TranscriptPath == "" {
		return fmt.Errorf("transcriptPath is empty in stdin payload")
	}

	// 2. Read transcript JSONL
	file, err := os.Open(input.TranscriptPath)
	if err != nil {
		return fmt.Errorf("failed to open transcript file %s: %w", input.TranscriptPath, err)
	}
	defer file.Close()

	type TranscriptLine struct {
		StepIndex int    `json:"step_index"`
		Source    string `json:"source"`
		Type      string `json:"type"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
		Content   string `json:"content"`
	}

	var latestAIResponse *TranscriptLine
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var line TranscriptLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if line.Source == "MODEL" && line.Type == "PLANNER_RESPONSE" && line.Content != "" {
			latestAIResponse = &line
		}
	}

	if latestAIResponse == nil {
		// No AI response found in transcript, nothing to do
		return nil
	}

	// 3. Set up paths
	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return fmt.Errorf("failed to get user home: %w", homeErr)
	}

	dirName := fmt.Sprintf("%s_%s", sessionID, folderHash)
	dbDir := filepath.Join(home, ".mpai", "shared context", dirName)
	_ = os.MkdirAll(dbDir, 0755)

	// 4. Log the response to post_invocation.log
	logPath := filepath.Join(dbDir, "post_invocation.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		logMsg := fmt.Sprintf("[%s] Step %d: %s\n", time.Now().Format("2006-01-02 15:04:05"), latestAIResponse.StepIndex, latestAIResponse.Content)
		_, _ = logFile.WriteString(logMsg)
		logFile.Close()
	} else {
		debugLogPath := filepath.Join(home, "hook_debug.log")
		debugFile, logErr := os.OpenFile(debugLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if logErr == nil {
			_, _ = fmt.Fprintf(debugFile, "  Failed to open post_invocation.log: %v\n", err)
			debugFile.Close()
		}
	}

	// 5. Open SQLite DB
	dbPath := filepath.Join(dbDir, "multiplayer_ai.db")
	dsn := dbPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to open SQLite database: %w", err)
	}
	defer db.Close()

	// Get active user ID
	var activeUserID string
	err = db.QueryRow("SELECT active_user_id FROM sessions WHERE id = ? LIMIT 1", sessionID).Scan(&activeUserID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to query active_user_id: %w", err)
	}
	if activeUserID == "" {
		activeUserID = "unknown_user"
	}

	// Check if message with step index already exists in local_ai_messages
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM local_ai_messages WHERE session_id = ? AND step_index = ?", sessionID, latestAIResponse.StepIndex).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to query local_ai_messages count: %w", err)
	}

	if count == 0 {
		// Generate UUID for the local DB
		msgID := uuid.New().String()

		createdAt, err := time.Parse(time.RFC3339, latestAIResponse.CreatedAt)
		if err != nil {
			createdAt = time.Now()
		}

		// Insert into local_ai_messages table
		query := `INSERT OR IGNORE INTO local_ai_messages (id, session_id, sender_id, modifier, content, step_index, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`
		_, err = db.Exec(query, msgID, sessionID, activeUserID, "AI (via Antigravity)", latestAIResponse.Content, latestAIResponse.StepIndex, createdAt.Format(time.RFC3339))
		if err != nil {
			return fmt.Errorf("failed to insert local AI message: %w", err)
		}

		// Log success to hook_debug.log
		if homeErr == nil {
			debugLogPath := filepath.Join(home, "hook_debug.log")
			debugFile, logErr := os.OpenFile(debugLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if logErr == nil {
				_, _ = fmt.Fprintf(debugFile, "  Saved AI message locally in local_ai_messages (step %d)\n", latestAIResponse.StepIndex)
				debugFile.Close()
			}
		}
	}

	return nil
}

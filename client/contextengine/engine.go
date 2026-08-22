package contextengine

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"multiplayer_ai_client/session"
)

type SqliteContextEngine struct {
	db      *sql.DB
	logFile *os.File
}

func NewSqliteContextEngine(db *sql.DB) *SqliteContextEngine {
	return &SqliteContextEngine{db: db}
}

// InitLogger initializes logging to client.log inside the session DB directory
func (e *SqliteContextEngine) InitLogger(sessionID, wd string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	hash := GetWDHash(wd)
	dirName := fmt.Sprintf("%s_%s", sessionID, hash)
	dbDir := filepath.Join(home, ".mpai", "shared context", dirName)
	_ = os.MkdirAll(dbDir, 0755)

	logPath := filepath.Join(dbDir, "client.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		e.logFile = file
		// Also redirect the standard go logger to this file
		log.SetOutput(file)
	}
}

func (e *SqliteContextEngine) Close() {
	if e.logFile != nil {
		_ = e.logFile.Close()
	}
}

func (e *SqliteContextEngine) LogInfo(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if e.logFile != nil {
		_, _ = e.logFile.WriteString(time.Now().Format("2006-01-02 15:04:05") + " [INFO] " + msg + "\n")
	} else {
		log.Printf("[INFO] %s\n", msg)
	}
}

func (e *SqliteContextEngine) LogError(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if e.logFile != nil {
		_, _ = e.logFile.WriteString(time.Now().Format("2006-01-02 15:04:05") + " [ERROR] " + msg + "\n")
	} else {
		log.Printf("[ERROR] %s\n", msg)
	}
}

func (e *SqliteContextEngine) SetActiveSession(s *session.Session, userID, localPath string) error {
	return SetActiveSession(e.db, s, userID, localPath)
}

func (e *SqliteContextEngine) SaveAIMessage(id, sessionID, senderID, modifier, content string, stepIndex int, createdAt time.Time) error {
	return SaveAIMessage(e.db, id, sessionID, senderID, modifier, content, stepIndex, createdAt)
}

func (e *SqliteContextEngine) GetSessionMessages(sessionID string, limit int) ([]session.AIMessage, error) {
	return GetSessionMessages(e.db, sessionID, limit)
}

func (e *SqliteContextEngine) StartPoller(ctx context.Context, sessionID, userID, projectPath string, broadcastFn func(modifier string, content string, stepIndex int)) context.CancelFunc {
	pollerCtx, cancel := context.WithCancel(ctx)
	
	// Start polling messages created after the current time
	lastSeenTime := time.Now().UTC().Format(time.RFC3339)
	ticker := time.NewTicker(1000 * time.Millisecond)

	e.LogInfo("Starting local SQLite poller for AI messages...")

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-pollerCtx.Done():
				e.LogInfo("Stopped local SQLite poller.")
				return
			case <-ticker.C:
				// Query local_ai_messages table for new messages
				query := `SELECT id, sender_id, modifier, content, step_index, created_at 
					FROM local_ai_messages 
					WHERE session_id = ? AND created_at > ? 
					ORDER BY created_at ASC`
				rows, err := e.db.Query(query, sessionID, lastSeenTime)
				if err != nil {
					e.LogError("Failed to query local_ai_messages: %v", err)
					continue
				}
				
				var lastTime time.Time
				for rows.Next() {
					var id, senderID, modifier, content, createdAtStr string
					var stepIdx int
					if err := rows.Scan(&id, &senderID, &modifier, &content, &stepIdx, &createdAtStr); err == nil {
						// Parse message time
						t, err := time.Parse(time.RFC3339, createdAtStr)
						if err != nil {
							t = time.Now().UTC()
						}
						
						e.LogInfo("SQLite Poller: Found new AI message for step %d, saving and broadcasting", stepIdx)

						// Save locally in the shared context DB's ai_messages table
						_ = e.SaveAIMessage(id, sessionID, senderID, modifier, content, stepIdx, t)
						
						// Trigger the WebSocket broadcast
						broadcastFn(modifier, content, stepIdx)
						
						if t.After(lastTime) {
							lastTime = t
						}
					}
				}
				rows.Close()
				
				if !lastTime.IsZero() {
					// Add a tiny buffer (1 millisecond) to avoid matching the same message again
					lastSeenTime = lastTime.Add(1 * time.Millisecond).Format(time.RFC3339)
				}
			}
		}
	}()

	return cancel
}

func (e *SqliteContextEngine) SaveFileChange(id, sessionID, senderID, filePath, operation, modifier string, isAiEdit bool, changeContent string, createdAt time.Time) error {
	return SaveFileChange(e.db, id, sessionID, senderID, filePath, operation, modifier, isAiEdit, changeContent, createdAt)
}

func (e *SqliteContextEngine) GetFileChanges(sessionID string, limit int) ([]session.FileChange, error) {
	return GetFileChanges(e.db, sessionID, limit)
}

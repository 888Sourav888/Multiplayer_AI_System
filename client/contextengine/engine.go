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
	poller := NewAITranscriptPoller(sessionID, userID, projectPath, e.db, broadcastFn, e)
	go poller.Start(pollerCtx)
	return cancel
}

func (e *SqliteContextEngine) SaveFileChange(id, sessionID, senderID, filePath, operation, modifier string, isAiEdit bool, changeContent string, createdAt time.Time) error {
	return SaveFileChange(e.db, id, sessionID, senderID, filePath, operation, modifier, isAiEdit, changeContent, createdAt)
}

func (e *SqliteContextEngine) GetFileChanges(sessionID string, limit int) ([]session.FileChange, error) {
	return GetFileChanges(e.db, sessionID, limit)
}

package contextengine

import (
	"crypto/md5"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"multiplayer_ai_client/session"

	_ "modernc.org/sqlite"
)

// GetWDHash hashes the working directory path to a 16-character hex string
func GetWDHash(wd string) string {
	h := md5.New()
	abs, err := filepath.Abs(wd)
	if err != nil {
		abs = wd
	}
	_, _ = io.WriteString(h, strings.ToLower(filepath.ToSlash(abs)))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// InitSessionDB initializes the SQLite database at %USERPROFILE%/.mpai/shared context/[sessionID]_[wdHash]/multiplayer_ai.db
func InitSessionDB(sessionID, wd string) (*sql.DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home dir: %w", err)
	}

	hash := GetWDHash(wd)
	dirName := fmt.Sprintf("%s_%s", sessionID, hash)

	dbDir := filepath.Join(home, ".mpai", "shared context", dirName)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dbDir, err)
	}

	dbPath := filepath.Join(dbDir, "multiplayer_ai.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite DB %s: %w", dbPath, err)
	}

	// Optimize SQLite for concurrent access
	_, err = db.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set SQLite pragmas: %w", err)
	}

	// Create tables if not exists
	err = createTables(db)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// FindAndOpenSessionDBForWD scans the shared context directory and matches the database by working directory hash
func FindAndOpenSessionDBForWD(wd string) (*sql.DB, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user home dir: %w", err)
	}

	sharedContextDir := filepath.Join(home, ".mpai", "shared context")
	if _, err := os.Stat(sharedContextDir); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("shared context folder does not exist")
	}

	hash := GetWDHash(wd)
	suffix := "_" + hash

	entries, err := os.ReadDir(sharedContextDir)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read shared context folder: %w", err)
	}

	var matchedDir string
	var sessionID string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), suffix) {
			matchedDir = filepath.Join(sharedContextDir, entry.Name())
			sessionID = strings.TrimSuffix(entry.Name(), suffix)
			break
		}
	}

	if matchedDir == "" {
		return nil, "", fmt.Errorf("no session DB folder found matching working directory hash %s", hash)
	}

	dbPath := filepath.Join(matchedDir, "multiplayer_ai.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open sqlite DB %s: %w", dbPath, err)
	}

	// Optimize SQLite for concurrent access
	_, err = db.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;")
	if err != nil {
		db.Close()
		return nil, "", fmt.Errorf("failed to set SQLite pragmas: %w", err)
	}

	return db, sessionID, nil
}

func createTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			owner_id TEXT,
			project_storage_path TEXT,
			local_project_path TEXT,
			status TEXT,
			created_at TEXT,
			last_active_at TEXT,
			is_active INTEGER DEFAULT 0,
			active_user_id TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS ai_messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			sender_id TEXT NOT NULL,
			modifier TEXT NOT NULL,
			content TEXT NOT NULL,
			step_index INTEGER,
			created_at TEXT NOT NULL,
			FOREIGN KEY(session_id) REFERENCES sessions(id)
		);`,
		`CREATE TABLE IF NOT EXISTS file_changes (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			sender_id TEXT NOT NULL,
			file_path TEXT NOT NULL,
			operation TEXT NOT NULL,
			modifier TEXT NOT NULL,
			is_ai_edit INTEGER NOT NULL,
			change_content TEXT,
			created_at TEXT NOT NULL,
			FOREIGN KEY(session_id) REFERENCES sessions(id)
		);`,
		`CREATE TABLE IF NOT EXISTS local_ai_messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			sender_id TEXT NOT NULL,
			modifier TEXT NOT NULL,
			content TEXT NOT NULL,
			step_index INTEGER,
			created_at TEXT NOT NULL
		);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("failed to execute query %s: %w", q, err)
		}
	}
	return nil
}

// SetActiveSession inserts or replaces the session record in this database
func SetActiveSession(db *sql.DB, s *session.Session, userID, localPath string) error {
	localPathClean := filepath.ToSlash(localPath)

	query := `INSERT OR REPLACE INTO sessions 
		(id, name, owner_id, project_storage_path, local_project_path, status, created_at, last_active_at, is_active, active_user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`
	_, err := db.Exec(query, s.ID, s.Name, s.OwnerID, s.ProjectStoragePath, localPathClean, s.Status, s.CreatedAt, s.LastActiveAt, userID)
	if err != nil {
		return fmt.Errorf("failed to save active session: %w", err)
	}
	return nil
}

// GetActiveSession retrieves the session details in this database
func GetActiveSession(db *sql.DB) (*session.Session, string, error) {
	query := `SELECT id, name, owner_id, project_storage_path, status, created_at, last_active_at, active_user_id 
		FROM sessions LIMIT 1`
	row := db.QueryRow(query)

	var s session.Session
	var activeUserID string
	err := row.Scan(&s.ID, &s.Name, &s.OwnerID, &s.ProjectStoragePath, &s.Status, &s.CreatedAt, &s.LastActiveAt, &activeUserID)
	if err == sql.ErrNoRows {
		return nil, "", nil
	} else if err != nil {
		return nil, "", fmt.Errorf("failed to query session record: %w", err)
	}
	return &s, activeUserID, nil
}

// SaveAIMessage inserts an AI message into the database
func SaveAIMessage(db *sql.DB, id, sessionID, senderID, modifier, content string, stepIndex int, createdAt time.Time) error {
	query := `INSERT OR IGNORE INTO ai_messages (id, session_id, sender_id, modifier, content, step_index, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(query, id, sessionID, senderID, modifier, content, stepIndex, createdAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to save AI message: %w", err)
	}
	return nil
}

// HasAIMessage returns true if the message with stepIndex exists for the session
func HasAIMessage(db *sql.DB, sessionID string, stepIndex int) (bool, error) {
	query := "SELECT COUNT(*) FROM ai_messages WHERE session_id = ? AND step_index = ?"
	var count int
	err := db.QueryRow(query, sessionID, stepIndex).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to query if AI message exists: %w", err)
	}
	return count > 0, nil
}

// GetSessionMessages retrieves all AI messages for the session sorted chronologically
func GetSessionMessages(db *sql.DB, sessionID string, limit int) ([]session.AIMessage, error) {
	query := `SELECT id, session_id, sender_id, modifier, content, step_index, created_at 
		FROM ai_messages WHERE session_id = ? 
		ORDER BY step_index DESC, created_at DESC LIMIT ?`
	rows, err := db.Query(query, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query AI messages: %w", err)
	}
	defer rows.Close()

	var messages []session.AIMessage
	for rows.Next() {
		var m session.AIMessage
		var createdAtStr string
		err := rows.Scan(&m.ID, &m.SessionID, &m.SenderID, &m.Modifier, &m.Content, &m.StepIndex, &createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed to scan AI message: %w", err)
		}
		t, err := time.Parse(time.RFC3339, createdAtStr)
		if err == nil {
			m.CreatedAt = t
		} else {
			m.CreatedAt = time.Now()
		}
		messages = append(messages, m)
	}

	// Reverse to return chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// SaveFileChange inserts a file change record into the database
func SaveFileChange(db *sql.DB, id, sessionID, senderID, filePath, operation, modifier string, isAiEdit bool, changeContent string, createdAt time.Time) error {
	query := `INSERT OR IGNORE INTO file_changes (id, session_id, sender_id, file_path, operation, modifier, is_ai_edit, change_content, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	isAiVal := 0
	if isAiEdit {
		isAiVal = 1
	}
	_, err := db.Exec(query, id, sessionID, senderID, filePath, operation, modifier, isAiVal, changeContent, createdAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to save file change: %w", err)
	}
	return nil
}

// GetFileChanges retrieves all file changes for the session sorted chronologically
func GetFileChanges(db *sql.DB, sessionID string, limit int) ([]session.FileChange, error) {
	query := `SELECT id, session_id, sender_id, file_path, operation, modifier, is_ai_edit, change_content, created_at 
		FROM file_changes WHERE session_id = ? 
		ORDER BY created_at DESC LIMIT ?`
	rows, err := db.Query(query, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query file changes: %w", err)
	}
	defer rows.Close()

	var changes []session.FileChange
	for rows.Next() {
		var c session.FileChange
		var createdAtStr string
		var isAiVal int
		err := rows.Scan(&c.ID, &c.SessionID, &c.SenderID, &c.FilePath, &c.Operation, &c.Modifier, &isAiVal, &c.ChangeContent, &createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file change: %w", err)
		}
		c.IsAiEdit = (isAiVal == 1)
		t, err := time.Parse(time.RFC3339, createdAtStr)
		if err == nil {
			c.CreatedAt = t
		} else {
			c.CreatedAt = time.Now()
		}
		changes = append(changes, c)
	}

	// Reverse to return chronological order
	for i, j := 0, len(changes)-1; i < j; i, j = i+1, j-1 {
		changes[i], changes[j] = changes[j], changes[i]
	}

	return changes, nil
}

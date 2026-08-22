package contextengine

/*
import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"multiplayer_ai_client/session"

	"github.com/google/uuid"
)

// TranscriptLine represents a single line in the JSONL transcript file.
type TranscriptLine struct {
	StepIndex int    `json:"step_index"`
	Source    string `json:"source"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	Content   string `json:"content"`
}

type AITranscriptPoller struct {
	sessionID   string
	userID      string
	projectPath string
	db          *sql.DB
	broadcastFn func(modifier string, content string, stepIndex int)
	engine      session.ContextEngine
}

func NewAITranscriptPoller(sessionID, userID, projectPath string, db *sql.DB, broadcastFn func(modifier string, content string, stepIndex int), engine session.ContextEngine) *AITranscriptPoller {
	return &AITranscriptPoller{
		sessionID:   sessionID,
		userID:      userID,
		projectPath: projectPath,
		db:          db,
		broadcastFn: broadcastFn,
		engine:      engine,
	}
}

// Start runs the polling loop. It exits when context is cancelled.
func (p *AITranscriptPoller) Start(ctx context.Context) {
	p.engine.LogInfo("Starting background AI transcript poller...")
	home, err := os.UserHomeDir()
	if err != nil {
		p.engine.LogError("Error: failed to get user home: %v", err)
		return
	}

	brainDir := filepath.Join(home, ".gemini", "antigravity-ide", "brain")
	startTime := time.Now().Add(-2 * time.Second) // 2-second buffer

	var activePath string
	var lastSize int64
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.engine.LogInfo("Stopped AI transcript poller.")
			return
		case <-ticker.C:
			// Discover transcript path if not found yet
			if activePath == "" {
				path, err := p.findTranscriptForProject(brainDir)
				if err != nil {
					// Silent retry, might not be created yet
					continue
				}
				activePath = path
				p.engine.LogInfo("Mapped active AI transcript to: %s", activePath)
			}

			// Get metadata
			info, err := os.Stat(activePath)
			if err != nil {
				// File went away or error
				activePath = ""
				lastSize = 0
				continue
			}

			if info.Size() != lastSize {
				p.processNewEntries(activePath, &lastSize, startTime)
			}
		}
	}
}

func (p *AITranscriptPoller) findTranscriptForProject(brainDir string) (string, error) {
	entries, err := os.ReadDir(brainDir)
	if err != nil {
		return "", err
	}

	var latestPath string
	var latestTime time.Time

	// Normalise project path for substring match
	projClean := strings.ToLower(filepath.ToSlash(p.projectPath))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		transcriptPath := filepath.Join(brainDir, entry.Name(), ".system_generated", "logs", "transcript.jsonl")
		info, err := os.Stat(transcriptPath)
		if err != nil {
			continue
		}

		// Read first part of the file to see if it mentions our project storage path
		matched, err := p.isTranscriptRelevant(transcriptPath, projClean)
		if err == nil && matched {
			if info.ModTime().After(latestTime) {
				latestTime = info.ModTime()
				latestPath = transcriptPath
			}
		}
	}

	if latestPath == "" {
		return "", fmt.Errorf("no matching transcript for project path %s", p.projectPath)
	}

	return latestPath, nil
}

func (p *AITranscriptPoller) isTranscriptRelevant(path string, targetProj string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	// Read first 30 lines or up to 64KB, which is plenty to find step 0 USER_INPUT
	scanner := bufio.NewScanner(file)
	linesRead := 0
	for scanner.Scan() && linesRead < 30 {
		line := strings.ToLower(scanner.Text())
		if strings.Contains(line, targetProj) {
			return true, nil
		}
		linesRead++
	}
	return false, nil
}

func (p *AITranscriptPoller) processNewEntries(path string, startOffset *int64, startTime time.Time) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return
	}

	// Reset if file was truncated
	if info.Size() < *startOffset {
		*startOffset = 0
	}

	_, err = file.Seek(*startOffset, io.SeekStart)
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		if len(lineBytes) == 0 {
			continue
		}

		var line TranscriptLine
		if err := json.Unmarshal(lineBytes, &line); err != nil {
			continue
		}

		// Pick up AI responses
		if line.Source == "MODEL" && line.Type == "PLANNER_RESPONSE" && line.Content != "" {
			createdAt, err := time.Parse(time.RFC3339, line.CreatedAt)
			if err != nil {
				createdAt = time.Now()
			}

			// Check if already processed
			exists, err := HasAIMessage(p.db, p.sessionID, line.StepIndex)
			if err == nil && !exists {
				// Generate UUID for the local DB
				msgID := uuid.New().String()
				// Save locally
				_ = SaveAIMessage(p.db, msgID, p.sessionID, p.userID, "AI", line.Content, line.StepIndex, createdAt)

				// Only broadcast if it is a new message created after startup
				if createdAt.After(startTime) {
					p.broadcastFn("AI (via Antigravity)", line.Content, line.StepIndex)
				}
			}
		}
	}

	*startOffset = info.Size()
}
*/


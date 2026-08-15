package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

var defaultAIProcesses = []string{
	"cursor", "cursor.exe", "windsurf", "windsurf.exe",
	"code", "code.exe", "claude", "claude.exe",
	"gemini", "gemini.exe", "copilot-agent", "copilot-agent.exe",
	"antigravity", "antigravity.exe", "Antigravity IDE.exe", "cline", "cline.exe",
	"roo-cline", "roo-cline.exe",
}

func isAIProcessName(name string, aiNames map[string]bool) bool {
	nameLower := strings.ToLower(name)
	if aiNames[nameLower] {
		return true
	}
	nameNoExe := strings.TrimSuffix(nameLower, ".exe")
	if aiNames[nameNoExe] {
		return true
	}
	for aiName := range aiNames {
		if len(aiName) > 4 && strings.Contains(nameLower, aiName) {
			return true
		}
		if aiName == "code" && (strings.HasPrefix(nameLower, "code") || strings.Contains(nameLower, "vscode")) {
			return true
		}
	}
	return false
}

// FileSnapshotManager holds baseline lines of text files.
type FileSnapshotManager struct {
	mu        sync.RWMutex
	snapshots map[string][]string
}

func NewFileSnapshotManager() *FileSnapshotManager {
	return &FileSnapshotManager{
		snapshots: make(map[string][]string),
	}
}

func (fsm *FileSnapshotManager) Get(path string) ([]string, bool) {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	lines, ok := fsm.snapshots[path]
	return lines, ok
}

func (fsm *FileSnapshotManager) Set(path string, lines []string) {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()
	fsm.snapshots[path] = lines
}

func (fsm *FileSnapshotManager) Remove(path string) {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()
	delete(fsm.snapshots, path)
}

type DiffOpKind int

const (
	DiffEqual DiffOpKind = iota
	DiffInsert
	DiffDelete
)

type DiffChunk struct {
	Kind    DiffOpKind
	OldLine int
	NewLine int
	Text    string
}

func computeLineDiff(oldLines, newLines []string) []DiffChunk {
	n := len(oldLines)
	m := len(newLines)

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}

	for i := 0; i < n; i++ {
		for j := 0; j < m; j++ {
			if oldLines[i] == newLines[j] {
				dp[i+1][j+1] = dp[i][j] + 1
			} else {
				if dp[i][j+1] > dp[i+1][j] {
					dp[i+1][j+1] = dp[i][j+1]
				} else {
					dp[i+1][j+1] = dp[i+1][j]
				}
			}
		}
	}

	var revChunks []DiffChunk
	i, j := n, m
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			revChunks = append(revChunks, DiffChunk{
				Kind:    DiffEqual,
				OldLine: i,
				NewLine: j,
				Text:    oldLines[i-1],
			})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			revChunks = append(revChunks, DiffChunk{
				Kind:    DiffInsert,
				NewLine: j,
				Text:    newLines[j-1],
			})
			j--
		} else if i > 0 && (j == 0 || dp[i][j-1] < dp[i-1][j]) {
			revChunks = append(revChunks, DiffChunk{
				Kind:    DiffDelete,
				OldLine: i,
				Text:    oldLines[i-1],
			})
			i--
		}
	}

	chunks := make([]DiffChunk, len(revChunks))
	for idx, c := range revChunks {
		chunks[len(revChunks)-1-idx] = c
	}

	return chunks
}

func isTextFile(path string, sizeBytes int64) bool {
	if sizeBytes > 2*1024*1024 { // Skip files > 2MB
		return false
	}

	ext := strings.ToLower(filepath.Ext(path))
	knownTextExts := map[string]bool{
		".txt": true, ".cpp": true, ".hpp": true, ".c": true, ".h": true,
		".go": true, ".rs": true, ".py": true, ".json": true, ".md": true,
		".html": true, ".css": true, ".js": true, ".ts": true, ".jsx": true,
		".tsx": true, ".xml": true, ".yaml": true, ".yml": true, ".sh": true,
		".bat": true, ".ps1": true, ".sql": true, ".ini": true, ".toml": true,
		".log": true, ".env": true, ".csv": true, ".properties": true,
	}

	if knownTextExts[ext] {
		return true
	}

	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return false
	}

	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return false
		}
	}

	return true
}

func readLinesWithRetry(path string) ([]string, error) {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		data, err := os.ReadFile(path)
		if err == nil {
			text := strings.ReplaceAll(string(data), "\r\n", "\n")
			text = strings.ReplaceAll(text, "\r", "\n")
			return strings.Split(text, "\n"), nil
		}
		lastErr = err
		time.Sleep(25 * time.Millisecond)
	}
	return nil, lastErr
}

type DiffLine struct {
	LineNum int    `json:"lineNum"`
	Text    string `json:"text"`
}

type ContentDiffResult struct {
	IsText       bool       `json:"isText"`
	IsWholeFile  bool       `json:"isWholeFile"`
	AddedLines   []DiffLine `json:"added"`
	RemovedLines []DiffLine `json:"removed"`
}

type FileChangeCallback func(filePathFromRoot string, fileName string, fileExtension string, operation string, sizeBytes int64, modifier string, isAiEdit bool, isRevert bool, isWholeFile bool, contentDeltaJSON string)

type HighPrecisionWatcher struct {
	watcher        *fsnotify.Watcher
	mu             sync.RWMutex
	watched        map[string]bool
	snapshots      *FileSnapshotManager
	aiProcessNames map[string]bool
	watchDir       string
	activeAIPIDs   map[uint32]string
	callback       FileChangeCallback

	incomingWrites map[string]time.Time
	incomingMu     sync.Mutex
}

func NewHighPrecisionWatcher(watchDir string, callback FileChangeCallback) (*HighPrecisionWatcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	aiNames := make(map[string]bool)
	for _, name := range defaultAIProcesses {
		aiNames[strings.ToLower(name)] = true
	}

	absWatch, err := filepath.Abs(watchDir)
	if err != nil {
		absWatch = watchDir
	}

	return &HighPrecisionWatcher{
		watcher:        fw,
		watched:        make(map[string]bool),
		snapshots:      NewFileSnapshotManager(),
		aiProcessNames: aiNames,
		watchDir:       absWatch,
		activeAIPIDs:   make(map[uint32]string),
		callback:       callback,
		incomingWrites: make(map[string]time.Time),
	}, nil
}

// IgnorePath registers an absolute path that should temporarily skip watcher triggers.
func (hpw *HighPrecisionWatcher) IgnorePath(absPath string) {
	hpw.incomingMu.Lock()
	defer hpw.incomingMu.Unlock()
	// Set ignore window (2 seconds)
	hpw.incomingWrites[filepath.Clean(absPath)] = time.Now().Add(2 * time.Second)
}

func (hpw *HighPrecisionWatcher) isIgnored(absPath string) bool {
	hpw.incomingMu.Lock()
	defer hpw.incomingMu.Unlock()
	cleanPath := filepath.Clean(absPath)
	expiry, exists := hpw.incomingWrites[cleanPath]
	if !exists {
		return false
	}
	if time.Now().Before(expiry) {
		return true
	}
	delete(hpw.incomingWrites, cleanPath)
	return false
}

func shouldIgnore(path string) bool {
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") && base != "." && base != ".." {
		return true
	}
	ignoredDirs := map[string]bool{
		"node_modules": true,
		"target":       true,
		"bin":          true,
		"obj":          true,
		"dist":         true,
		".git":         true,
		".idea":        true,
		".gradle":      true,
	}
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		if ignoredDirs[strings.ToLower(part)] {
			return true
		}
	}
	if strings.HasSuffix(base, ".exe") || base == "multiplayer_ai_client" {
		return true
	}
	return false
}

func (hpw *HighPrecisionWatcher) AddRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		absPath, absErr := filepath.Abs(path)
		if absErr != nil {
			absPath = path
		}

		if shouldIgnore(absPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			if err := hpw.addDir(absPath); err != nil {
				// Log or skip
			}
		} else {
			if info, err := d.Info(); err == nil && isTextFile(absPath, info.Size()) {
				if lines, err := readLinesWithRetry(absPath); err == nil {
					hpw.snapshots.Set(absPath, lines)
				}
			}
		}
		return nil
	})
}

func (hpw *HighPrecisionWatcher) addDir(path string) error {
	hpw.mu.Lock()
	defer hpw.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	if hpw.watched[absPath] {
		return nil
	}

	if err := hpw.watcher.Add(absPath); err != nil {
		return err
	}

	hpw.watched[absPath] = true
	return nil
}

func (hpw *HighPrecisionWatcher) removeDir(path string) {
	hpw.mu.Lock()
	defer hpw.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	delete(hpw.watched, absPath)
}

func (hpw *HighPrecisionWatcher) Start(ctx context.Context) error {
	defer hpw.watcher.Close()

	// Periodic AI process scanner
	go func() {
		if active, err := findActiveAIProcesses(hpw.aiProcessNames); err == nil {
			hpw.mu.Lock()
			hpw.activeAIPIDs = active
			hpw.mu.Unlock()
		}

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if active, err := findActiveAIProcesses(hpw.aiProcessNames); err == nil {
					hpw.mu.Lock()
					hpw.activeAIPIDs = active
					hpw.mu.Unlock()
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil

		case event, ok := <-hpw.watcher.Events:
			if !ok {
				return nil
			}
			hpw.handleEvent(event)

		case _, ok := <-hpw.watcher.Errors:
			if !ok {
				return nil
			}
		}
	}
}

func (hpw *HighPrecisionWatcher) handleEvent(event fsnotify.Event) {
	absPath, err := filepath.Abs(event.Name)
	if err != nil {
		absPath = event.Name
	}

	if shouldIgnore(absPath) {
		return
	}

	if hpw.isIgnored(absPath) {
		return
	}

	info, statErr := os.Stat(absPath)
	exists := statErr == nil

	var isDir bool
	var sizeBytes int64

	if exists && info != nil {
		isDir = info.IsDir()
		sizeBytes = info.Size()

		if event.Has(fsnotify.Create) && isDir {
			if err := hpw.AddRecursive(absPath); err != nil {
				// Log or skip
			}
		}
	}

	if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		hpw.removeDir(absPath)
		hpw.snapshots.Remove(absPath)

		// Trigger deletion callback
		relPath, _ := filepath.Rel(hpw.watchDir, absPath)
		hpw.callback(relPath, filepath.Base(absPath), filepath.Ext(absPath), "REMOVE", 0, "User", false, false, false, "")
		return
	}

	// Capture process locking modifiers
	var modifierProcesses []string
	if exists && !isDir && (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) {
		if _, names, err := findLockingProcesses(absPath); err == nil && len(names) > 0 {
			modifierProcesses = names
		}
	}

	modifier := "User"
	isAI := false
	if len(modifierProcesses) > 0 {
		for _, name := range modifierProcesses {
			if isAIProcessName(name, hpw.aiProcessNames) {
				modifier = fmt.Sprintf("AI (via %s)", name)
				isAI = true
				break
			}
		}
	}

	if exists && !isDir && (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) {
		delta := hpw.analyzeContentDelta(absPath, sizeBytes)
		if delta != nil && delta.IsText {
			deltaJSON, _ := json.Marshal(delta)
			relPath, _ := filepath.Rel(hpw.watchDir, absPath)
			op := "WRITE"
			if event.Has(fsnotify.Create) {
				op = "CREATE"
			}
			hpw.callback(relPath, filepath.Base(absPath), filepath.Ext(absPath), op, sizeBytes, modifier, isAI, false, delta.IsWholeFile, string(deltaJSON))
		}
	}
}

func (hpw *HighPrecisionWatcher) analyzeContentDelta(absPath string, sizeBytes int64) *ContentDiffResult {
	if !isTextFile(absPath, sizeBytes) {
		return &ContentDiffResult{IsText: false}
	}

	time.Sleep(35 * time.Millisecond)

	currentLines, err := readLinesWithRetry(absPath)
	if err != nil {
		return &ContentDiffResult{IsText: true}
	}

	prevLines, cached := hpw.snapshots.Get(absPath)
	if !cached {
		prevLines = []string{}
	}

	chunks := computeLineDiff(prevLines, currentLines)

	var added []DiffLine
	var removed []DiffLine
	equalCount := 0

	for _, c := range chunks {
		switch c.Kind {
		case DiffEqual:
			equalCount++
		case DiffInsert:
			added = append(added, DiffLine{LineNum: c.NewLine, Text: c.Text})
		case DiffDelete:
			removed = append(removed, DiffLine{LineNum: c.OldLine, Text: c.Text})
		}
	}

	if len(added) == 0 && len(removed) == 0 && cached {
		return &ContentDiffResult{
			IsText:       true,
			IsWholeFile:  false,
			AddedLines:   nil,
			RemovedLines: nil,
		}
	}

	hpw.snapshots.Set(absPath, currentLines)

	if (len(prevLines) == 0 || equalCount == 0) && len(currentLines) > 50 {
		return &ContentDiffResult{
			IsText:      true,
			IsWholeFile: true,
			AddedLines:  added, // If whole file, added lines are all lines
		}
	}

	return &ContentDiffResult{
		IsText:       true,
		IsWholeFile:  false,
		AddedLines:   added,
		RemovedLines: removed,
	}
}

func CleanDirectoryIgnore(dir string) error {
	// Standard zip helper CleanDirectory handles this
	return nil
}

func UnzipBytesIgnore(zipBytes []byte, dest string) error {
	// Standard zip helper handles this
	return nil
}

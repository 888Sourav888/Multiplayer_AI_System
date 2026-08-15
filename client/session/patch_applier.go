package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ApplyPatch applies a remote file change patch (diff or deletion) locally.
func ApplyPatch(watchDir string, filePathFromRoot string, operation string, isWholeFile bool, contentDeltaJSON string) error {
	absPath := filepath.Clean(filepath.Join(watchDir, filePathFromRoot))

	if operation == "REMOVE" {
		fmt.Printf("   [SYNC] Removing file: %s\n", filePathFromRoot)
		err := os.Remove(absPath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove file %s: %w", absPath, err)
		}
		return nil
	}

	// Create directory if not exists
	parentDir := filepath.Dir(absPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}

	// If isWholeFile, we reconstruct the file from AddedLines
	if isWholeFile {
		var delta ContentDiffResult
		if contentDeltaJSON != "" {
			if err := json.Unmarshal([]byte(contentDeltaJSON), &delta); err == nil && len(delta.AddedLines) > 0 {
				var lines []string
				sort.Slice(delta.AddedLines, func(i, j int) bool {
					return delta.AddedLines[i].LineNum < delta.AddedLines[j].LineNum
				})
				for _, line := range delta.AddedLines {
					lines = append(lines, line.Text)
				}
				content := strings.Join(lines, "\n")
				fmt.Printf("   [SYNC] Overwriting file (whole file): %s\n", filePathFromRoot)
				return os.WriteFile(absPath, []byte(content), 0644)
			}
		}
		return nil
	}

	// Otherwise, apply incremental delta additions and removals
	var delta ContentDiffResult
	if err := json.Unmarshal([]byte(contentDeltaJSON), &delta); err != nil {
		return fmt.Errorf("failed to unmarshal content delta: %w", err)
	}

	// Read current file lines
	var currentLines []string
	if fileBytes, err := os.ReadFile(absPath); err == nil {
		text := strings.ReplaceAll(string(fileBytes), "\r\n", "\n")
		text = strings.ReplaceAll(text, "\r", "\n")
		currentLines = strings.Split(text, "\n")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read existing file: %w", err)
	}

	// 1. Apply removals (bottom-to-top, descending LineNum)
	if len(delta.RemovedLines) > 0 {
		sort.Slice(delta.RemovedLines, func(i, j int) bool {
			return delta.RemovedLines[i].LineNum > delta.RemovedLines[j].LineNum
		})
		for _, r := range delta.RemovedLines {
			idx := r.LineNum - 1
			if idx >= 0 && idx < len(currentLines) {
				currentLines = append(currentLines[:idx], currentLines[idx+1:]...)
			}
		}
	}

	// 2. Apply additions (top-to-bottom, ascending LineNum)
	if len(delta.AddedLines) > 0 {
		sort.Slice(delta.AddedLines, func(i, j int) bool {
			return delta.AddedLines[i].LineNum < delta.AddedLines[j].LineNum
		})
		for _, a := range delta.AddedLines {
			idx := a.LineNum - 1
			if idx < 0 {
				idx = 0
			}
			if idx >= len(currentLines) {
				currentLines = append(currentLines, a.Text)
			} else {
				currentLines = append(currentLines[:idx], append([]string{a.Text}, currentLines[idx:]...)...)
			}
		}
	}

	content := strings.Join(currentLines, "\n")
	fmt.Printf("   [SYNC] Patching file (%d additions, %d deletions): %s\n", len(delta.AddedLines), len(delta.RemovedLines), filePathFromRoot)
	return os.WriteFile(absPath, []byte(content), 0644)
}

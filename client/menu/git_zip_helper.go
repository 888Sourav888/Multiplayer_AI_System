package menu

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitInfo holds git repository metadata.
type GitInfo struct {
	RepoURL   string
	Branch    string
	CommitSHA string
}

// GetGitInfo checks if the current working directory is a git repository
// and extracts repo URL, branch, and commit SHA if it is.
func GetGitInfo() (*GitInfo, bool) {
	// Check if git is installed and directory is inside a git worktree
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	output, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) != "true" {
		return nil, false
	}

	// Extract remote origin URL
	cmd = exec.Command("git", "config", "--get", "remote.origin.url")
	repoURLBytes, err := cmd.Output()
	var repoURL string
	if err == nil {
		repoURL = strings.TrimSpace(string(repoURLBytes))
	}

	// Extract branch name
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchBytes, err := cmd.Output()
	var branch string
	if err == nil {
		branch = strings.TrimSpace(string(branchBytes))
	}

	// Extract commit SHA
	cmd = exec.Command("git", "rev-parse", "HEAD")
	shaBytes, err := cmd.Output()
	var sha string
	if err == nil {
		sha = strings.TrimSpace(string(shaBytes))
	}

	return &GitInfo{
		RepoURL:   repoURL,
		Branch:    branch,
		CommitSHA: sha,
	}, true
}

// ZipDirectory creates a zip archive of the specified directory source in-memory.
// It skips output binaries and hidden .git folders to keep the size minimal.
func ZipDirectory(src string) ([]byte, error) {
	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Relativize the path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		// Skip hidden folders like .git
		parts := strings.Split(filepath.ToSlash(relPath), "/")
		for _, part := range parts {
			if strings.HasPrefix(part, ".") && part != "." && part != ".." {
				return filepath.SkipDir
			}
		}

		// Skip output binaries
		base := filepath.Base(path)
		if strings.HasSuffix(base, ".exe") || base == "multiplayer_ai_client" {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name = filepath.ToSlash(relPath)
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	if err != nil {
		return nil, err
	}

	if err := archive.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

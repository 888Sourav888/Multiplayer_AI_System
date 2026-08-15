//go:build !windows

package session

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func getProcessNameByPID(pid uint32) (string, error) {
	// Try /proc/<pid>/comm (Linux)
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid)); err == nil {
		return strings.TrimSpace(string(data)), nil
	}

	// Try ps command (macOS/Unix fallback)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(int(pid)), "-o", "comm=")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		name := strings.TrimSpace(out.String())
		if name != "" {
			return filepath.Base(name), nil
		}
	}

	return "", fmt.Errorf("unable to resolve process name for PID %d", pid)
}

func findLockingProcesses(filePath string) ([]uint32, []string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	cmd := exec.CommandContext(ctx, "lsof", "-t", absPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, nil, err
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var pids []uint32
	var names []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pidVal, err := strconv.ParseUint(line, 10, 32)
		if err != nil {
			continue
		}
		pid := uint32(pidVal)
		pids = append(pids, pid)

		name, _ := getProcessNameByPID(pid)
		if name == "" {
			name = "Unknown"
		}
		names = append(names, name)
	}

	return pids, names, nil
}

func findActiveAIProcesses(aiNames map[string]bool) (map[uint32]string, error) {
	results := make(map[uint32]string)

	// Try reading /proc directory (Linux)
	files, err := filepath.Glob("/proc/[0-9]*")
	if err == nil && len(files) > 0 {
		for _, f := range files {
			pidStr := filepath.Base(f)
			pidVal, err := strconv.ParseUint(pidStr, 10, 32)
			if err != nil {
				continue
			}
			pid := uint32(pidVal)
			if comm, err := os.ReadFile(filepath.Join(f, "comm")); err == nil {
				name := strings.TrimSpace(string(comm))
				if isAIProcessName(name, aiNames) {
					results[pid] = name
				}
			}
		}
		return results, nil
	}

	// Try running ps command (macOS fallback)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-ax", "-o", "pid,comm")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		lines := strings.Split(out.String(), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			pidVal, err := strconv.ParseUint(parts[0], 10, 32)
			if err != nil {
				continue
			}
			pid := uint32(pidVal)
			name := filepath.Base(parts[1])
			if isAIProcessName(name, aiNames) {
				results[pid] = name
			}
		}
	}

	return results, nil
}

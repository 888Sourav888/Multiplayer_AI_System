package contextengine

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// JSONRPCRequest represents an incoming JSON-RPC request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents an outgoing JSON-RPC response.
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error.
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ToolCallParams parses the arguments of tools/call.
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}



func readDBPathFromRules(cwd string) (string, string, error) {
	paths := []string{
		filepath.Join(cwd, ".cursor/rules/multiplayer.mdc"),
		filepath.Join(cwd, ".cursorrules"),
		filepath.Join(cwd, ".clinerules"),
	}

	for _, p := range paths {
		file, err := os.Open(p)
		if err != nil {
			continue
		}
		defer file.Close()

		var dbPath string
		var sessionID string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "# DB Path:") {
				dbPath = strings.TrimSpace(strings.TrimPrefix(line, "# DB Path:"))
			} else if strings.HasPrefix(line, "# Session ID:") {
				sessionID = strings.TrimSpace(strings.TrimPrefix(line, "# Session ID:"))
			}
		}
		if dbPath != "" {
			return dbPath, sessionID, nil
		}
	}
	return "", "", fmt.Errorf("metadata not found in rule files")
}

var mcpWorkspaceDir string

func getSessionDB() (*sql.DB, string, error) {
	cwd := mcpWorkspaceDir
	if cwd == "" {
		cwd = "."
	}
	if abs, err := filepath.Abs(cwd); err == nil {
		cwd = abs
	}

	// 1. Try reading the DB path and Session ID from workspace rules first
	ruleDBPath, ruleSessionID, err := readDBPathFromRules(cwd)
	if err == nil && ruleDBPath != "" {
		db, err := sql.Open("sqlite", ruleDBPath)
		if err == nil {
			// Optimize SQLite for concurrent access
			_, err = db.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;")
			if err == nil {
				return db, ruleSessionID, nil
			}
			db.Close()
		}
	}

	// 2. Fallback to directory scanning
	return FindAndOpenSessionDBForWD(cwd)
}

func initMCPLogger(cwd string) {
	var dbDir string
	dbPath, _, err := readDBPathFromRules(cwd)
	if err == nil && dbPath != "" {
		dbDir = filepath.Dir(dbPath)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			hash := GetWDHash(cwd)
			suffix := "_" + hash
			sharedContextDir := filepath.Join(home, ".mpai", "shared context")
			entries, err := os.ReadDir(sharedContextDir)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() && strings.HasSuffix(entry.Name(), suffix) {
						dbDir = filepath.Join(sharedContextDir, entry.Name())
						break
					}
				}
			}
		}
	}

	if dbDir != "" {
		_ = os.MkdirAll(dbDir, 0755)
		logPath := filepath.Join(dbDir, "mcp.log")
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			log.SetOutput(io.MultiWriter(os.Stderr, file))
			log.Printf("[MCP] Logging initialized at: %s\n", logPath)
		}
	}
}

// RunMCPServer runs the main Model Context Protocol JSON-RPC stdin/stdout loop
func RunMCPServer() {
	// Set up logging to stderr since stdout is reserved for JSON-RPC
	log.SetOutput(os.Stderr)
	log.Println("[MCP] Starting Model Context Protocol Server...")

	// 1. Resolve workspace directory:
	// Priority 1: os.Args[2] (passed by IDE as ["mcp", "<path>"])
	// Priority 2: MPAI_WORKSPACE_DIR environment variable
	// Priority 3: os.Getwd()
	cwd := ""
	if len(os.Args) > 2 {
		cwd = os.Args[2]
	}
	if cwd == "" {
		cwd = os.Getenv("MPAI_WORKSPACE_DIR")
	}
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			cwd = "."
		}
	}
	if abs, err := filepath.Abs(cwd); err == nil {
		cwd = abs
	}
	mcpWorkspaceDir = cwd

	// 2. Initialize MCP logger in session dir if exists
	initMCPLogger(cwd)

	log.Printf("[MCP] Server initialized in workspace context '%s'\n", cwd)

	reader := bufio.NewReader(os.Stdin)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("[MCP] Error reading stdin: %v\n", err)
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			sendErrorResponse(nil, -32700, "Parse error", nil)
			continue
		}

		handleMCPRequest(&req)
	}
}

func handleMCPRequest(req *JSONRPCRequest) {
	log.Printf("[MCP Request] Method: %s, ID: %v\n", req.Method, req.ID)
	switch req.Method {
	case "initialize":
		result := map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]string{
				"name":    "Multiplayer-AI-MCP",
				"version": "1.0.0",
			},
		}
		sendResponse(req.ID, result)

	case "notifications/initialized":
		// No response required for notifications

	case "tools/list":
		tools := []map[string]interface{}{
			{
				"name":        "get_session_messages",
				"description": "Get chronological logs of AI coding messages and user updates inside the active session. You MUST call this at startup to align your state with other participants and their AIs.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum number of AI messages to fetch (default 50)",
							"minimum":     1,
						},
					},
				},
			},
		}
		result := map[string]interface{}{
			"tools": tools,
		}
		sendResponse(req.ID, result)

	case "tools/call":
		var params ToolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			sendErrorResponse(req.ID, -32602, "Invalid params", nil)
			return
		}

		handleToolCall(req.ID, params.Name, params.Arguments)

	default:
		sendErrorResponse(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method), nil)
	}
}

func handleToolCall(id interface{}, toolName string, argsJSON []byte) {
	log.Printf("[MCP Tool Call] Tool: %s, Args: %s\n", toolName, string(argsJSON))
	db, sessionID, err := getSessionDB()
	if err != nil {
		log.Printf("[MCP Tool Call Error] Failed to get session DB: %v\n", err)
		sendToolCallResultError(id, fmt.Sprintf("No active session found matching workspace directory. Please join a session using the multiplayer CLI client first. (Workspace: %s, Hash: %s, Error: %v)", mcpWorkspaceDir, GetWDHash(mcpWorkspaceDir), err))
		return
	}
	defer db.Close()
	_ = sessionID

	switch toolName {
	case "get_session_messages":
		// Parse limit
		var limitArg struct {
			Limit int `json:"limit"`
		}
		limitArg.Limit = 50 // default
		if len(argsJSON) > 0 {
			_ = json.Unmarshal(argsJSON, &limitArg)
		}

		s, _, err := GetActiveSession(db)
		if err != nil || s == nil {
			sendToolCallResultText(id, "No active session found matching this project directory. Please join a session first.")
			return
		}

		messages, err := GetSessionMessages(db, s.ID, limitArg.Limit)
		if err != nil {
			sendToolCallResultError(id, fmt.Sprintf("Failed to query messages: %v", err))
			return
		}

		if len(messages) == 0 {
			sendToolCallResultText(id, fmt.Sprintf("No AI coding messages stored yet in session %s.", s.Name))
			return
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("=== AI coding transcript logs for session: %s ===\n", s.Name))
		for _, m := range messages {
			sb.WriteString(fmt.Sprintf("\n[%s] %s (%s):\n%s\n",
				m.CreatedAt.Local().Format("2006-01-02 15:04:05"),
				m.Modifier,
				m.SenderID,
				m.Content,
			))
			sb.WriteString("--------------------------------------------------\n")
		}

		sendToolCallResultText(id, sb.String())

	default:
		sendToolCallResultError(id, fmt.Sprintf("Unknown tool: %s", toolName))
	}
}

func sendResponse(id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	bytes, _ := json.Marshal(resp)
	log.Printf("[MCP Response] ID: %v, Length: %d\n", id, len(bytes))
	fmt.Println(string(bytes))
}

func sendErrorResponse(id interface{}, code int, message string, data interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	bytes, _ := json.Marshal(resp)
	log.Printf("[MCP Error Response] ID: %v, Code: %d, Message: %s\n", id, code, message)
	fmt.Println(string(bytes))
}

type toolCallContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolCallResult struct {
	Content []toolCallContent `json:"content"`
	IsError bool              `json:"isError,omitempty"`
}

func sendToolCallResultText(id interface{}, text string) {
	sendResponse(id, toolCallResult{
		Content: []toolCallContent{
			{
				Type: "text",
				Text: text,
			},
		},
	})
}

func sendToolCallResultError(id interface{}, errMessage string) {
	sendResponse(id, toolCallResult{
		Content: []toolCallContent{
			{
				Type: "text",
				Text: fmt.Sprintf("Error: %s", errMessage),
			},
		},
		IsError: true,
	})
}

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
	"time"

	"multiplayer_ai_client/session"
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

type clientConfig struct {
	LowerEnvBackendURL string `json:"lowerEnvBackendURL"`
	ProdBackendURL     string `json:"prodBackendURL"`
}

// RunMCPServer runs the main Model Context Protocol JSON-RPC stdin/stdout loop
func RunMCPServer() {
	// Set up logging to stderr since stdout is reserved for JSON-RPC
	log.SetOutput(os.Stderr)
	log.Println("[MCP] Starting Model Context Protocol Server...")

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	db, sessionID, err := FindAndOpenSessionDBForWD(cwd)
	if err != nil {
		log.Printf("[MCP] Fatal error: failed to initialize SQLite DB for workspace '%s': %v\n", cwd, err)
		os.Exit(1)
	}
	defer db.Close()

	engine := NewSqliteContextEngine(db)
	engine.InitLogger(sessionID, cwd)
	defer engine.Close()

	log.Printf("[MCP] Resolved session ID '%s' for workspace '%s'\n", sessionID, cwd)

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

		handleMCPRequest(db, &req)
	}
}

func handleMCPRequest(db *sql.DB, req *JSONRPCRequest) {
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
				"name":        "get_active_session",
				"description": "Get the currently active multiplayer session details matching the workspace directory (ID, Name, status, project storage path, and active user ID). Use this to check the current session context.",
				"inputSchema": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
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
			{
				"name":        "get_file_changes_history",
				"description": "Get chronological history of recent file changes (additions, modifications, deletions) made by all developers and AI agents in the active session. This helps you track who made what file changes.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum number of file changes to fetch (default 50)",
							"minimum":     1,
						},
					},
				},
			},
			{
				"name":        "broadcast_ai_message",
				"description": "Broadcast an AI message or status update to all other developers and AI agents in the active multiplayer session.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"message": map[string]interface{}{
							"type":        "string",
							"description": "The text content or status report to broadcast.",
						},
					},
					"required": []string{"message"},
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

		handleToolCall(db, req.ID, params.Name, params.Arguments)

	default:
		sendErrorResponse(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method), nil)
	}
}

func handleToolCall(db *sql.DB, id interface{}, toolName string, argsJSON []byte) {
	switch toolName {
	case "get_active_session":
		s, activeUser, err := GetActiveSession(db)
		if err != nil {
			sendToolCallResultError(id, fmt.Sprintf("Failed to query active session: %v", err))
			return
		}
		if s == nil {
			sendToolCallResultText(id, "No active session found matching this project directory. Please join a session via the multiplayer CLI client first.")
			return
		}

		cwd, _ := os.Getwd()
		resStr := fmt.Sprintf("Active Session Details:\n- ID: %s\n- Name: %s\n- Owner ID: %s\n- Backend Storage Path: %s\n- Local Project Path: %s\n- Status: %s\n- Active User: %s",
			s.ID, s.Name, s.OwnerID, s.ProjectStoragePath, cwd, s.Status, activeUser)
		sendToolCallResultText(id, resStr)

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

	case "get_file_changes_history":
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

		changes, err := GetFileChanges(db, s.ID, limitArg.Limit)
		if err != nil {
			sendToolCallResultError(id, fmt.Sprintf("Failed to query file changes: %v", err))
			return
		}

		if len(changes) == 0 {
			sendToolCallResultText(id, fmt.Sprintf("No file changes recorded yet in session %s.", s.Name))
			return
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("=== File change history logs for session: %s ===\n", s.Name))
		for _, c := range changes {
			actor := "User"
			if c.IsAiEdit {
				actor = "AI"
			}
			sb.WriteString(fmt.Sprintf("\n[%s] %s (%s) performed %s on '%s' via '%s'\n",
				c.CreatedAt.Local().Format("2006-01-02 15:04:05"),
				actor,
				c.SenderID,
				c.Operation,
				c.FilePath,
				c.Modifier,
			))
			if len(c.ChangeContent) > 0 {
				contentTrunc := c.ChangeContent
				if len(contentTrunc) > 300 {
					contentTrunc = contentTrunc[:300] + "... [truncated]"
				}
				sb.WriteString(fmt.Sprintf("Changes:\n%s\n", contentTrunc))
			}
			sb.WriteString("--------------------------------------------------\n")
		}

		sendToolCallResultText(id, sb.String())

	case "broadcast_ai_message":
		var broadcastArg struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(argsJSON, &broadcastArg); err != nil || broadcastArg.Message == "" {
			sendToolCallResultError(id, "Invalid arguments: missing 'message'")
			return
		}

		s, activeUser, err := GetActiveSession(db)
		if err != nil || s == nil {
			sendToolCallResultText(id, "No active session found matching this project directory. Please join a session first.")
			return
		}

		// Load backend URL from config
		backendURL, err := loadBackendURL()
		if err != nil {
			sendToolCallResultError(id, fmt.Sprintf("Failed to resolve backend URL: %v", err))
			return
		}

		// Connect and send over WS
		err = SendMCPServerMessage(backendURL, s.ID, activeUser, broadcastArg.Message)
		if err != nil {
			sendToolCallResultError(id, fmt.Sprintf("Failed to broadcast message: %v", err))
			return
		}

		// Save the message locally as well so it's in the local DB
		msgID := fmt.Sprintf("mcp-%d", time.Now().UnixNano())
		_ = SaveAIMessage(db, msgID, s.ID, activeUser, "AI (via MCP Server)", broadcastArg.Message, -1, time.Now())

		sendToolCallResultText(id, fmt.Sprintf("Successfully broadcasted AI message to all other participants in session %s!", s.Name))

	default:
		sendToolCallResultError(id, fmt.Sprintf("Unknown tool: %s", toolName))
	}
}

func loadBackendURL() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	execDir := filepath.Dir(execPath)
	configPath := filepath.Join(execDir, "config.json")

	file, err := os.Open(configPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var cfg clientConfig
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return "", err
	}

	url := cfg.ProdBackendURL
	if url == "" {
		url = cfg.LowerEnvBackendURL
	}
	if url == "" {
		return "", fmt.Errorf("no backend URL configured in config.json")
	}
	return url, nil
}

func SendMCPServerMessage(backendURL, sessionID, userID, message string) error {
	backend := session.NewSessionBackend(backendURL)
	defer backend.Close()

	msgChan, err := backend.ConnectAndSubscribe(sessionID, userID)
	if err != nil {
		return err
	}
	_ = msgChan // not used

	patchItem := session.FilePatchItem{
		FilePathFromRoot: "ai_transcript.jsonl",
		FileName:         "transcript.jsonl",
		FileExtension:    ".jsonl",
		Operation:        "AI_MESSAGE",
		SizeBytes:        int64(len(message)),
		Modifier:         "AI (via MCP Server)",
		IsAiEdit:         false,
		IsWholeFile:      true,
		ContentDelta:     message,
		FileChanges:      message,
	}

	msg := session.WSMessage{
		Type:      "PATCH_TRANSFER",
		SessionID: sessionID,
		SenderID:  userID,
		Message:   "AI broadcast from MCP",
		Patches:   []session.FilePatchItem{patchItem},
		Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
	}

	return backend.SendMessage(msg)
}

func sendResponse(id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	bytes, _ := json.Marshal(resp)
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

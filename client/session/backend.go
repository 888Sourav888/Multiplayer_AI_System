package session

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// Session represents the session model within the active session scope.
type Session struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	OwnerID            string `json:"ownerId"`
	ProjectStoragePath string `json:"projectStoragePath"`
	CurrentVersion     int    `json:"currentVersion"`
	Status             string `json:"status"`
	CreatedAt          string `json:"createdAt"`
	LastActiveAt       string `json:"lastActiveAt"`
}

// FilePatchItem represents a single file change diff/patch.
type FilePatchItem struct {
	FilePathFromRoot string `json:"filePathFromRoot"`
	FileName         string `json:"fileName"`
	FileExtension    string `json:"fileExtension"`
	Operation        string `json:"operation"`
	SizeBytes        int64  `json:"sizeBytes"`
	Modifier         string `json:"modifier"`
	IsAiEdit         bool   `json:"isAiEdit"`
	IsRevert         bool   `json:"isRevert"`
	IsWholeFile      bool   `json:"isWholeFile"`
	ContentDelta     string `json:"contentDelta"`
	FileChanges      string `json:"fileChanges"`
}

// WSMessage represents a generic message sent or received via WebSocket.
type WSMessage struct {
	Type      string          `json:"type,omitempty"`
	Status    string          `json:"status,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	Message   string          `json:"message,omitempty"`
	SenderID  string          `json:"senderId,omitempty"`
	Error     string          `json:"error,omitempty"`
	Patches   []FilePatchItem `json:"patches,omitempty"`
	Timestamp int64           `json:"timestamp,omitempty"`
}

// SessionBackend handles WebSocket connections and REST actions for an active session.
type SessionBackend struct {
	BaseHTTPURL string
	wsConn      *websocket.Conn
	stopChan    chan struct{}
}

// NewSessionBackend creates a new SessionBackend instance.
func NewSessionBackend(baseHTTPURL string) *SessionBackend {
	return &SessionBackend{
		BaseHTTPURL: baseHTTPURL,
		stopChan:    make(chan struct{}),
	}
}

// GetSessionByID fetches the session detail via REST.
func (sb *SessionBackend) GetSessionByID(sessionID string) (*Session, error) {
	url := fmt.Sprintf("%s/api/sessions/%s", sb.BaseHTTPURL, sessionID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend returned non-OK status: %s (body: %s)", resp.Status, string(body))
	}

	var session Session
	if err := json.Unmarshal(body, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session response: %w (raw: %s)", err, string(body))
	}

	return &session, nil
}

// ConnectAndSubscribe establishes the WebSocket connection and sends the SUBSCRIBE message.
func (sb *SessionBackend) ConnectAndSubscribe(sessionID string) (<-chan WSMessage, error) {
	// Derive websocket URL from HTTP base URL
	wsURL := sb.BaseHTTPURL
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL = wsURL + "/ws-multiplayer"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("websocket connection failed: %w", err)
	}
	sb.wsConn = conn

	// 1. Read initial CONNECTED message from server
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read connection acknowledgment: %w", err)
	}

	var connAck WSMessage
	if err := json.Unmarshal(msgBytes, &connAck); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to parse connection acknowledgment: %w", err)
	}

	if connAck.Status != "CONNECTED" {
		conn.Close()
		return nil, fmt.Errorf("expected CONNECTED status, got: %s", connAck.Status)
	}

	// 2. Send SUBSCRIBE message
	subMsg := WSMessage{
		Type:      "SUBSCRIBE",
		SessionID: sessionID,
	}
	subBytes, err := json.Marshal(subMsg)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to marshal subscribe message: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, subBytes); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send subscribe message: %w", err)
	}

	// 3. Read SUBSCRIBED confirmation
	_, subAckBytes, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read subscription acknowledgment: %w", err)
	}

	var subAck WSMessage
	if err := json.Unmarshal(subAckBytes, &subAck); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to parse subscription acknowledgment: %w", err)
	}

	if subAck.Status != "SUBSCRIBED" {
		conn.Close()
		return nil, fmt.Errorf("expected SUBSCRIBED status, got: %s", subAck.Status)
	}

	// 4. Start background listener
	messageChan := make(chan WSMessage, 10)
	go sb.listen(messageChan)

	return messageChan, nil
}

// listen reads incoming messages from the WebSocket and pushes them to the channel.
func (sb *SessionBackend) listen(messageChan chan<- WSMessage) {
	defer close(messageChan)
	defer sb.Close()

	for {
		select {
		case <-sb.stopChan:
			return
		default:
			_, msgBytes, err := sb.wsConn.ReadMessage()
			if err != nil {
				// Connection closed or error occurred
				return
			}

			var msg WSMessage
			if err := json.Unmarshal(msgBytes, &msg); err != nil {
				continue
			}

			messageChan <- msg
		}
	}
}

// SendMessage sends a JSON message over the WebSocket.
func (sb *SessionBackend) SendMessage(msg WSMessage) error {
	if sb.wsConn == nil {
		return fmt.Errorf("not connected")
	}

	bytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return sb.wsConn.WriteMessage(websocket.TextMessage, bytes)
}

// Close gracefully closes the WebSocket connection.
func (sb *SessionBackend) Close() {
	select {
	case <-sb.stopChan:
		// already closed
	default:
		close(sb.stopChan)
	}

	if sb.wsConn != nil {
		sb.wsConn.Close()
		sb.wsConn = nil
	}
}

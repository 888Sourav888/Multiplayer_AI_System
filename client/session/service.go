package session

import (
	"fmt"
	"strings"
)

// SessionService manages the business logic for the active session room.
type SessionService struct {
	backend *SessionBackend
}

// NewSessionService creates a new SessionService.
func NewSessionService(backend *SessionBackend) *SessionService {
	return &SessionService{
		backend: backend,
	}
}

// GetSessionInfo fetches the session metadata.
func (ss *SessionService) GetSessionInfo(sessionID string) (*Session, error) {
	return ss.backend.GetSessionByID(sessionID)
}

// ConnectSession initiates the WebSocket channel.
func (ss *SessionService) ConnectSession(sessionID string) (<-chan WSMessage, error) {
	return ss.backend.ConnectAndSubscribe(sessionID)
}

// FormatSessionInfo returns a formatted metadata string for the active session.
func (ss *SessionService) FormatSessionInfo(s *Session) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("  Session Name: %s\n", s.Name))
	builder.WriteString(fmt.Sprintf("  Session ID:   %s\n", s.ID))
	builder.WriteString(fmt.Sprintf("  Owner ID:     %s\n", s.OwnerID))
	builder.WriteString(fmt.Sprintf("  Version:      v%d\n", s.CurrentVersion))
	builder.WriteString(fmt.Sprintf("  Status:       %s\n", s.Status))
	builder.WriteString(fmt.Sprintf("  Storage Path: %s\n", s.ProjectStoragePath))
	builder.WriteString(fmt.Sprintf("  Last Active:  %s", s.LastActiveAt))
	return builder.String()
}

// SendSimulatedPatch formats and sends a patch message via WebSocket.
func (ss *SessionService) SendSimulatedPatch(sessionID string, userID string, filePath string, content string) error {
	// Build a simulated patch object matching the expected backend DTO structure
	patchItem := map[string]interface{}{
		"filePath":      filePath,
		"patchContent":  content,
		"status":        "PENDING",
	}

	payload := WSMessage{
		Type:      "PATCH_TRANSFER",
		SessionID: sessionID,
		SenderID:  userID,
		Message:   fmt.Sprintf("Sent patch for %s", filePath),
		Patches:   []interface{}{patchItem},
	}

	return ss.backend.SendMessage(payload)
}

// Close disconnects the live session.
func (ss *SessionService) Close() {
	ss.backend.Close()
}

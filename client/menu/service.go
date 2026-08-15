package menu

import (
	"fmt"
	"strings"
)

// MenuService coordinates business logic between the UI and backend calls.
type MenuService struct {
	apiClient *APIClient
}

// NewMenuService creates a new MenuService.
func NewMenuService(apiClient *APIClient) *MenuService {
	return &MenuService{
		apiClient: apiClient,
	}
}

// GetUserSessions retrieves the sessions owned by a user from the backend.
func (s *MenuService) GetUserSessions(userID string) ([]Session, error) {
	return s.apiClient.FetchSessionsRequest(userID)
}

// FormatSessionList formats a slice of sessions into a readable numbered list.
func (s *MenuService) FormatSessionList(sessions []Session) string {
	if len(sessions) == 0 {
		return "No sessions found."
	}

	var builder strings.Builder
	for i, session := range sessions {
		builder.WriteString(fmt.Sprintf("%d. Name:        %s\n", i+1, session.Name))
		builder.WriteString(fmt.Sprintf("   Session ID:  %s\n", session.ID))
		builder.WriteString(fmt.Sprintf("   Status:      %s\n", session.Status))
		builder.WriteString(fmt.Sprintf("   Version:     v%d\n", session.CurrentVersion))
		if session.GitRepoUrl != "" {
			builder.WriteString(fmt.Sprintf("   Git Repo:    %s [%s @ %s]\n", session.GitRepoUrl, session.GitBranch, session.GitCommitSha[:8]))
		}
		builder.WriteString(fmt.Sprintf("   Last Active: %s\n\n", session.LastActiveAt))
	}
	return strings.TrimSuffix(builder.String(), "\n")
}

// FormatSessionDetail formats a single session into a readable summary.
func (s *MenuService) FormatSessionDetail(session *Session) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("  Name:        %s\n", session.Name))
	builder.WriteString(fmt.Sprintf("  Session ID:  %s\n", session.ID))
	builder.WriteString(fmt.Sprintf("  Status:      %s\n", session.Status))
	builder.WriteString(fmt.Sprintf("  Version:     v%d\n", session.CurrentVersion))
	if session.GitRepoUrl != "" {
		builder.WriteString(fmt.Sprintf("  Git Repo:    %s\n", session.GitRepoUrl))
		builder.WriteString(fmt.Sprintf("  Git Branch:  %s\n", session.GitBranch))
		builder.WriteString(fmt.Sprintf("  Git Commit:  %s\n", session.GitCommitSha))
	}
	builder.WriteString(fmt.Sprintf("  Last Active: %s", session.LastActiveAt))
	return builder.String()
}

// RenameSession updates the name of a session by ID.
func (s *MenuService) RenameSession(sessionID string, newName string) (*Session, error) {
	payload := UpdateSessionPayload{
		Name: &newName,
	}
	return s.apiClient.UpdateSessionByID(sessionID, payload)
}

// ChangeSessionStatus updates the status of a session by ID.
func (s *MenuService) ChangeSessionStatus(sessionID string, newStatus string) (*Session, error) {
	payload := UpdateSessionPayload{
		Status: &newStatus,
	}
	return s.apiClient.UpdateSessionByID(sessionID, payload)
}

// DeleteSession fully removes a session by ID.
func (s *MenuService) DeleteSession(sessionID string) error {
	return s.apiClient.DeleteSessionByID(sessionID)
}

// CreateSession creates a new session, optionally attaching Git metadata.
func (s *MenuService) CreateSession(userID string, sessionName string, gitRepo string, gitBranch string, gitCommitSha string) (*Session, error) {
	payload := CreateSessionPayload{
		Name:         sessionName,
		OwnerID:      userID,
		GitRepoUrl:   gitRepo,
		GitBranch:    gitBranch,
		GitCommitSha: gitCommitSha,
	}
	return s.apiClient.CreateSession(payload)
}

// JoinSession joins the specified user to a session.
func (s *MenuService) JoinSession(userID string, sessionID string) (*Session, error) {
	return s.apiClient.JoinSession(sessionID, userID)
}

// UploadSnapshot compresses and uploads the local workspace code to the backend.
func (s *MenuService) UploadSnapshot(sessionID string, zipBytes []byte) (*SnapshotResponse, error) {
	return s.apiClient.UploadSnapshot(sessionID, zipBytes)
}

// UpdateSessionGitInfo updates a session's git synchronization metadata.
func (s *MenuService) UpdateSessionGitInfo(sessionID string, gitRepo string, gitBranch string, gitCommitSha string) (*Session, error) {
	payload := UpdateSessionPayload{
		GitRepoUrl:   &gitRepo,
		GitBranch:    &gitBranch,
		GitCommitSha: &gitCommitSha,
	}
	return s.apiClient.UpdateSessionByID(sessionID, payload)
}

// GetBaseURL returns the configured backend URL.
func (s *MenuService) GetBaseURL() string {
	return s.apiClient.BaseURL
}

// DownloadSnapshot downloads the zipped workspace bytes of a specific session version.
func (s *MenuService) DownloadSnapshot(sessionID string, version int) ([]byte, error) {
	return s.apiClient.DownloadSnapshot(sessionID, version)
}

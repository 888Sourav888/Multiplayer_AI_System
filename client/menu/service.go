package menu

import (
	"fmt"
	"strings"

	"multiplayer_ai_client/ui"
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

// FormatSessionList formats a slice of sessions as rich bordered cards.
func (s *MenuService) FormatSessionList(sessions []Session) string {
	if len(sessions) == 0 {
		return ui.BrightBlack + "  No sessions found." + ui.Reset + "\n"
	}

	var b strings.Builder
	for i, sess := range sessions {
		ver := fmt.Sprintf("%d", sess.CurrentVersion)
		gitCommit := sess.GitCommitSha
		if len(gitCommit) > 8 {
			gitCommit = gitCommit[:8]
		}
		b.WriteString(ui.SessionCard(
			i+1,
			sess.Name,
			sess.ID,
			sess.Status,
			ver,
			sess.LastActiveAt,
			sess.GitRepoUrl,
			sess.GitBranch,
			gitCommit,
		))
	}
	return b.String()
}

// FormatSessionDetail formats a single session as a rich bordered detail card.
func (s *MenuService) FormatSessionDetail(sess *Session) string {
	ver := fmt.Sprintf("%d", sess.CurrentVersion)
	return ui.SessionDetailCard(
		sess.Name,
		sess.ID,
		sess.Status,
		ver,
		"", // ownerID not shown in menu detail (shown in session room)
		"",
		sess.LastActiveAt,
		sess.GitRepoUrl,
		sess.GitBranch,
		sess.GitCommitSha,
	)
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

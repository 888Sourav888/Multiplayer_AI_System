package menu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// Session represents a multiplayer AI session entity returned by the backend.
type Session struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	OwnerID            string `json:"ownerId"`
	ProjectStoragePath string `json:"projectStoragePath"`
	CurrentVersion     int    `json:"currentVersion"`
	Status             string `json:"status"`
	GitRepoUrl         string `json:"gitRepoUrl"`
	GitBranch          string `json:"gitBranch"`
	GitCommitSha       string `json:"gitCommitSha"`
	CreatedAt          string `json:"createdAt"`
	LastActiveAt       string `json:"lastActiveAt"`
}

// UpdateSessionPayload is the request body for PATCH /api/sessions/{sessionId}.
// Pointer fields allow partial updates — nil fields are omitted from JSON.
type UpdateSessionPayload struct {
	Name         *string `json:"name,omitempty"`
	Status       *string `json:"status,omitempty"`
	GitRepoUrl   *string `json:"gitRepoUrl,omitempty"`
	GitBranch    *string `json:"gitBranch,omitempty"`
	GitCommitSha *string `json:"gitCommitSha,omitempty"`
}

// APIClient handles communication with the backend services.
type APIClient struct {
	BaseURL string
}

// NewAPIClient creates and returns a new APIClient instance.
func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		BaseURL: baseURL,
	}
}

// FetchSessionsRequest makes a GET request to retrieve sessions owned by the user.
func (c *APIClient) FetchSessionsRequest(userID string) ([]Session, error) {
	url := fmt.Sprintf("%s/api/sessions?ownerId=%s", c.BaseURL, userID)

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

	var sessions []Session
	if err := json.Unmarshal(body, &sessions); err != nil {
		return nil, fmt.Errorf("failed to parse sessions JSON: %w (raw: %s)", err, string(body))
	}

	return sessions, nil
}

// UpdateSessionByID sends a PATCH request to update a session's fields.
func (c *APIClient) UpdateSessionByID(sessionID string, payload UpdateSessionPayload) (*Session, error) {
	url := fmt.Sprintf("%s/api/sessions/%s", c.BaseURL, sessionID)

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal update payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create PATCH request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
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

	var updated Session
	if err := json.Unmarshal(body, &updated); err != nil {
		return nil, fmt.Errorf("failed to parse update response: %w (raw: %s)", err, string(body))
	}

	return &updated, nil
}

// DeleteSessionByID sends a DELETE request to fully remove a session.
func (c *APIClient) DeleteSessionByID(sessionID string) error {
	url := fmt.Sprintf("%s/api/sessions/%s", c.BaseURL, sessionID)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create DELETE request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backend returned non-OK status: %s (body: %s)", resp.Status, string(body))
	}

	return nil
}

// CreateSessionPayload is the request body for POST /api/sessions.
type CreateSessionPayload struct {
	Name         string `json:"name"`
	OwnerID      string `json:"ownerId"`
	GitRepoUrl   string `json:"gitRepoUrl"`
	GitBranch    string `json:"gitBranch"`
	GitCommitSha string `json:"gitCommitSha"`
}

// CreateSession sends a POST request to create a new session.
func (c *APIClient) CreateSession(payload CreateSessionPayload) (*Session, error) {
	url := fmt.Sprintf("%s/api/sessions", c.BaseURL)

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal create payload: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend returned non-OK status: %s (body: %s)", resp.Status, string(body))
	}

	var created Session
	if err := json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("failed to parse create response: %w (raw: %s)", err, string(body))
	}

	return &created, nil
}

// JoinSession sends a POST request to join a user to a session.
func (c *APIClient) JoinSession(sessionID string, userID string) (*Session, error) {
	url := fmt.Sprintf("%s/api/sessions/%s/join?userId=%s", c.BaseURL, sessionID, userID)

	resp, err := http.Post(url, "application/json", nil)
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
		return nil, fmt.Errorf("failed to parse join response: %w (raw: %s)", err, string(body))
	}

	return &session, nil
}

// SnapshotResponse represents the metadata of an uploaded snapshot.
type SnapshotResponse struct {
	ID              string `json:"id"`
	SessionID       string `json:"sessionId"`
	Version         int    `json:"version"`
	StorageLocation string `json:"storageLocation"`
	CreatedAt       string `json:"createdAt"`
}

// UploadSnapshot sends a multipart POST request with the zipped directory content.
func (c *APIClient) UploadSnapshot(sessionID string, zipBytes []byte) (*SnapshotResponse, error) {
	url := fmt.Sprintf("%s/api/sessions/%s/persist", c.BaseURL, sessionID)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "snapshot.zip")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	_, err = part.Write(zipBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to write zip bytes: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend returned non-OK status: %s (body: %s)", resp.Status, string(respBody))
	}

	var snapshot SnapshotResponse
	if err := json.Unmarshal(respBody, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot response: %w (raw: %s)", err, string(respBody))
	}

	return &snapshot, nil
}


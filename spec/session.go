package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GetSessionPath returns the session file path for a given working directory
// Sanitizes the path similar to Claude Code's approach
func GetSessionPath(cwd string) (string, error) {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Sanitize the path for use as a directory name
	// Replace / with - and remove any problematic characters
	sanitized := strings.ReplaceAll(cwd, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "\\", "-")
	sanitized = strings.ReplaceAll(sanitized, ":", "")
	sanitized = strings.TrimPrefix(sanitized, "-") // Remove leading dash

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	sessionDir := filepath.Join(homeDir, ".rapid", "projects", sanitized)
	return filepath.Join(sessionDir, "session.json"), nil
}

// SaveSession saves a session to disk
func SaveSession(session *Session, cwd string) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}

	sessionPath, err := GetSessionPath(cwd)
	if err != nil {
		return err
	}

	// Ensure directory exists
	sessionDir := filepath.Dir(sessionPath)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Update timestamp
	session.UpdatedAt = time.Now()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Write to file
	if err := os.WriteFile(sessionPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// LoadSession loads a session from disk
// Returns nil if session doesn't exist (not an error)
func LoadSession(cwd string) (*Session, error) {
	sessionPath, err := GetSessionPath(cwd)
	if err != nil {
		return nil, err
	}

	// Check if file exists
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return nil, nil // No session exists, not an error
	}

	// Read file
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	// Unmarshal JSON
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// DeleteSession removes a session file
func DeleteSession(cwd string) error {
	sessionPath, err := GetSessionPath(cwd)
	if err != nil {
		return err
	}

	if err := os.Remove(sessionPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

package client

import (
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
)

// SessionManager manages multiple WhatsApp sessions
type SessionManager struct {
	sessions map[string]*WAClient
	mu       sync.RWMutex
	logger   *zap.SugaredLogger
	dataDir  string
}

// NewSessionManager creates a new session manager
func NewSessionManager(logger *zap.SugaredLogger) *SessionManager {
	dataDir := os.Getenv("SESSION_DIR")
	if dataDir == "" {
		dataDir = "./sessions"
	}

	// Create sessions directory if not exists
	os.MkdirAll(dataDir, 0755)

	return &SessionManager{
		sessions: make(map[string]*WAClient),
		logger:   logger,
		dataDir:  dataDir,
	}
}

// CreateSession creates a new WhatsApp session
func (sm *SessionManager) CreateSession(sessionID string) (*WAClient, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if session already exists
	if _, exists := sm.sessions[sessionID]; exists {
		return nil, ErrSessionExists
	}

	// Create new client
	client := NewWAClient(sessionID, sm.logger, sm.dataDir)
	sm.sessions[sessionID] = client

	// Start connection in background
	go func() {
		if err := client.Connect(); err != nil {
			sm.logger.Errorf("Failed to connect session %s: %v", sessionID, err)
		}
	}()

	return client, nil
}

// GetSession returns a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*WAClient, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	client, exists := sm.sessions[sessionID]
	return client, exists
}

// DeleteSession removes and disconnects a session
func (sm *SessionManager) DeleteSession(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	client, exists := sm.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	// Disconnect and remove
	client.Disconnect()
	delete(sm.sessions, sessionID)

	// Remove session data from disk
	sessionPath := filepath.Join(sm.dataDir, sessionID)
	os.RemoveAll(sessionPath)

	return nil
}

// GetAllSessions returns all active sessions
func (sm *SessionManager) GetAllSessions() []*WAClient {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*WAClient, 0, len(sm.sessions))
	for _, client := range sm.sessions {
		sessions = append(sessions, client)
	}
	return sessions
}

// GetStats returns session statistics
func (sm *SessionManager) GetStats() SessionStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := SessionStats{
		Total: len(sm.sessions),
	}

	for _, client := range sm.sessions {
		switch client.GetStatus() {
		case StatusReady:
			stats.Ready++
			stats.Active++
		case StatusConnecting, StatusQRReady:
			stats.Initializing++
		case StatusDisconnected:
			// Not counted as active
		}
	}

	return stats
}

// LoadPersistedSessions loads sessions from disk
func (sm *SessionManager) LoadPersistedSessions() error {
	entries, err := os.ReadDir(sm.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		credsPath := filepath.Join(sm.dataDir, sessionID, "creds.json")

		// Only load sessions with credentials
		if _, err := os.Stat(credsPath); err == nil {
			sm.logger.Infof("Loading persisted session: %s", sessionID)
			sm.CreateSession(sessionID)
		}
	}

	return nil
}

// DisconnectAll disconnects all sessions
func (sm *SessionManager) DisconnectAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, client := range sm.sessions {
		client.Disconnect()
	}
}

// SessionStats holds session statistics
type SessionStats struct {
	Total        int `json:"total"`
	Active       int `json:"active"`
	Ready        int `json:"ready"`
	Initializing int `json:"initializing"`
}

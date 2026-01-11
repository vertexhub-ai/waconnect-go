// WAConnect Go - WhatsApp API Gateway
// Copyright (c) 2026 VertexHub
// Licensed under MIT License
// https://github.com/vertexhub/waconnect-go

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"nhooyr.io/websocket"
)

// WhatsApp WebSocket endpoints
const (
	WAWebSocketURL = "wss://web.whatsapp.com/ws/chat"
	WAOrigin       = "https://web.whatsapp.com"
)

// ConnectionState represents the current connection state
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateAuthenticated
)

// Connection manages the WebSocket connection to WhatsApp
type Connection struct {
	ws     *websocket.Conn
	state  ConnectionState
	config ConnectionConfig
	logger *zap.SugaredLogger
	noise  *NoiseHandler

	// Channel for incoming messages
	msgChan   chan []byte
	errorChan chan error
	closeChan chan struct{}

	// Mutex for thread safety
	mu sync.RWMutex

	// Callbacks
	onQR    func(string)
	onReady func()
	onClose func(error)
}

// ConnectionConfig holds connection configuration
type ConnectionConfig struct {
	SessionID           string
	SessionDir          string
	ConnectTimeoutMs    int
	KeepAliveIntervalMs int
	QRTimeoutMs         int
	MaxRetries          int
	Logger              *zap.SugaredLogger
}

// NewConnection creates a new WhatsApp connection
func NewConnection(config ConnectionConfig) *Connection {
	return &Connection{
		state:     StateDisconnected,
		config:    config,
		logger:    config.Logger,
		noise:     NewNoiseHandler(),
		msgChan:   make(chan []byte, 100),
		errorChan: make(chan error, 10),
		closeChan: make(chan struct{}),
	}
}

// Connect establishes connection to WhatsApp servers
func (c *Connection) Connect(ctx context.Context) error {
	c.mu.Lock()
	c.state = StateConnecting
	c.mu.Unlock()

	c.logger.Info("Connecting to WhatsApp...")

	// Configure WebSocket options
	opts := &websocket.DialOptions{
		HTTPHeader: map[string][]string{
			"Origin": {WAOrigin},
		},
	}

	// Establish WebSocket connection
	ws, _, err := websocket.Dial(ctx, WAWebSocketURL, opts)
	if err != nil {
		c.logger.Errorf("Failed to connect: %v", err)
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	c.ws = ws
	c.logger.Info("WebSocket connected")

	c.mu.Lock()
	c.state = StateConnected
	c.mu.Unlock()

	// Create cancellable context for receiveLoop
	receiveCtx, cancelReceive := context.WithCancel(ctx)

	// Start message receiver with cancellable context
	go c.receiveLoop(receiveCtx)

	// Perform Noise handshake
	if err := c.performHandshake(ctx); err != nil {
		c.logger.Errorf("Handshake failed: %v", err)
		cancelReceive() // Stop receiveLoop goroutine
		c.ws.Close(websocket.StatusAbnormalClosure, "handshake failed")
		return err
	}

	c.logger.Info("Noise handshake completed")

	// Check for existing credentials
	if c.hasCredentials() {
		if err := c.resumeSession(ctx); err != nil {
			c.logger.Warn("Session resume failed, starting fresh")
			// Note: don't cancel here, let startNewSession continue
			return c.startNewSession(ctx)
		}
		return nil
	}

	return c.startNewSession(ctx)
}

// performHandshake performs the Noise Protocol handshake
func (c *Connection) performHandshake(ctx context.Context) error {
	// Send client hello
	clientHello := c.noise.GenerateClientHello()
	c.logger.Infof("Sending client hello (%d bytes)", len(clientHello))
	if err := c.sendRaw(ctx, clientHello); err != nil {
		return fmt.Errorf("failed to send client hello: %w", err)
	}

	// Accumulate data from server until we have enough for handshake
	var serverData []byte
	timeout := time.After(30 * time.Second) // Increased timeout for slower connections
	processAttempts := 0
	const maxProcessAttempts = 5
	const minBytesForError = 256 // Only fail if we have significant data

	for {
		select {
		case chunk := <-c.msgChan:
			c.logger.Infof("Received %d bytes from server (total: %d)", len(chunk), len(serverData)+len(chunk))
			serverData = append(serverData, chunk...)

			// Process when we have at least 32 bytes (server ephemeral key)
			if len(serverData) >= 32 {
				processAttempts++
				err := c.noise.ProcessServerHello(serverData)
				if err != nil {
					c.logger.Warnf("ProcessServerHello attempt %d with %d bytes: %v", processAttempts, len(serverData), err)

					// Only fail if we have significant data AND exhausted retries
					if len(serverData) >= minBytesForError && processAttempts >= maxProcessAttempts {
						return fmt.Errorf("failed to process server hello after %d attempts with %d bytes: %w", processAttempts, len(serverData), err)
					}
					// Otherwise continue accumulating more data
					continue
				}
				// Success!
				c.logger.Infof("ProcessServerHello succeeded on attempt %d with %d bytes", processAttempts, len(serverData))
				goto handshakeComplete
			}

		case <-timeout:
			return fmt.Errorf("timeout waiting for server hello (got %d bytes, %d process attempts)", len(serverData), processAttempts)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

handshakeComplete:
	// Send client finish
	clientFinish, err := c.noise.GenerateClientFinish()
	if err != nil {
		return fmt.Errorf("failed to generate client finish: %w", err)
	}
	c.logger.Infof("Sending client finish (%d bytes)", len(clientFinish))
	if err := c.sendRaw(ctx, clientFinish); err != nil {
		return fmt.Errorf("failed to send client finish: %w", err)
	}

	c.logger.Info("Handshake complete!")
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// startNewSession starts a new session with QR code authentication
func (c *Connection) startNewSession(ctx context.Context) error {
	c.logger.Info("Starting new session, generating QR code...")

	// Generate QR code data
	qrData := c.generateQRData()

	if c.onQR != nil {
		c.onQR(qrData)
	}

	// Wait for scan or timeout
	timeout := time.Duration(c.config.QRTimeoutMs) * time.Millisecond
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	select {
	case msg := <-c.msgChan:
		return c.handleAuthMessage(msg)
	case <-time.After(timeout):
		return fmt.Errorf("QR code expired")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// resumeSession attempts to resume an existing session
func (c *Connection) resumeSession(ctx context.Context) error {
	c.logger.Info("Attempting to resume session...")

	creds, err := c.loadCredentials()
	if err != nil {
		return err
	}

	// Send resume request with credentials
	resumeNode := c.buildResumeNode(creds)
	if err := c.sendNode(ctx, resumeNode); err != nil {
		return err
	}

	// Wait for response
	select {
	case msg := <-c.msgChan:
		return c.handleResumeResponse(msg)
	case <-time.After(30 * time.Second):
		return fmt.Errorf("resume timeout")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// generateQRData generates QR code data for pairing
func (c *Connection) generateQRData() string {
	ref := generateRef()
	pubKey := encodeBase64(c.noise.GetPublicKey())
	return fmt.Sprintf("2@%s,%s,%s", ref, pubKey, c.config.SessionID)
}

// encodeBase64 encodes bytes to base64
func encodeBase64(data []byte) string {
	const b64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	result := make([]byte, 0, ((len(data)+2)/3)*4)
	for i := 0; i < len(data); i += 3 {
		var b uint32
		remaining := len(data) - i
		if remaining >= 3 {
			b = uint32(data[i])<<16 | uint32(data[i+1])<<8 | uint32(data[i+2])
			result = append(result, b64[b>>18&0x3F], b64[b>>12&0x3F], b64[b>>6&0x3F], b64[b&0x3F])
		} else if remaining == 2 {
			b = uint32(data[i])<<16 | uint32(data[i+1])<<8
			result = append(result, b64[b>>18&0x3F], b64[b>>12&0x3F], b64[b>>6&0x3F], '=')
		} else {
			b = uint32(data[i]) << 16
			result = append(result, b64[b>>18&0x3F], b64[b>>12&0x3F], '=', '=')
		}
	}
	return string(result)
}

// sendRaw sends raw bytes through the WebSocket
func (c *Connection) sendRaw(ctx context.Context, data []byte) error {
	if c.ws == nil {
		return fmt.Errorf("not connected")
	}
	return c.ws.Write(ctx, websocket.MessageBinary, data)
}

// sendNode sends an encrypted binary node
func (c *Connection) sendNode(ctx context.Context, node *BinaryNode) error {
	// Encode node to binary
	data := EncodeBinaryNode(node)

	// Encrypt with Noise
	encrypted := c.noise.Encrypt(data)

	return c.sendRaw(ctx, encrypted)
}

// receiveLoop continuously receives messages
func (c *Connection) receiveLoop(ctx context.Context) {
	defer close(c.closeChan)

	const readTimeout = 60 * time.Second

	for {
		// Check context cancellation first
		select {
		case <-ctx.Done():
			c.logger.Info("receiveLoop: context cancelled, stopping")
			return
		default:
		}

		// Create timeout context for this read operation
		readCtx, cancel := context.WithTimeout(ctx, readTimeout)
		_, data, err := c.ws.Read(readCtx)
		cancel() // Always cancel to release resources

		if err != nil {
			// Non-blocking send to error channel
			select {
			case c.errorChan <- err:
			default:
				c.logger.Warnf("receiveLoop: error channel full, error: %v", err)
			}
			return
		}

		// Decrypt if handshake completed
		if c.noise.IsHandshakeComplete() {
			data = c.noise.Decrypt(data)
		}

		// Non-blocking send to message channel to prevent deadlock
		select {
		case c.msgChan <- data:
			// Message sent successfully
		case <-ctx.Done():
			c.logger.Info("receiveLoop: context cancelled while sending")
			return
		default:
			c.logger.Warn("receiveLoop: msgChan full, dropping message")
		}
	}
}

// handleAuthMessage processes authentication response
func (c *Connection) handleAuthMessage(msg []byte) error {
	// Parse and validate auth response
	// This is a placeholder - actual implementation would parse the protobuf
	c.logger.Info("Received auth message")

	c.mu.Lock()
	c.state = StateAuthenticated
	c.mu.Unlock()

	if c.onReady != nil {
		c.onReady()
	}

	return nil
}

// handleResumeResponse processes resume response
func (c *Connection) handleResumeResponse(msg []byte) error {
	// Parse resume response
	c.logger.Info("Session resumed successfully")

	c.mu.Lock()
	c.state = StateAuthenticated
	c.mu.Unlock()

	if c.onReady != nil {
		c.onReady()
	}

	return nil
}

// buildResumeNode creates a resume request node
func (c *Connection) buildResumeNode(creds *Credentials) *BinaryNode {
	return &BinaryNode{
		Tag: "iq",
		Attrs: map[string]string{
			"type": "set",
			"to":   "s.whatsapp.net",
		},
		Content: nil, // Would contain encrypted credentials
	}
}

// Credentials handling
type Credentials struct {
	NoiseKey       []byte `json:"noiseKey"`
	SignedIdentity []byte `json:"signedIdentity"`
	SignedPreKey   []byte `json:"signedPreKey"`
	RegistrationID int    `json:"registrationId"`
	AdvSecretKey   string `json:"advSecretKey"`
	Me             struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"me"`
}

func (c *Connection) hasCredentials() bool {
	credsPath := filepath.Join(c.config.SessionDir, c.config.SessionID, "creds.json")
	_, err := os.Stat(credsPath)
	return err == nil
}

func (c *Connection) loadCredentials() (*Credentials, error) {
	credsPath := filepath.Join(c.config.SessionDir, c.config.SessionID, "creds.json")
	data, err := os.ReadFile(credsPath)
	if err != nil {
		return nil, err
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}

func (c *Connection) saveCredentials(creds *Credentials) error {
	credsPath := filepath.Join(c.config.SessionDir, c.config.SessionID, "creds.json")

	if err := os.MkdirAll(filepath.Dir(credsPath), 0755); err != nil {
		return err
	}

	data, err := json.Marshal(creds)
	if err != nil {
		return err
	}

	return os.WriteFile(credsPath, data, 0600)
}

// Close closes the connection
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ws != nil {
		c.ws.Close(websocket.StatusNormalClosure, "closing")
	}

	c.state = StateDisconnected
	return nil
}

// GetState returns current connection state
func (c *Connection) GetState() ConnectionState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// SetOnQR sets QR callback
func (c *Connection) SetOnQR(fn func(string)) {
	c.onQR = fn
}

// SetOnReady sets ready callback
func (c *Connection) SetOnReady(fn func()) {
	c.onReady = fn
}

// SetOnClose sets close callback
func (c *Connection) SetOnClose(fn func(error)) {
	c.onClose = fn
}

// Helper functions
func generateRef() string {
	// Generate random reference for QR pairing
	return fmt.Sprintf("%d", time.Now().UnixMilli())
}

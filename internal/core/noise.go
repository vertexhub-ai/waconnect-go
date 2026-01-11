// WAConnect Go - WhatsApp API Gateway
// Copyright (c) 2026 VertexHub
// Licensed under MIT License
// https://github.com/vertexhub/waconnect-go

package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// Noise Protocol constants
const (
	NoiseMode   = "Noise_XX_25519_AESGCM_SHA256\x00\x00\x00\x00"
	NoiseHeader = "WA\x06\x03" // WA + version 6 + dict version 3
)

// NoiseHandler implements Noise Protocol XX handshake for WhatsApp
type NoiseHandler struct {
	// Key pairs
	ephemeralPrivate []byte
	ephemeralPublic  []byte
	staticPrivate    []byte
	staticPublic     []byte
	serverEphemeral  []byte // Stored for DH in ClientFinish

	// Cryptographic state
	hash         []byte
	salt         []byte
	encKey       []byte
	decKey       []byte
	readCounter  uint32
	writeCounter uint32
	isFinished   bool

	// Frame buffer for decoding
	frameBuffer []byte

	mu sync.Mutex
}

// NewNoiseHandler creates a new Noise Protocol handler
func NewNoiseHandler() *NoiseHandler {
	n := &NoiseHandler{
		ephemeralPrivate: make([]byte, 32),
		ephemeralPublic:  make([]byte, 32),
		staticPrivate:    make([]byte, 32),
		staticPublic:     make([]byte, 32),
		frameBuffer:      make([]byte, 0),
	}

	// Generate ephemeral key pair
	rand.Read(n.ephemeralPrivate)
	curve25519.ScalarBaseMult((*[32]byte)(n.ephemeralPublic), (*[32]byte)(n.ephemeralPrivate))

	// Generate static (noise) key pair
	rand.Read(n.staticPrivate)
	curve25519.ScalarBaseMult((*[32]byte)(n.staticPublic), (*[32]byte)(n.staticPrivate))

	// Initialize hash with Noise mode
	n.initializeState()

	return n
}

// initializeState initializes the Noise protocol state
func (n *NoiseHandler) initializeState() {
	modeBytes := []byte(NoiseMode)
	if len(modeBytes) == 32 {
		n.hash = modeBytes
	} else {
		h := sha256.Sum256(modeBytes)
		n.hash = h[:]
	}
	n.salt = n.hash
	n.encKey = n.hash
	n.decKey = n.hash
	n.readCounter = 0
	n.writeCounter = 0
	n.isFinished = false

	// Authenticate with header and public key
	n.authenticate([]byte(NoiseHeader))
	n.authenticate(n.ephemeralPublic)
}

// authenticate mixes data into the hash
func (n *NoiseHandler) authenticate(data []byte) {
	if !n.isFinished {
		h := sha256.New()
		h.Write(n.hash)
		h.Write(data)
		n.hash = h.Sum(nil)
	}
}

// generateIV creates an IV from counter
func (n *NoiseHandler) generateIV(counter uint32) []byte {
	iv := make([]byte, 12)
	binary.BigEndian.PutUint32(iv[8:], counter)
	return iv
}

// encrypt encrypts plaintext using AES-GCM
func (n *NoiseHandler) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(n.encKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	iv := n.generateIV(n.writeCounter)
	ciphertext := gcm.Seal(nil, iv, plaintext, n.hash)

	n.writeCounter++
	n.authenticate(ciphertext)

	return ciphertext, nil
}

// decrypt decrypts ciphertext using AES-GCM
func (n *NoiseHandler) decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(n.decKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	var iv []byte
	if n.isFinished {
		iv = n.generateIV(n.readCounter)
		n.readCounter++
	} else {
		iv = n.generateIV(n.writeCounter)
		n.writeCounter++
	}

	plaintext, err := gcm.Open(nil, iv, ciphertext, n.hash)
	if err != nil {
		return nil, err
	}

	n.authenticate(ciphertext)
	return plaintext, nil
}

// mixIntoKey derives new keys using HKDF
func (n *NoiseHandler) mixIntoKey(data []byte) error {
	reader := hkdf.New(sha256.New, data, n.salt, nil)

	key := make([]byte, 64)
	if _, err := reader.Read(key); err != nil {
		return err
	}

	n.salt = key[:32]
	n.encKey = key[32:]
	n.decKey = key[32:]
	n.readCounter = 0
	n.writeCounter = 0

	return nil
}

// finishInit completes the initialization after handshake
func (n *NoiseHandler) finishInit() error {
	reader := hkdf.New(sha256.New, nil, n.salt, nil)

	key := make([]byte, 64)
	if _, err := reader.Read(key); err != nil {
		return err
	}

	n.encKey = key[:32]
	n.decKey = key[32:]
	n.hash = make([]byte, 0)
	n.readCounter = 0
	n.writeCounter = 0
	n.isFinished = true

	return nil
}

// dh performs Diffie-Hellman key exchange
func (n *NoiseHandler) dh(privateKey, publicKey []byte) ([]byte, error) {
	if len(privateKey) != 32 || len(publicKey) != 32 {
		return nil, errors.New("invalid key length")
	}

	shared, err := curve25519.X25519(privateKey, publicKey)
	if err != nil {
		return nil, err
	}
	return shared, nil
}

// GenerateClientHello creates the initial handshake frame with Protobuf encoding
func (n *NoiseHandler) GenerateClientHello() []byte {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Encode ephemeral public key in Protobuf HandshakeMessage.ClientHello format
	clientHelloProto := EncodeClientHello(n.ephemeralPublic)

	// Frame format: [header][3-byte length][protobuf data]
	header := []byte(NoiseHeader)
	payloadLen := len(clientHelloProto)

	frame := make([]byte, len(header)+3+payloadLen)
	copy(frame, header)
	frame[len(header)] = byte(payloadLen >> 16)
	binary.BigEndian.PutUint16(frame[len(header)+1:], uint16(payloadLen&0xFFFF))
	copy(frame[len(header)+3:], clientHelloProto)

	return frame
}

// ProcessServerHello processes the server's handshake response (Protobuf encoded)
func (n *NoiseHandler) ProcessServerHello(data []byte) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Minimum response size
	if len(data) < 32 {
		return fmt.Errorf("server hello too short: got %d bytes, need at least 32", len(data))
	}

	// Try to decode as Protobuf ServerHello
	serverHello, err := DecodeServerHello(data)
	if err != nil || len(serverHello.Ephemeral) != 32 {
		// Fallback: treat as raw bytes (first 32 = ephemeral key)
		serverHello = &ServerHelloData{Ephemeral: data[:32]}
		if len(data) > 32 {
			serverHello.Static = data[32:]
		}
	}

	// Store server ephemeral for later use in ClientFinish
	n.serverEphemeral = serverHello.Ephemeral

	// Authenticate server ephemeral
	n.authenticate(serverHello.Ephemeral)

	// Perform DH1: ephemeral-ephemeral
	shared1, err := n.dh(n.ephemeralPrivate, serverHello.Ephemeral)
	if err != nil {
		return fmt.Errorf("DH1 failed: %w", err)
	}
	if err := n.mixIntoKey(shared1); err != nil {
		return fmt.Errorf("mixIntoKey failed: %w", err)
	}

	// If we have encrypted static key from server
	if len(serverHello.Static) >= 48 {
		decryptedStatic, err := n.decrypt(serverHello.Static[:48])
		if err == nil && len(decryptedStatic) == 32 {
			// Perform DH2: our ephemeral with server static
			shared2, err := n.dh(n.ephemeralPrivate, decryptedStatic)
			if err == nil {
				_ = n.mixIntoKey(shared2)
			}
		}
	}

	return nil
}

// GenerateClientFinish creates the client finish message with Protobuf encoding
func (n *NoiseHandler) GenerateClientFinish() ([]byte, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Encrypt our static public key
	encryptedStaticKey, err := n.encrypt(n.staticPublic)
	if err != nil {
		return nil, err
	}

	// Perform DH3: our static key with server ephemeral
	if len(n.serverEphemeral) == 32 {
		shared3, err := n.dh(n.staticPrivate, n.serverEphemeral)
		if err == nil {
			_ = n.mixIntoKey(shared3)
		}
	}

	// Create empty payload (will be populated if needed for credentials)
	var payload []byte

	// Encrypt the payload if needed
	if len(payload) > 0 {
		payload, err = n.encrypt(payload)
		if err != nil {
			return nil, err
		}
	}

	// Encode as Protobuf HandshakeMessage.ClientFinish
	clientFinishProto := EncodeClientFinish(encryptedStaticKey, payload)

	// Finish initialization for transport
	if err := n.finishInit(); err != nil {
		return nil, err
	}

	return clientFinishProto, nil
}

// EncodeFrame encodes data for sending with length prefix
func (n *NoiseHandler) EncodeFrame(data []byte) ([]byte, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	var payload []byte
	if n.isFinished {
		var err error
		payload, err = n.encrypt(data)
		if err != nil {
			return nil, err
		}
	} else {
		payload = data
	}

	// Add 3-byte length prefix
	frame := make([]byte, 3+len(payload))
	frame[0] = byte(len(payload) >> 16)
	binary.BigEndian.PutUint16(frame[1:], uint16(len(payload)&0xFFFF))
	copy(frame[3:], payload)

	return frame, nil
}

// DecodeFrame decodes received data
func (n *NoiseHandler) DecodeFrame(data []byte) ([][]byte, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.frameBuffer = append(n.frameBuffer, data...)

	var frames [][]byte

	for len(n.frameBuffer) >= 3 {
		// Read 3-byte length
		size := int(n.frameBuffer[0])<<16 | int(binary.BigEndian.Uint16(n.frameBuffer[1:3]))

		if len(n.frameBuffer) < 3+size {
			break // Need more data
		}

		frame := n.frameBuffer[3 : 3+size]
		n.frameBuffer = n.frameBuffer[3+size:]

		if n.isFinished {
			decrypted, err := n.decrypt(frame)
			if err != nil {
				return frames, err
			}
			frames = append(frames, decrypted)
		} else {
			frames = append(frames, frame)
		}
	}

	return frames, nil
}

// IsHandshakeComplete returns whether handshake is finished
func (n *NoiseHandler) IsHandshakeComplete() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.isFinished
}

// GetPublicKey returns the ephemeral public key
func (n *NoiseHandler) GetPublicKey() []byte {
	return n.ephemeralPublic
}

// GetStaticPublicKey returns the static public key
func (n *NoiseHandler) GetStaticPublicKey() []byte {
	return n.staticPublic
}

// Encrypt encrypts data for sending (public interface)
func (n *NoiseHandler) Encrypt(data []byte) []byte {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.isFinished {
		return data
	}

	encrypted, err := n.encrypt(data)
	if err != nil {
		return data
	}
	return encrypted
}

// Decrypt decrypts received data (public interface)
func (n *NoiseHandler) Decrypt(data []byte) []byte {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.isFinished {
		return data
	}

	decrypted, err := n.decrypt(data)
	if err != nil {
		return data
	}
	return decrypted
}

// Errors
var ErrInvalidHandshake = &NoiseError{Message: "invalid handshake data"}

type NoiseError struct {
	Message string
}

func (e *NoiseError) Error() string {
	return e.Message
}

// WAConnect Go - WhatsApp API Gateway
// Copyright (c) 2026 VertexHub
// Licensed under MIT License
// https://github.com/vertexhub/waconnect-go

package core

// Manual Protobuf encoder/decoder for HandshakeMessage
// This avoids dependency on protoc-generated code while maintaining compatibility
// with WhatsApp's expected Protobuf format.
//
// HandshakeMessage structure:
//   - ClientHello: field 2
//   - ServerHello: field 3
//   - ClientFinish: field 4

// Wire types
const (
	wireVarint  = 0
	wireFixed64 = 1
	wireBytes   = 2
	wireFixed32 = 5
)

// Field numbers for HandshakeMessage
const (
	fieldClientHello  = 2
	fieldServerHello  = 3
	fieldClientFinish = 4
)

// Field numbers for inner messages
const (
	fieldEphemeral = 1
	fieldStatic    = 2
	fieldPayload   = 3
)

// encodeVarint encodes an unsigned integer as a varint
func encodeVarint(n uint64) []byte {
	if n == 0 {
		return []byte{0}
	}
	var buf []byte
	for n > 0 {
		b := byte(n & 0x7F)
		n >>= 7
		if n > 0 {
			b |= 0x80
		}
		buf = append(buf, b)
	}
	return buf
}

// decodeVarint decodes a varint from data, returns value and bytes consumed
func decodeVarint(data []byte) (uint64, int) {
	var n uint64
	var shift uint
	for i, b := range data {
		n |= uint64(b&0x7F) << shift
		if b < 0x80 {
			return n, i + 1
		}
		shift += 7
		if shift >= 64 {
			return 0, 0 // overflow
		}
	}
	return 0, 0
}

// encodeTag creates a protobuf field tag
func encodeTag(fieldNum int, wireType int) []byte {
	return encodeVarint(uint64(fieldNum<<3 | wireType))
}

// pbEncodeBytes encodes a bytes field with tag
func pbEncodeBytes(fieldNum int, data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	tag := encodeTag(fieldNum, wireBytes)
	length := encodeVarint(uint64(len(data)))
	result := make([]byte, 0, len(tag)+len(length)+len(data))
	result = append(result, tag...)
	result = append(result, length...)
	result = append(result, data...)
	return result
}

// EncodeClientHello creates a HandshakeMessage with ClientHello
// ClientHello contains ephemeral public key (field 1)
func EncodeClientHello(ephemeral []byte) []byte {
	// Build ClientHello inner message
	clientHello := pbEncodeBytes(fieldEphemeral, ephemeral)

	// Wrap in HandshakeMessage (field 2 = ClientHello)
	return pbEncodeBytes(fieldClientHello, clientHello)
}

// EncodeClientFinish creates a HandshakeMessage with ClientFinish
// ClientFinish contains static key (field 1) and payload (field 2)
func EncodeClientFinish(static, payload []byte) []byte {
	// Build ClientFinish inner message
	var clientFinish []byte
	clientFinish = append(clientFinish, pbEncodeBytes(fieldStatic, static)...)
	if len(payload) > 0 {
		clientFinish = append(clientFinish, pbEncodeBytes(fieldPayload, payload)...)
	}

	// Wrap in HandshakeMessage (field 4 = ClientFinish)
	return pbEncodeBytes(fieldClientFinish, clientFinish)
}

// ServerHelloData contains parsed ServerHello fields
type ServerHelloData struct {
	Ephemeral []byte
	Static    []byte
	Payload   []byte
}

// DecodeServerHello extracts fields from a HandshakeMessage containing ServerHello
func DecodeServerHello(data []byte) (*ServerHelloData, error) {
	result := &ServerHelloData{}

	// First, find the ServerHello field (field 3) in HandshakeMessage
	serverHelloBytes, err := findField(data, fieldServerHello)
	if err != nil {
		// Maybe the data IS the ServerHello directly (without HandshakeMessage wrapper)
		// Try parsing as raw ServerHello
		serverHelloBytes = data
	}

	// Parse ServerHello fields
	if ephemeral, err := findField(serverHelloBytes, fieldEphemeral); err == nil {
		result.Ephemeral = ephemeral
	}
	if static, err := findField(serverHelloBytes, fieldStatic); err == nil {
		result.Static = static
	}
	if payload, err := findField(serverHelloBytes, fieldPayload); err == nil {
		result.Payload = payload
	}

	// If no ephemeral found, the data might be raw bytes (32-byte key)
	if len(result.Ephemeral) == 0 && len(data) >= 32 {
		// Fallback: treat first 32 bytes as ephemeral public key
		result.Ephemeral = data[:32]
		if len(data) > 32 {
			result.Static = data[32:]
		}
	}

	return result, nil
}

// findField searches for a specific field number in protobuf data
func findField(data []byte, targetField int) ([]byte, error) {
	pos := 0
	for pos < len(data) {
		// Read tag
		tag, n := decodeVarint(data[pos:])
		if n == 0 {
			break
		}
		pos += n

		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x7)

		switch wireType {
		case wireVarint:
			// Skip varint value
			_, n := decodeVarint(data[pos:])
			if n == 0 {
				return nil, ErrInvalidProtobuf
			}
			pos += n

		case wireFixed64:
			pos += 8

		case wireFixed32:
			pos += 4

		case wireBytes:
			// Read length
			length, n := decodeVarint(data[pos:])
			if n == 0 {
				return nil, ErrInvalidProtobuf
			}
			pos += n

			if pos+int(length) > len(data) {
				return nil, ErrInvalidProtobuf
			}

			if fieldNum == targetField {
				return data[pos : pos+int(length)], nil
			}
			pos += int(length)

		default:
			return nil, ErrInvalidProtobuf
		}
	}

	return nil, ErrFieldNotFound
}

// Protobuf errors
type ProtobufError struct {
	Message string
}

func (e *ProtobufError) Error() string {
	return e.Message
}

var (
	ErrInvalidProtobuf = &ProtobufError{Message: "invalid protobuf data"}
	ErrFieldNotFound   = &ProtobufError{Message: "field not found"}
)

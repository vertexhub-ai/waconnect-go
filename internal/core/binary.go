package core

import (
	"bytes"
	"encoding/binary"
)

// BinaryNode represents a WhatsApp binary protocol node
type BinaryNode struct {
	Tag     string            `json:"tag"`
	Attrs   map[string]string `json:"attrs,omitempty"`
	Content interface{}       `json:"content,omitempty"` // []byte, string, or []*BinaryNode
}

// Dictionary of common tags used in WhatsApp protocol
var tagDictionary = []string{
	"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "",
	"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "",
	"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "",
	"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15",
	"16", "17", "18", "19", "20", "21", "22", "23", "24", "25", "26", "27", "28", "29", "30",
	"account", "ack", "action", "active", "add", "after", "all", "allow", "and", "android",
	"announce", "archive", "available", "battery", "before", "block", "body", "broadcast",
	"call", "call-creator", "call-id", "cancel", "caption", "chat", "child", "clear",
	"code", "composing", "config", "contact", "contacts", "count", "create", "creator",
	"decrypt", "delete", "demote", "description", "device", "devices", "disappearing",
	"done", "download", "edit", "elapsed", "encoding", "encrypt", "end", "ephemeral",
	"error", "event", "exit", "exposure", "failure", "false", "fan_out", "file",
	"filename", "format", "from", "full", "g.us", "get", "gif", "group", "groups",
	"hash", "height", "host", "id", "image", "in", "inactive", "index", "info",
	"interactive", "invite", "ios", "iq", "is", "item", "items", "jid", "keep",
	"key", "keyvalue", "keys", "kind", "large", "last", "leave", "limit",
	"linked", "list", "live", "location", "locked", "md", "media", "media_type",
	"member", "merry", "message", "messages", "meta", "mime", "mirror", "mms",
	"modify", "msg", "mute", "name", "network", "new", "news", "newsletter", "none",
	"not", "notification", "notify", "number", "of", "offline", "opt", "order", "out",
	"owner", "paid", "pairing", "participant", "participants", "paused", "phash",
	"phone", "photo", "picture", "pin", "pinned", "platform", "pn", "preview", "previous",
	"primary", "private", "promote", "props", "protocol", "push", "pushname", "query",
	"quit", "quote", "rate", "read", "reason", "receipt", "received", "recipient", "remove",
	"removed", "reply", "report", "request", "require", "reset", "resource", "result",
	"retry", "revoke", "s.whatsapp.net", "screen", "search", "sec", "secret", "seen",
	"selected", "self", "sender", "serial", "server", "session", "set", "settings",
	"sf", "shake", "share", "short", "side", "sig", "silent", "size", "sky", "slow",
	"smax", "smbiz", "source", "sponsor", "srcjid", "starred", "start", "status",
	"sticky", "storage", "store", "stop", "subject", "subscribe", "success", "sync",
	"system", "t", "tag", "taken", "target", "template", "terminate", "text", "thread",
	"ticket", "time", "timestamp", "to", "token", "true", "type", "unavailable", "undefined",
	"unique", "unknown", "unlock", "unread", "until", "update", "upgrade", "url", "user",
	"users", "v", "value", "version", "video", "voip", "wa", "web", "webp", "width",
	"write", "xmlns", "xmpp", "you", "years",
}

// EncodeBinaryNode encodes a BinaryNode to binary format
func EncodeBinaryNode(node *BinaryNode) []byte {
	buf := new(bytes.Buffer)
	encodeNode(buf, node)
	return buf.Bytes()
}

// DecodeBinaryNode decodes binary data to a BinaryNode
func DecodeBinaryNode(data []byte) (*BinaryNode, error) {
	reader := bytes.NewReader(data)
	return decodeNode(reader)
}

func encodeNode(buf *bytes.Buffer, node *BinaryNode) {
	if node == nil {
		buf.WriteByte(0x00)
		return
	}

	// Encode descriptor
	numAttrs := len(node.Attrs)
	hasContent := node.Content != nil

	descriptor := (numAttrs << 1)
	if hasContent {
		descriptor |= 1
	}

	// Write list header
	buf.WriteByte(byte(descriptor))

	// Encode tag
	encodeString(buf, node.Tag)

	// Encode attributes
	for key, val := range node.Attrs {
		encodeString(buf, key)
		encodeString(buf, val)
	}

	// Encode content
	if hasContent {
		switch content := node.Content.(type) {
		case []byte:
			encodeBytes(buf, content)
		case string:
			encodeString(buf, content)
		case []*BinaryNode:
			buf.WriteByte(byte(len(content)))
			for _, child := range content {
				encodeNode(buf, child)
			}
		}
	}
}

func encodeString(buf *bytes.Buffer, s string) {
	// Check if string is in dictionary
	for i, dictStr := range tagDictionary {
		if dictStr == s && dictStr != "" {
			buf.WriteByte(byte(i))
			return
		}
	}

	// Encode as packed string or raw bytes
	if len(s) < 128 {
		buf.WriteByte(byte(len(s)))
		buf.WriteString(s)
	} else {
		buf.WriteByte(0xFD)
		binary.Write(buf, binary.BigEndian, uint16(len(s)))
		buf.WriteString(s)
	}
}

func encodeBytes(buf *bytes.Buffer, data []byte) {
	if len(data) < 256 {
		buf.WriteByte(byte(len(data)))
	} else {
		buf.WriteByte(0xFE)
		binary.Write(buf, binary.BigEndian, uint32(len(data)))
	}
	buf.Write(data)
}

func decodeNode(reader *bytes.Reader) (*BinaryNode, error) {
	descriptor, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if descriptor == 0x00 {
		return nil, nil
	}

	numAttrs := int(descriptor >> 1)
	hasContent := descriptor&1 == 1

	// Decode tag
	tag, err := decodeString(reader)
	if err != nil {
		return nil, err
	}

	// Decode attributes
	attrs := make(map[string]string)
	for i := 0; i < numAttrs; i++ {
		key, err := decodeString(reader)
		if err != nil {
			return nil, err
		}
		val, err := decodeString(reader)
		if err != nil {
			return nil, err
		}
		attrs[key] = val
	}

	node := &BinaryNode{
		Tag:   tag,
		Attrs: attrs,
	}

	// Decode content
	if hasContent {
		// Try to determine content type from first byte
		contentType, _ := reader.ReadByte()
		reader.UnreadByte()

		if contentType < 128 {
			// Likely a list of child nodes
			count, _ := reader.ReadByte()
			children := make([]*BinaryNode, count)
			for i := range children {
				child, err := decodeNode(reader)
				if err != nil {
					return nil, err
				}
				children[i] = child
			}
			node.Content = children
		} else {
			// Raw bytes
			data, err := decodeBytes(reader)
			if err != nil {
				return nil, err
			}
			node.Content = data
		}
	}

	return node, nil
}

func decodeString(reader *bytes.Reader) (string, error) {
	b, err := reader.ReadByte()
	if err != nil {
		return "", err
	}

	// Dictionary lookup
	if int(b) < len(tagDictionary) && tagDictionary[b] != "" {
		return tagDictionary[b], nil
	}

	// Length-prefixed string
	var length int
	if b == 0xFD {
		var l uint16
		binary.Read(reader, binary.BigEndian, &l)
		length = int(l)
	} else {
		length = int(b)
	}

	buf := make([]byte, length)
	reader.Read(buf)
	return string(buf), nil
}

func decodeBytes(reader *bytes.Reader) ([]byte, error) {
	b, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	var length int
	if b == 0xFE {
		var l uint32
		binary.Read(reader, binary.BigEndian, &l)
		length = int(l)
	} else {
		length = int(b)
	}

	buf := make([]byte, length)
	reader.Read(buf)
	return buf, nil
}

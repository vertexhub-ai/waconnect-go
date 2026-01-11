package core

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"

	qrcode "github.com/skip2/go-qrcode"
)

// QRGenerator generates QR codes for WhatsApp pairing
type QRGenerator struct {
	size int
}

// NewQRGenerator creates a new QR generator
func NewQRGenerator() *QRGenerator {
	return &QRGenerator{
		size: 256,
	}
}

// SetSize sets the QR code size
func (g *QRGenerator) SetSize(size int) {
	g.size = size
}

// GeneratePNG generates a QR code as PNG bytes
func (g *QRGenerator) GeneratePNG(data string) ([]byte, error) {
	qr, err := qrcode.New(data, qrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("failed to create QR code: %w", err)
	}

	var buf bytes.Buffer
	err = png.Encode(&buf, qr.Image(g.size))
	if err != nil {
		return nil, fmt.Errorf("failed to encode PNG: %w", err)
	}

	return buf.Bytes(), nil
}

// GenerateBase64 generates a QR code as base64 PNG
func (g *QRGenerator) GenerateBase64(data string) (string, error) {
	pngBytes, err := g.GeneratePNG(data)
	if err != nil {
		return "", err
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes), nil
}

// GenerateSVG generates a QR code as SVG string
func (g *QRGenerator) GenerateSVG(data string) (string, error) {
	qr, err := qrcode.New(data, qrcode.Medium)
	if err != nil {
		return "", fmt.Errorf("failed to create QR code: %w", err)
	}

	// Generate SVG manually from bitmap
	bitmap := qr.Bitmap()
	size := len(bitmap)
	moduleSize := g.size / size

	var svg bytes.Buffer
	svg.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d">`, g.size, g.size, g.size, g.size))
	svg.WriteString(`<rect width="100%" height="100%" fill="#ffffff"/>`)

	for y, row := range bitmap {
		for x, cell := range row {
			if cell {
				svg.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="#000000"/>`,
					x*moduleSize, y*moduleSize, moduleSize, moduleSize))
			}
		}
	}

	svg.WriteString(`</svg>`)
	return svg.String(), nil
}

// GenerateWhatsAppQR generates QR for WhatsApp pairing
func GenerateWhatsAppQR(ref, publicKey, sessionID string) string {
	// Format: 2@<ref>,<publicKey>,<clientId>
	return fmt.Sprintf("2@%s,%s,%s", ref, publicKey, sessionID)
}

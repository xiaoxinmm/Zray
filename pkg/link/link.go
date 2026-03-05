// Package link implements ZA:// encrypted link generation and parsing.
// v2 Format: ZA://BASE64URL (~40 chars for typical config)
// v1 Format: ZA://BASE26 (legacy, still parseable)
// Encryption: AES-256-GCM
package link

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
)

var DefaultKey = "ZRaySecretKey!!!"

type LinkConfig struct {
	Host       string `json:"h"`
	Port       int    `json:"p"`
	UserHash   string `json:"u"`
	SmartPort  int    `json:"s,omitempty"`
	GlobalPort int    `json:"g,omitempty"`
	TFO        bool   `json:"t,omitempty"`
}

// --- v2 Compact Binary Format ---
// Layout: [ver:1][flags:1][ip:4|16][port:2][hashLen:1][hash:N]
// Typical IPv4: 1+1+4+2+1+16 = 25 bytes plaintext
// AES-GCM: 12 nonce + 25 payload + 16 tag = 53 bytes
// Base64URL(53) = 71 chars... still long
//
// Optimization: use 8-byte nonce (custom GCM), but risky.
// Better: just use compact JSON + Base64URL instead of Base26.
// That alone cuts from ~2000 chars to ~60 chars.

// Generate creates a ZA:// link from config.
func Generate(cfg *LinkConfig, key string) (string, error) {
	if key == "" {
		key = DefaultKey
	}

	// Compact JSON (short keys already)
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}

	encrypted, err := encrypt(data, deriveKey(key))
	if err != nil {
		return "", err
	}

	encoded := base64.RawURLEncoding.EncodeToString(encrypted)
	return "ZA://" + encoded, nil
}

// GenerateBinary creates a minimal binary ZA:// link (IPv4 only, ~45 chars).
func GenerateBinary(cfg *LinkConfig, key string) (string, error) {
	if key == "" {
		key = DefaultKey
	}

	ip := net.ParseIP(cfg.Host)
	if ip == nil {
		// Fall back to JSON mode for hostnames
		return Generate(cfg, key)
	}
	ipv4 := ip.To4()
	if ipv4 == nil {
		return Generate(cfg, key)
	}

	// Binary: [0x02][flags:1][ip:4][port:2][hash UTF-8]
	flags := byte(0)
	if cfg.TFO {
		flags |= 0x01
	}

	buf := []byte{0x02, flags}
	buf = append(buf, ipv4...)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(cfg.Port))
	buf = append(buf, portBytes...)
	buf = append(buf, []byte(cfg.UserHash)...)

	encrypted, err := encrypt(buf, deriveKey(key))
	if err != nil {
		return "", err
	}

	encoded := base64.RawURLEncoding.EncodeToString(encrypted)
	return "ZA://" + encoded, nil
}

// Parse decodes a ZA:// link. Supports v1 (Base26) and v2 (Base64URL).
func Parse(link string, key string) (*LinkConfig, error) {
	if key == "" {
		key = DefaultKey
	}

	upper := strings.ToUpper(link)
	if !strings.HasPrefix(upper, "ZA://") {
		return nil, fmt.Errorf("invalid ZA link: missing ZA:// prefix")
	}
	body := link[5:]

	// v1 detection: all uppercase A-Z and long
	if isAllUpperAlpha(body) && len(body) > 80 {
		return parseLegacyV1(body, key)
	}

	// v2: Base64URL
	encrypted, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return parseLegacyV1(strings.ToUpper(body), key)
	}

	data, err := decrypt(encrypted, deriveKey(key))
	if err != nil {
		return nil, fmt.Errorf("decrypt failed: %w", err)
	}

	// Check if binary format (starts with 0x02)
	if len(data) > 0 && data[0] == 0x02 {
		return decodeBinary(data)
	}

	// JSON format
	var cfg LinkConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}
	if cfg.SmartPort == 0 {
		cfg.SmartPort = 1080
	}
	if cfg.GlobalPort == 0 {
		cfg.GlobalPort = 1081
	}
	return &cfg, nil
}

func decodeBinary(data []byte) (*LinkConfig, error) {
	if len(data) < 9 { // 1+1+4+2+1 minimum
		return nil, fmt.Errorf("binary data too short")
	}
	flags := data[1]
	ip := net.IP(data[2:6]).String()
	port := int(binary.BigEndian.Uint16(data[6:8]))
	userHash := string(data[8:])

	return &LinkConfig{
		Host:       ip,
		Port:       port,
		UserHash:   userHash,
		SmartPort:  1080,
		GlobalPort: 1081,
		TFO:        flags&0x01 != 0,
	}, nil
}

// --- Crypto ---

func deriveKey(key string) []byte {
	h := sha256.Sum256([]byte(key))
	return h[:]
}

func encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decrypt(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return gcm.Open(nil, ciphertext[:ns], ciphertext[ns:], nil)
}

// --- Legacy v1 ---

func isAllUpperAlpha(s string) bool {
	for _, c := range s {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

func parseLegacyV1(body string, key string) (*LinkConfig, error) {
	encrypted, err := base26ToBytes(body)
	if err != nil {
		return nil, err
	}
	data, err := decrypt(encrypted, deriveKey(key))
	if err != nil {
		return nil, fmt.Errorf("v1 decrypt failed: %w", err)
	}
	var cfg LinkConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.SmartPort == 0 {
		cfg.SmartPort = 1080
	}
	if cfg.GlobalPort == 0 {
		cfg.GlobalPort = 1081
	}
	return &cfg, nil
}

func base26ToBytes(s string) ([]byte, error) {
	n := new(big.Int)
	base := big.NewInt(26)
	for _, c := range s {
		if c < 'A' || c > 'Z' {
			return nil, fmt.Errorf("invalid char: %c", c)
		}
		n.Mul(n, base)
		n.Add(n, big.NewInt(int64(c-'A')))
	}
	data := n.Bytes()
	if len(data) > 0 && data[0] == 0x01 {
		data = data[1:]
	}
	return data, nil
}

// Package link implements ZA:// encrypted link generation and parsing.
// Format: ZA://BASE26_ENCODED_CIPHERTEXT
// Encryption: AES-256-GCM
// Encoding: Base26 (A-Z only, no digits)
package link

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strings"
)

// DefaultKey is the built-in encryption key. Users can override.
var DefaultKey = "ZRaySecretKey!!!"

// LinkConfig holds the connection parameters encoded in a ZA link.
type LinkConfig struct {
	Host       string `json:"h"`
	Port       int    `json:"p"`
	UserHash   string `json:"u"`
	SmartPort  int    `json:"s,omitempty"`  // default 1080
	GlobalPort int    `json:"g,omitempty"`  // default 1081
	TFO        bool   `json:"t,omitempty"`
}

// Generate creates a ZA:// link from config.
func Generate(cfg *LinkConfig, key string) (string, error) {
	if key == "" {
		key = DefaultKey
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}

	encrypted, err := encrypt(data, deriveKey(key))
	if err != nil {
		return "", err
	}

	encoded := bytesToBase26(encrypted)
	return "ZA://" + encoded, nil
}

// Parse decodes a ZA:// link back to config.
func Parse(link string, key string) (*LinkConfig, error) {
	if key == "" {
		key = DefaultKey
	}

	// Strip prefix (case insensitive)
	upper := strings.ToUpper(link)
	if !strings.HasPrefix(upper, "ZA://") {
		return nil, fmt.Errorf("invalid ZA link: missing ZA:// prefix")
	}
	body := link[5:]
	// Normalize to uppercase
	body = strings.ToUpper(body)

	encrypted, err := base26ToBytes(body)
	if err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	data, err := decrypt(encrypted, deriveKey(key))
	if err != nil {
		return nil, fmt.Errorf("decrypt failed: %w", err)
	}

	var cfg LinkConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config failed: %w", err)
	}

	// Apply defaults
	if cfg.SmartPort == 0 {
		cfg.SmartPort = 1080
	}
	if cfg.GlobalPort == 0 {
		cfg.GlobalPort = 1081
	}

	return &cfg, nil
}

// deriveKey creates a 32-byte AES key from arbitrary string.
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
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// bytesToBase26 encodes bytes to uppercase A-Z string (no digits).
func bytesToBase26(data []byte) string {
	// Prefix with 0x01 to preserve leading zeros
	padded := append([]byte{0x01}, data...)
	n := new(big.Int).SetBytes(padded)

	base := big.NewInt(26)
	zero := big.NewInt(0)
	mod := new(big.Int)

	var result []byte
	for n.Cmp(zero) > 0 {
		n.DivMod(n, base, mod)
		result = append(result, byte('A'+mod.Int64()))
	}

	// Reverse
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}

// base26ToBytes decodes an A-Z string back to bytes.
func base26ToBytes(s string) ([]byte, error) {
	n := new(big.Int)
	base := big.NewInt(26)

	for _, c := range s {
		if c < 'A' || c > 'Z' {
			return nil, fmt.Errorf("invalid character: %c", c)
		}
		n.Mul(n, base)
		n.Add(n, big.NewInt(int64(c-'A')))
	}

	data := n.Bytes()
	// Strip the 0x01 prefix
	if len(data) > 0 && data[0] == 0x01 {
		data = data[1:]
	}
	return data, nil
}

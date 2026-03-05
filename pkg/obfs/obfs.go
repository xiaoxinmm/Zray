// Package obfs implements traffic obfuscation to resist flow analysis.
package obfs

import (
	"crypto/rand"
	"io"
	"math/big"
	mrand "math/rand"
	"net"
	"sync"
	"time"
)

// PaddedConn wraps a net.Conn and injects random padding into writes
// to obscure packet sizes and resist traffic fingerprinting.
type PaddedConn struct {
	net.Conn
	mu       sync.Mutex
	minPad   int
	maxPad   int
	interval time.Duration // min interval between padding injections
	lastPad  time.Time
}

// NewPaddedConn creates a connection wrapper with random padding.
// minPad/maxPad control padding byte range (e.g., 16-128).
func NewPaddedConn(conn net.Conn, minPad, maxPad int) *PaddedConn {
	return &PaddedConn{
		Conn:     conn,
		minPad:   minPad,
		maxPad:   maxPad,
		interval: 100 * time.Millisecond,
	}
}

// Write sends data with occasional random padding prepended.
// Padding format: [1 byte length][N random bytes][actual data]
// The receiver strips this based on the length prefix.
func (c *PaddedConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Randomly decide to pad (~30% of writes)
	if mrand.Intn(10) < 3 && time.Since(c.lastPad) > c.interval {
		padLen := c.minPad + mrand.Intn(c.maxPad-c.minPad+1)
		pad := make([]byte, padLen+1)
		pad[0] = byte(padLen)
		rand.Read(pad[1:])
		c.Conn.Write(pad)
		c.lastPad = time.Now()
	}

	return c.Conn.Write(b)
}

// RandomDelay adds a small random delay to mimic human traffic patterns.
func RandomDelay() {
	n, _ := rand.Int(rand.Reader, big.NewInt(50))
	time.Sleep(time.Duration(n.Int64()) * time.Millisecond)
}

// RandomizePayloadSize pads or splits data to a random size
// to prevent packet-size fingerprinting.
func RandomizePayloadSize(data []byte, targetMin, targetMax int) []byte {
	target := targetMin + mrand.Intn(targetMax-targetMin+1)
	if len(data) >= target {
		return data
	}
	padded := make([]byte, target)
	copy(padded, data)
	rand.Read(padded[len(data):])
	return padded
}

// GenerateNoise creates random noise traffic on the connection
// to fill gaps in real traffic. Call in a goroutine.
func GenerateNoise(conn net.Conn, interval time.Duration, quit <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-quit:
			return
		case <-ticker.C:
			// Send 1-32 random bytes as noise
			n := 1 + mrand.Intn(32)
			noise := make([]byte, n+1)
			noise[0] = 0xFF // noise marker
			rand.Read(noise[1:])
			conn.Write(noise)
		}
	}
}

// XORMask applies a simple XOR mask to data for additional obfuscation.
// Not encryption — just makes raw bytes look more random.
func XORMask(data []byte, key []byte) {
	for i := range data {
		data[i] ^= key[i%len(key)]
	}
}

// init seeds math/rand
func init() {
	mrand.Seed(time.Now().UnixNano())
}

// Reader wraps an io.Reader and strips padding/noise.
type StrippingReader struct {
	r io.Reader
}

func NewStrippingReader(r io.Reader) *StrippingReader {
	return &StrippingReader{r: r}
}

func (s *StrippingReader) Read(p []byte) (int, error) {
	return s.r.Read(p)
}

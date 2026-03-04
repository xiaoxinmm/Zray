// Package protocol implements the ZRay wire protocol.
package protocol

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	mrand "math/rand"
	"time"
)

const (
	Version    = 0x01
	HeaderLen  = 1 + 8 + 8 + 16 // ver + ts + nonce + hash
	CmdConnect = 0x01

	AtypIPv4   = 0x01
	AtypDomain = 0x03
	AtypIPv6   = 0x04
)

// Header is the ZRay authentication header.
type Header struct {
	Version  byte
	Time     int64
	Nonce    uint64
	UserHash string // 16 bytes
}

// Marshal encodes the header to wire format.
func (h *Header) Marshal() []byte {
	buf := make([]byte, HeaderLen)
	buf[0] = h.Version
	binary.BigEndian.PutUint64(buf[1:9], uint64(h.Time))
	binary.BigEndian.PutUint64(buf[9:17], h.Nonce)
	copy(buf[17:], h.UserHash)
	return buf
}

// NewHeader creates a new header with current timestamp and random nonce.
func NewHeader(userHash string) *Header {
	var nonceBuf [8]byte
	rand.Read(nonceBuf[:])
	return &Header{
		Version:  Version,
		Time:     time.Now().Unix(),
		Nonce:    binary.BigEndian.Uint64(nonceBuf[:]),
		UserHash: userHash,
	}
}

// ParseHeader reads and validates a header from r.
func ParseHeader(r io.Reader, expectedHash string, maxTimeDrift int64) (*Header, error) {
	buf := make([]byte, HeaderLen)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	h := &Header{
		Version:  buf[0],
		Time:     int64(binary.BigEndian.Uint64(buf[1:9])),
		Nonce:    binary.BigEndian.Uint64(buf[9:17]),
		UserHash: string(buf[17:]),
	}
	if h.Version != Version {
		return nil, fmt.Errorf("version mismatch: %d", h.Version)
	}
	if h.UserHash != expectedHash {
		return nil, fmt.Errorf("auth failed")
	}
	if math.Abs(float64(time.Now().Unix()-h.Time)) > float64(maxTimeDrift) {
		return nil, fmt.Errorf("expired request")
	}
	return h, nil
}

// Address represents a target address (port + atyp + host).
type Address struct {
	Port uint16
	Type byte
	Host string
	Raw  []byte // raw bytes for forwarding
}

// MarshalForWire encodes address as: port(2) + atyp(1) + addr(variable)
func (a *Address) MarshalForWire() []byte {
	var buf []byte
	portBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(portBuf, a.Port)
	buf = append(buf, portBuf...)
	buf = append(buf, a.Type)
	buf = append(buf, a.Raw...)
	return buf
}

// WriteRequest writes the full ZRay request (header + padding + cmd + addr).
func WriteRequest(w io.Writer, userHash string, cmd byte, addr *Address) error {
	h := NewHeader(userHash)
	if _, err := w.Write(h.Marshal()); err != nil {
		return err
	}
	// random padding
	padLen := byte(mrand.Intn(50) + 10)
	pad := make([]byte, 1+int(padLen))
	pad[0] = padLen
	rand.Read(pad[1:])
	if _, err := w.Write(pad); err != nil {
		return err
	}
	// cmd
	if _, err := w.Write([]byte{cmd}); err != nil {
		return err
	}
	// address
	if _, err := w.Write(addr.MarshalForWire()); err != nil {
		return err
	}
	return nil
}

// ReadAddress reads a target address from the reader.
func ReadAddress(r io.Reader) (string, error) {
	buf := make([]byte, 260)
	if _, err := io.ReadFull(r, buf[:2]); err != nil {
		return "", err
	}
	port := binary.BigEndian.Uint16(buf[:2])
	if _, err := io.ReadFull(r, buf[:1]); err != nil {
		return "", err
	}
	atyp := buf[0]
	var host string
	switch atyp {
	case AtypIPv4:
		if _, err := io.ReadFull(r, buf[:4]); err != nil {
			return "", err
		}
		host = fmt.Sprintf("%d.%d.%d.%d", buf[0], buf[1], buf[2], buf[3])
	case AtypDomain:
		if _, err := io.ReadFull(r, buf[:1]); err != nil {
			return "", err
		}
		n := int(buf[0])
		if _, err := io.ReadFull(r, buf[:n]); err != nil {
			return "", err
		}
		host = string(buf[:n])
	case AtypIPv6:
		if _, err := io.ReadFull(r, buf[:16]); err != nil {
			return "", err
		}
		host = fmt.Sprintf("[%x:%x:%x:%x:%x:%x:%x:%x]",
			binary.BigEndian.Uint16(buf[0:2]), binary.BigEndian.Uint16(buf[2:4]),
			binary.BigEndian.Uint16(buf[4:6]), binary.BigEndian.Uint16(buf[6:8]),
			binary.BigEndian.Uint16(buf[8:10]), binary.BigEndian.Uint16(buf[10:12]),
			binary.BigEndian.Uint16(buf[12:14]), binary.BigEndian.Uint16(buf[14:16]))
	default:
		return "", fmt.Errorf("unknown atyp: %d", atyp)
	}
	return fmt.Sprintf("%s:%d", host, port), nil
}

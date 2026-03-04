// Package zraylib exposes ZRay client functionality for Android via gomobile.
package zraylib

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync/atomic"
	"time"
)

var (
	running    int32
	smartAddr  string
	globalAddr string
	stats      Stats
)

// Stats holds runtime statistics.
type Stats struct {
	Upload    int64
	Download  int64
	Active    int64
	Direct    int64
	Proxied   int64
}

// GetStats returns current stats as JSON.
func GetStats() string {
	s := Stats{
		Upload:   atomic.LoadInt64(&stats.Upload),
		Download: atomic.LoadInt64(&stats.Download),
		Active:   atomic.LoadInt64(&stats.Active),
		Direct:   atomic.LoadInt64(&stats.Direct),
		Proxied:  atomic.LoadInt64(&stats.Proxied),
	}
	b, _ := json.Marshal(s)
	return string(b)
}

// Start starts the ZRay client with the given JSON config.
func Start(configJSON string) error {
	if !atomic.CompareAndSwapInt32(&running, 0, 1) {
		return fmt.Errorf("already running")
	}

	var cfg struct {
		SmartPort  string `json:"smart_port"`
		GlobalPort string `json:"global_port"`
		RemoteHost string `json:"remote_host"`
		RemotePort int    `json:"remote_port"`
		UserHash   string `json:"user_hash"`
	}

	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		atomic.StoreInt32(&running, 0)
		return err
	}

	if cfg.SmartPort == "" {
		cfg.SmartPort = "127.0.0.1:1080"
	}
	if cfg.GlobalPort == "" {
		cfg.GlobalPort = "127.0.0.1:1081"
	}

	smartAddr = cfg.SmartPort
	globalAddr = cfg.GlobalPort

	go runSocks5Listener(cfg.SmartPort, false)
	go runSocks5Listener(cfg.GlobalPort, true)

	log.Printf("[ZRay] Started: smart=%s global=%s", cfg.SmartPort, cfg.GlobalPort)
	return nil
}

// Stop stops the ZRay client.
func Stop() {
	atomic.StoreInt32(&running, 0)
	log.Println("[ZRay] Stopped")
}

// IsRunning returns whether the client is running.
func IsRunning() bool {
	return atomic.LoadInt32(&running) == 1
}

func runSocks5Listener(addr string, forceProxy bool) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[ZRay] Listen failed: %s: %v", addr, err)
		return
	}
	defer l.Close()

	for atomic.LoadInt32(&running) == 1 {
		l.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))
		conn, err := l.Accept()
		if err != nil {
			continue
		}
		go handleConn(conn, forceProxy)
	}
}

func handleConn(c net.Conn, forceProxy bool) {
	defer c.Close()
	atomic.AddInt64(&stats.Active, 1)
	defer atomic.AddInt64(&stats.Active, -1)

	buf := make([]byte, 512)
	// SOCKS5 handshake
	if _, err := io.ReadFull(c, buf[:2]); err != nil {
		return
	}
	n := int(buf[1])
	if _, err := io.ReadFull(c, buf[:n]); err != nil {
		return
	}
	c.Write([]byte{5, 0})

	if _, err := io.ReadFull(c, buf[:4]); err != nil {
		return
	}
	atyp := buf[3]

	var host string
	var destBytes []byte
	switch atyp {
	case 1:
		io.ReadFull(c, buf[4:10])
		destBytes = buf[4:10]
		host = fmt.Sprintf("%d.%d.%d.%d", buf[4], buf[5], buf[6], buf[7])
	case 3:
		io.ReadFull(c, buf[4:5])
		l := int(buf[4])
		io.ReadFull(c, buf[5:5+l+2])
		destBytes = buf[4 : 5+l+2]
		host = string(buf[5 : 5+l])
	case 4:
		io.ReadFull(c, buf[4:22])
		destBytes = buf[4:22]
		host = "ipv6"
	}
	_ = destBytes

	port := binary.BigEndian.Uint16(destBytes[len(destBytes)-2:])
	target := fmt.Sprintf("%s:%d", host, port)

	// Simple direct connection for now (proxy logic to be added)
	dst, err := net.DialTimeout("tcp", target, 10*time.Second)
	if err != nil {
		return
	}
	defer dst.Close()

	c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})

	done := make(chan struct{}, 2)
	go func() { io.Copy(dst, c); done <- struct{}{} }()
	go func() { io.Copy(c, dst); done <- struct{}{} }()
	<-done
}

// Package api provides an HTTP control API for the ZRay core engine.
package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

// Stats holds runtime statistics exposed to the UI.
type Stats struct {
	Upload      int64  `json:"upload"`
	Download    int64  `json:"download"`
	UpSpeed     int64  `json:"up_speed"`
	DownSpeed   int64  `json:"down_speed"`
	Active      int64  `json:"active"`
	Direct      int64  `json:"direct"`
	Proxied     int64  `json:"proxied"`
	Latency     int64  `json:"latency_ms"`
	SmartPort   string `json:"smart_port"`
	GlobalPort  string `json:"global_port"`
	RemoteHost  string `json:"remote_host"`
	RemotePort  int    `json:"remote_port"`
	Running     bool   `json:"running"`
}

var (
	UploadBytes   int64
	DownloadBytes int64
	ActiveConns   int64
	DirectConns   int64
	ProxiedConns  int64
	LatencyMs     int64
	UpSpeed       int64
	DownSpeed     int64
	LastUpload    int64
	LastDownload  int64
	IsRunning     int32

	SmartPort  string
	GlobalPort string
	RemoteHost string
	RemotePort int

	StopFunc func()
)

func GetStats() *Stats {
	up := atomic.LoadInt64(&UploadBytes)
	down := atomic.LoadInt64(&DownloadBytes)
	upSpeed := up - atomic.SwapInt64(&LastUpload, up)
	downSpeed := down - atomic.SwapInt64(&LastDownload, down)
	if upSpeed < 0 {
		upSpeed = 0
	}
	if downSpeed < 0 {
		downSpeed = 0
	}

	return &Stats{
		Upload:     up,
		Download:   down,
		UpSpeed:    atomic.LoadInt64(&UpSpeed),
		DownSpeed:  atomic.LoadInt64(&DownSpeed),
		Active:     atomic.LoadInt64(&ActiveConns),
		Direct:     atomic.LoadInt64(&DirectConns),
		Proxied:    atomic.LoadInt64(&ProxiedConns),
		Latency:    atomic.LoadInt64(&LatencyMs),
		SmartPort:  SmartPort,
		GlobalPort: GlobalPort,
		RemoteHost: RemoteHost,
		RemotePort: RemotePort,
		Running:    atomic.LoadInt32(&IsRunning) == 1,
	}
}

// StartAPI starts the HTTP API server on the given port.
func StartAPI(port int) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(GetStats())
	})

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if atomic.LoadInt32(&IsRunning) == 1 {
			w.Write([]byte("running"))
		} else {
			w.Write([]byte("stopped"))
		}
	})

	mux.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if StopFunc != nil {
			StopFunc()
		}
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		ms := atomic.LoadInt64(&LatencyMs)
		fmt.Fprintf(w, "%d", ms)
	})

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	go http.Serve(ln, mux)
	return nil
}

// StartLatencyProbe periodically measures TCP latency to the remote server.
func StartLatencyProbe(host string, port int, interval time.Duration) {
	go func() {
		for {
			addr := fmt.Sprintf("%s:%d", host, port)
			start := time.Now()
			conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
			if err == nil {
				ms := time.Since(start).Milliseconds()
				atomic.StoreInt64(&LatencyMs, ms)
				conn.Close()
			} else {
				atomic.StoreInt64(&LatencyMs, -1)
			}
			time.Sleep(interval)
		}
	}()
}

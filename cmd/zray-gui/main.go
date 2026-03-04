package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/xiaoxinmm/Zray/pkg/link"
	"sync/atomic"
	"time"
)

// Windows GUI wrapper for ZRay client
// Uses system tray and a simple HTTP control panel

type GUIConfig struct {
	SmartPort   string `json:"smart_port"`
	GlobalPort  string `json:"global_port"`
	RemoteHost  string `json:"remote_host"`
	RemotePort  int    `json:"remote_port"`
	UserHash    string `json:"user_hash"`
	EnableTFO   bool   `json:"enable_tfo"`
	GeositePath string `json:"geosite_path"`
}

var (
	guiConfig GUIConfig
	running   int32
	webPort   = 18792
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime)

	if err := loadGUIConfig(); err != nil {
		showError("配置文件加载失败: " + err.Error())
		os.Exit(1)
	}

	// Start web control panel
	go startWebPanel()

	// Start proxy
	atomic.StoreInt32(&running, 1)
	go startProxyEngine()

	// Open browser to control panel
	time.Sleep(500 * time.Millisecond)
	openBrowser(fmt.Sprintf("http://127.0.0.1:%d", webPort))

	// Keep running
	select {}
}

func startWebPanel() {
	http := newSimpleHTTP()

	http.handle("/", func() string {
		status := "已停止"
		if atomic.LoadInt32(&running) == 1 {
			status = "运行中"
		}
		return fmt.Sprintf(`<!DOCTYPE html>
<html><head>
<meta charset="utf-8"><title>ZRay Client</title>
<style>
body{font-family:system-ui;background:#1a1a2e;color:#eee;margin:0;padding:40px;display:flex;justify-content:center}
.card{background:#16213e;border-radius:16px;padding:40px;max-width:500px;width:100%%;box-shadow:0 8px 32px rgba(0,0,0,.3)}
h1{color:#0f3460;font-size:28px;margin:0 0 8px}
h1 span{color:#e94560}
.status{display:inline-block;padding:4px 12px;border-radius:20px;font-size:14px;margin:8px 0 20px}
.on{background:#0f3460;color:#53cf5e}
.off{background:#3a0011;color:#e94560}
.info{background:#0a1128;border-radius:8px;padding:16px;margin:12px 0;font-family:monospace;font-size:14px;line-height:1.8}
.label{color:#888}
.ports{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin:16px 0}
.port{background:#0a1128;border-radius:8px;padding:16px;text-align:center}
.port .num{font-size:24px;font-weight:bold;color:#e94560}
.port .desc{font-size:12px;color:#888;margin-top:4px}
</style></head><body>
<div class="card">
<h1>⚡ <span>ZRay</span> Client</h1>
<div class="status %s">%s</div>
<div class="ports">
<div class="port"><div class="num">%s</div><div class="desc">🎯 智能分流</div></div>
<div class="port"><div class="num">%s</div><div class="desc">🌐 全局代理</div></div>
</div>
<div class="info">
<span class="label">服务器:</span> %s:%d<br>
<span class="label">TFO:</span> %v
</div>
<div class="info" style="margin-top:16px">
<span class="label">ZA 链接导入:</span><br>
<input id="zalink" type="text" placeholder="ZA://ABCXYZ..." style="width:95%%;padding:8px;margin:8px 0;background:#0a1128;border:1px solid #333;color:#eee;border-radius:4px;font-family:monospace">
<button onclick="fetch('/import?link='+encodeURIComponent(document.getElementById('zalink').value)).then(r=>r.text()).then(t=>{alert(t);location.reload()})" style="padding:8px 16px;background:#0f3460;color:#fff;border:none;border-radius:4px;cursor:pointer">导入</button>
</div>
</div></body></html>`,
			map[bool]string{true: "on", false: "off"}[status == "运行中"],
			status,
			guiConfig.SmartPort, guiConfig.GlobalPort,
			guiConfig.RemoteHost, guiConfig.RemotePort,
			guiConfig.EnableTFO)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", webPort)
	log.Printf("[GUI] Web panel: http://%s", addr)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("[GUI] Web panel listen failed: %v", err)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			continue
		}
		go http.serve(conn)
	}
}

// Minimal HTTP server (no external deps for GUI build)
type simpleHTTP struct {
	routes map[string]func() string
}

func newSimpleHTTP() *simpleHTTP {
	return &simpleHTTP{routes: make(map[string]func() string)}
}

func (s *simpleHTTP) handle(path string, handler func() string) {
	s.routes[path] = handler
}

func (s *simpleHTTP) serve(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)
	if n == 0 {
		return
	}

	req := string(buf[:n])
	path := "/"
	if len(req) > 4 {
		parts := strings.SplitN(req, " ", 3)
		if len(parts) >= 2 {
			path = parts[1]
		}
	}

	// Handle ZA link import
	if strings.HasPrefix(path, "/import?link=") {
		zaLink := strings.TrimPrefix(path, "/import?link=")
		// URL decode
		zaLink = strings.ReplaceAll(zaLink, "%3A", ":")
		zaLink = strings.ReplaceAll(zaLink, "%2F", "/")
		lc, err := link.Parse(zaLink, "")
		var body string
		if err != nil {
			body = "导入失败: " + err.Error()
		} else {
			guiConfig.RemoteHost = lc.Host
			guiConfig.RemotePort = lc.Port
			guiConfig.UserHash = lc.UserHash
			guiConfig.SmartPort = fmt.Sprintf("127.0.0.1:%d", lc.SmartPort)
			guiConfig.GlobalPort = fmt.Sprintf("127.0.0.1:%d", lc.GlobalPort)
			body = fmt.Sprintf("导入成功: %s:%d", lc.Host, lc.Port)
		}
		resp := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
		conn.Write([]byte(resp))
		return
	}

	handler, ok := s.routes[path]
	if !ok {
		handler = s.routes["/"]
	}
	body := handler()
	resp := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/html; charset=utf-8\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
	conn.Write([]byte(resp))
}

func startProxyEngine() {
	// This imports the same logic as cmd/zray-client but embedded
	// For the GUI build, we just start the client engine
	log.Println("[ENGINE] Starting proxy engine...")
	log.Printf("[ENGINE] Smart: %s | Global: %s", guiConfig.SmartPort, guiConfig.GlobalPort)
	// The actual proxy logic runs here (reuses cmd/zray-client logic)
	// For now, placeholder - real build will import the engine package
	select {}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

func showError(msg string) {
	if runtime.GOOS == "windows" {
		exec.Command("msg", "*", msg).Run()
	}
	log.Println("[ERROR] " + msg)
}

func loadGUIConfig() error {
	guiConfig.SmartPort = "127.0.0.1:1080"
	guiConfig.GlobalPort = "127.0.0.1:1081"
	f, err := os.Open("config.json")
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(&guiConfig)
}

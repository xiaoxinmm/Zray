package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/xiaoxinmm/Zray/pkg/camo"
	"github.com/xiaoxinmm/Zray/pkg/link"
	"github.com/xiaoxinmm/Zray/pkg/protocol"
)

type Config struct {
	RemotePort int    `json:"remote_port"`
	UserHash   string `json:"user_hash"`
	CertFile   string `json:"cert_file"`
	KeyFile    string `json:"key_file"`
	EnableTFO  bool   `json:"enable_tfo"`
}

var (
	config     Config
	nonceCache sync.Map
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
	go cleanNonceCache()
}

func cleanNonceCache() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		now := time.Now().Unix()
		nonceCache.Range(func(key, value interface{}) bool {
			if now-value.(int64) > 60 {
				nonceCache.Delete(key)
			}
			return true
		})
	}
}

func main() {
	if err := loadConfig("config.json"); err != nil {
		log.Fatalf("[FATAL] 加载配置失败: %v", err)
	}

	cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		log.Fatalf("[FATAL] 证书加载失败: %v", err)
	}

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}

	log.Println("==================================================")
	log.Println("           ZRay Server v2.3")
	log.Println("==================================================")
	log.Printf("[INFO] 监听端口: %d", config.RemotePort)
	log.Printf("[INFO] TFO: %v", config.EnableTFO)
	log.Printf("[INFO] TLS: 已启用")

	// 生成 ZA 链接
	publicIP := getPublicIP()
	if publicIP != "" {
		lc := &link.LinkConfig{
			Host:     publicIP,
			Port:     config.RemotePort,
			UserHash: config.UserHash,
		}
		zaLink, err := link.GenerateBinary(lc, "")
		if err == nil {
			log.Println("--------------------------------------------------")
			log.Printf("[LINK] ZA 分享链接:")
			log.Printf("[LINK] %s", zaLink)
		}
	}
	log.Println("--------------------------------------------------")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("[INFO] 收到退出信号，正在关闭...")
		cancel()
	}()

	startListener(ctx, tlsConfig)
}

func startListener(ctx context.Context, tlsConfig *tls.Config) {
	lc := net.ListenConfig{}
	if config.EnableTFO {
		applyServerTFO(&lc)
	}

	ln, err := lc.Listen(ctx, "tcp", fmt.Sprintf(":%d", config.RemotePort))
	if err != nil {
		log.Fatalf("[FATAL] 监听失败: %v", err)
	}
	defer ln.Close()

	tlsLn := tls.NewListener(ln, tlsConfig)
	log.Printf("[INFO] 服务启动成功，等待连接...")

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := tlsLn.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	addr := conn.RemoteAddr().String()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	br := bufio.NewReader(conn)

	if err := camo.StripHTTPCamo(br); err != nil {
		return
	}

	hdr, err := protocol.ParseHeader(br, config.UserHash, 30)
	if err != nil {
		log.Printf("[SEC] %s: %v", addr, err)
		return
	}

	if _, loaded := nonceCache.LoadOrStore(hdr.Nonce, hdr.Time); loaded {
		log.Printf("[SEC] 重放攻击: %s", addr)
		return
	}

	conn.SetReadDeadline(time.Time{})
	log.Printf("[AUTH] 验证通过: %s", addr)

	padBuf := make([]byte, 1)
	if _, err := io.ReadFull(br, padBuf); err != nil {
		return
	}
	if padBuf[0] > 0 {
		io.ReadFull(br, make([]byte, padBuf[0]))
	}

	cmdBuf := make([]byte, 1)
	if _, err := io.ReadFull(br, cmdBuf); err != nil {
		return
	}

	if cmdBuf[0] == protocol.CmdConnect {
		handleTCP(conn, br, addr)
	}
}

func handleTCP(clientConn net.Conn, r io.Reader, clientAddr string) {
	targetAddr, err := protocol.ReadAddress(r)
	if err != nil {
		log.Printf("[ERR] 解析目标失败: %v", err)
		return
	}
	log.Printf("[PROXY] %s -> %s", clientAddr, targetAddr)

	targetConn, err := net.DialTimeout("tcp", targetAddr, 10*time.Second)
	if err != nil {
		log.Printf("[ERR] 连接失败: %s -> %v", targetAddr, err)
		return
	}
	defer targetConn.Close()

	done := make(chan struct{}, 2)
	go func() { io.Copy(targetConn, r); done <- struct{}{} }()
	go func() { io.Copy(clientConn, targetConn); done <- struct{}{} }()
	<-done
}

func loadConfig(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(&config)
}

func getPublicIP() string {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", "https://api.ipify.org", nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return ""
	}
	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) == nil {
		return ""
	}
	return ip
}

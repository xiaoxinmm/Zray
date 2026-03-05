package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"

	utls "github.com/refraction-networking/utls"
	"github.com/xiaoxinmm/Zray/pkg/api"
	"github.com/xiaoxinmm/Zray/pkg/camo"
	"github.com/xiaoxinmm/Zray/pkg/link"
	"github.com/xiaoxinmm/Zray/pkg/protocol"
	"github.com/xiaoxinmm/Zray/pkg/proxy"
	"github.com/xiaoxinmm/Zray/pkg/routing"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
)

type Config struct {
	SmartPort   string `json:"smart_port"`
	GlobalPort  string `json:"global_port"`
	RemoteHost  string `json:"remote_host"`
	RemotePort  int    `json:"remote_port"`
	UserHash    string `json:"user_hash"`
	EnableTFO   bool   `json:"enable_tfo"`
	GeositePath string `json:"geosite_path"`
}

var (
	config Config
	router *routing.Router
)

func main() {
	linkFlag := flag.String("link", "", "ZA:// 加密链接导入配置")
	linkKey := flag.String("key", "", "ZA 链接解密密钥 (默认内置)")
	flag.Parse()

	setupLogFile()

	// ZA 链接导入：解密后直接覆盖 config.json
	if *linkFlag != "" {
		lc, err := link.Parse(*linkFlag, *linkKey)
		if err != nil {
			fmt.Printf("%s[FATAL] ZA 链接解析失败: %v%s\n", ColorRed, err, ColorReset)
			os.Exit(1)
		}
		// 写入 config.json
		cfg := Config{
			SmartPort:   fmt.Sprintf("127.0.0.1:%d", lc.SmartPort),
			GlobalPort:  fmt.Sprintf("127.0.0.1:%d", lc.GlobalPort),
			RemoteHost:  lc.Host,
			RemotePort:  lc.Port,
			UserHash:    lc.UserHash,
			EnableTFO:   lc.TFO,
			GeositePath: "rules/geosite-cn.txt",
		}
		data, _ := json.MarshalIndent(cfg, "", "  ")
		if err := os.WriteFile("config.json", data, 0644); err != nil {
			fmt.Printf("%s[FATAL] 写入配置失败: %v%s\n", ColorRed, err, ColorReset)
			os.Exit(1)
		}
		fmt.Printf("%s[INFO] ZA 链接导入成功，配置已保存: %s:%d%s\n", ColorGreen, lc.Host, lc.Port, ColorReset)
		// 继续用导入的配置启动
		config = cfg
	} else {
		if err := loadConfig(); err != nil {
			fmt.Printf("%s[FATAL] 加载配置失败: %v%s\n", ColorRed, err, ColorReset)
			os.Exit(1)
		}
	}

	// 初始化路由
	var err error
	router, err = routing.NewRouter(config.GeositePath)
	if err != nil {
		fmt.Printf("%s[WARN] 路由初始化失败: %v, 全部走代理%s\n", ColorYellow, err, ColorReset)
		router, _ = routing.NewRouter("")
	}

	printBanner()

	// 注册 API 统计
	api.SmartPort = config.SmartPort
	api.GlobalPort = config.GlobalPort
	api.RemoteHost = config.RemoteHost
	api.RemotePort = config.RemotePort
	atomic.StoreInt32(&api.IsRunning, 1)

	// 热重载：监听 config.json 变化
	go watchConfig()

	// HTTP API
	if err := api.StartAPI(18790); err != nil {
		log.Printf("[WARN] API 启动失败: %v", err)
	}

	// 延迟探测
	api.StartLatencyProbe(config.RemoteHost, config.RemotePort, 10*time.Second)

	// SOCKS5 端口
	go startSocks5(config.SmartPort, false)
	go startSocks5(config.GlobalPort, true)
	monitorStats()
}

// watchConfig 每 3 秒检查 config.json 是否变化，实现热重载
func watchConfig() {
	var lastMod time.Time
	for {
		time.Sleep(3 * time.Second)
		info, err := os.Stat("config.json")
		if err != nil {
			continue
		}
		if info.ModTime().After(lastMod) && !lastMod.IsZero() {
			var newCfg Config
			f, err := os.Open("config.json")
			if err != nil {
				continue
			}
			newCfg.SmartPort = "127.0.0.1:1080"
			newCfg.GlobalPort = "127.0.0.1:1081"
			err = json.NewDecoder(f).Decode(&newCfg)
			f.Close()
			if err != nil {
				continue
			}
			// 更新运行时配置（连接参数）
			config.RemoteHost = newCfg.RemoteHost
			config.RemotePort = newCfg.RemotePort
			config.UserHash = newCfg.UserHash
			config.EnableTFO = newCfg.EnableTFO
			api.RemoteHost = newCfg.RemoteHost
			api.RemotePort = newCfg.RemotePort
			log.Printf("[INFO] 配置热重载: %s:%d", config.RemoteHost, config.RemotePort)
		}
		lastMod = info.ModTime()
	}
}

func startSocks5(addr string, forceProxy bool) {
	mode := "Smart"
	if forceProxy {
		mode = "Global"
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("%s[FATAL] %s 端口监听失败 %s: %v%s\n", ColorRed, mode, addr, err, ColorReset)
		os.Exit(1)
	}
	log.Printf("[INFO] %s SOCKS5 listening on %s", mode, addr)
	for {
		c, err := l.Accept()
		if err != nil {
			continue
		}
		go handleSocks5(c, forceProxy)
	}
}

func handleSocks5(c net.Conn, forceProxy bool) {
	defer c.Close()
	atomic.AddInt64(&api.ActiveConns, 1)
	defer atomic.AddInt64(&api.ActiveConns, -1)

	c.SetDeadline(time.Now().Add(10 * time.Second))

	buf := make([]byte, 512)

	if _, err := io.ReadFull(c, buf[:2]); err != nil {
		return
	}
	nMethods := int(buf[1])
	if _, err := io.ReadFull(c, buf[:nMethods]); err != nil {
		return
	}
	c.Write([]byte{5, 0})

	if _, err := io.ReadFull(c, buf[:4]); err != nil {
		return
	}
	cmd, atyp := buf[1], buf[3]
	if cmd != 1 {
		c.Write([]byte{5, 7, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}

	var destBytes []byte
	var hostStr string
	switch atyp {
	case 1:
		if _, err := io.ReadFull(c, buf[4:10]); err != nil {
			return
		}
		destBytes = buf[4:10]
		hostStr = fmt.Sprintf("%d.%d.%d.%d", buf[4], buf[5], buf[6], buf[7])
	case 3:
		if _, err := io.ReadFull(c, buf[4:5]); err != nil {
			return
		}
		l := int(buf[4])
		if _, err := io.ReadFull(c, buf[5:5+l+2]); err != nil {
			return
		}
		destBytes = buf[4 : 5+l+2]
		hostStr = string(buf[5 : 5+l])
	case 4:
		if _, err := io.ReadFull(c, buf[4:22]); err != nil {
			return
		}
		destBytes = buf[4:22]
		hostStr = "IPv6"
	default:
		return
	}

	port := binary.BigEndian.Uint16(destBytes[len(destBytes)-2:])
	targetAddr := fmt.Sprintf("%s:%d", hostStr, port)

	action := routing.ActionProxy
	if !forceProxy {
		action = router.Route(hostStr)
	}

	c.SetDeadline(time.Time{})

	if action == routing.ActionDirect {
		atomic.AddInt64(&api.DirectConns, 1)
		handleDirect(c, targetAddr)
	} else {
		atomic.AddInt64(&api.ProxiedConns, 1)
		handleProxy(c, targetAddr, atyp, destBytes)
	}
}

func handleDirect(c net.Conn, target string) {
	dst, err := net.DialTimeout("tcp", target, 10*time.Second)
	if err != nil {
		log.Printf("[DIRECT] 连接失败: %s: %v", target, err)
		return
	}
	defer dst.Close()

	c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
	log.Printf("[DIRECT] %s", target)
	proxy.Relay(c, dst)
}

func handleProxy(c net.Conn, target string, atyp byte, destBytes []byte) {
	svr, err := dialServer()
	if err != nil {
		log.Printf("[PROXY] 连接远程失败: %v", err)
		return
	}
	defer svr.Close()

	// TLS 加密的 HTTP 伪装
	if err := camo.WriteHTTPCamo(svr, config.RemoteHost); err != nil {
		return
	}

	addr := &protocol.Address{
		Port: binary.BigEndian.Uint16(destBytes[len(destBytes)-2:]),
		Type: atyp,
		Raw:  destBytes[:len(destBytes)-2],
	}

	if err := protocol.WriteRequest(svr, config.UserHash, protocol.CmdConnect, addr); err != nil {
		return
	}

	c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
	log.Printf("[PROXY] %s (TLS)", target)

	up, down := proxy.Relay(c, svr)
	atomic.AddInt64(&api.UploadBytes, up)
	atomic.AddInt64(&api.DownloadBytes, down)
}

func dialServer() (net.Conn, error) {
	addr := fmt.Sprintf("%s:%d", config.RemoteHost, config.RemotePort)
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	if config.EnableTFO {
		applyTFO(dialer)
	}

	tcpConn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	if tc, ok := tcpConn.(*net.TCPConn); ok {
		tc.SetNoDelay(true)
	}

	// uTLS 模拟 Chrome 指纹，TLS 加密传输
	uConn := utls.UClient(tcpConn, &utls.Config{
		InsecureSkipVerify: true, // 自签证书需要跳过验证
		ServerName:         config.RemoteHost,
	}, utls.HelloChrome_Auto)

	if err := uConn.Handshake(); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("TLS 握手失败: %w", err)
	}
	return uConn, nil
}

func printBanner() {
	fmt.Print("\033[H\033[2J")
	fmt.Printf(`
%s███████╗██████╗  █████╗ ██╗   ██╗
╚══███╔╝██╔══██╗██╔══██╗╚██╗ ██╔╝
  ███╔╝ ██████╔╝███████║ ╚████╔╝ 
 ███╔╝  ██╔══██╗██╔══██║  ╚██╔╝  
███████╗██║  ██║██║  ██║   ██║   
╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝   %s
    %s>> ZRay Client v2.3 <<%s

 %sSmart  :%s %s (自动分流)
 %sGlobal :%s %s (全局代理)
 %sRemote :%s %s:%d
 %sTLS    :%s 已启用 (uTLS Chrome)
`, ColorCyan, ColorReset,
		ColorYellow, ColorReset,
		ColorGreen, ColorReset, config.SmartPort,
		ColorPurple, ColorReset, config.GlobalPort,
		ColorBlue, ColorReset, config.RemoteHost, config.RemotePort,
		ColorGreen, ColorReset)
	fmt.Println(strings.Repeat("-", 50))
	fmt.Println()
}

func monitorStats() {
	ticker := time.NewTicker(1 * time.Second)
	var lastUp, lastDown int64
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	idx := 0

	for range ticker.C {
		up := atomic.LoadInt64(&api.UploadBytes)
		down := atomic.LoadInt64(&api.DownloadBytes)
		conns := atomic.LoadInt64(&api.ActiveConns)
		direct := atomic.LoadInt64(&api.DirectConns)
		proxied := atomic.LoadInt64(&api.ProxiedConns)

		upSpeed := up - lastUp
		downSpeed := down - lastDown
		atomic.StoreInt64(&api.UpSpeed, upSpeed)
		atomic.StoreInt64(&api.DownSpeed, downSpeed)

		fmt.Printf("\r%s %s↑%s %s↓%s | ⚡%-3d | 🎯D:%-4d P:%-4d",
			spinner[idx],
			ColorGreen, formatSpeed(float64(upSpeed)),
			ColorCyan, formatSpeed(float64(downSpeed)),
			conns, direct, proxied)

		lastUp, lastDown = up, down
		idx = (idx + 1) % len(spinner)
	}
}

func formatSpeed(s float64) string {
	if s < 1024 {
		return fmt.Sprintf("%4.0f B/s%s", s, ColorReset)
	}
	if s < 1024*1024 {
		return fmt.Sprintf("%4.1f K/s%s", s/1024, ColorReset)
	}
	return fmt.Sprintf("%4.1f M/s%s", s/1024/1024, ColorReset)
}

func setupLogFile() {
	f, _ := os.OpenFile("zray_client.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if f != nil {
		log.SetOutput(f)
	}
}

func loadConfig() error {
	f, err := os.Open("config.json")
	if err != nil {
		return err
	}
	defer f.Close()
	config.SmartPort = "127.0.0.1:1080"
	config.GlobalPort = "127.0.0.1:1081"
	return json.NewDecoder(f).Decode(&config)
}

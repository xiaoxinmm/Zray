// Package routing implements domain/IP-based traffic routing using geosite.dat and geoip.dat.
package routing

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

// Action defines what to do with a connection.
type Action int

const (
	ActionProxy  Action = iota // route through ZRay proxy
	ActionDirect               // connect directly
	ActionBlock                // drop connection
)

// Router decides whether traffic should go through proxy or direct.
type Router struct {
	mu           sync.RWMutex
	directDomains map[string]bool // domain suffix -> direct
	proxyDomains  map[string]bool // domain suffix -> proxy
	directCIDRs   []*net.IPNet
	cnDomains     map[string]bool // domains from geosite cn category
}

// NewRouter creates a router and loads rules.
func NewRouter(geositePath string) (*Router, error) {
	r := &Router{
		directDomains: make(map[string]bool),
		proxyDomains:  make(map[string]bool),
		cnDomains:     make(map[string]bool),
	}

	// Load geosite data if available
	if geositePath != "" {
		if err := r.loadGeositeText(geositePath); err != nil {
			return nil, fmt.Errorf("load geosite: %w", err)
		}
	}

	// Add well-known China domains
	r.addBuiltinCNDomains()

	return r, nil
}

// Route decides the action for a given host.
func (r *Router) Route(host string) Action {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Strip port if present
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}

	// Check if IP
	ip := net.ParseIP(h)
	if ip != nil {
		return r.routeIP(ip)
	}

	// Domain routing
	return r.routeDomain(h)
}

func (r *Router) routeIP(ip net.IP) Action {
	// Private IPs -> direct
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return ActionDirect
	}
	// Check China CIDRs
	for _, cidr := range r.directCIDRs {
		if cidr.Contains(ip) {
			return ActionDirect
		}
	}
	return ActionProxy
}

func (r *Router) routeDomain(domain string) Action {
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))

	// Exact match first
	if r.directDomains[domain] || r.cnDomains[domain] {
		return ActionDirect
	}
	if r.proxyDomains[domain] {
		return ActionProxy
	}

	// Suffix match
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		suffix := strings.Join(parts[i:], ".")
		if r.directDomains[suffix] || r.cnDomains[suffix] {
			return ActionDirect
		}
		if r.proxyDomains[suffix] {
			return ActionProxy
		}
	}

	// Default: proxy (assume foreign)
	return ActionProxy
}

// loadGeositeText loads a simple text-based geosite file.
// Format: one domain per line, lines starting with # are comments.
// Sections: [cn], [proxy], [direct]
func (r *Router) loadGeositeText(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	section := "cn" // default section
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(line[1 : len(line)-1])
			continue
		}
		switch section {
		case "cn", "direct":
			r.cnDomains[strings.ToLower(line)] = true
		case "proxy", "gfw":
			r.proxyDomains[strings.ToLower(line)] = true
		}
	}
	return scanner.Err()
}

// AddCIDR adds a CIDR range for direct routing (e.g., China IP ranges).
func (r *Router) AddCIDR(cidr string) error {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.directCIDRs = append(r.directCIDRs, ipnet)
	r.mu.Unlock()
	return nil
}

func (r *Router) addBuiltinCNDomains() {
	// Top China domains that should always go direct
	domains := []string{
		// Major platforms
		"cn", "com.cn", "net.cn", "org.cn",
		"baidu.com", "qq.com", "weixin.qq.com", "wx.qq.com",
		"taobao.com", "tmall.com", "alipay.com", "alibaba.com",
		"jd.com", "163.com", "126.com", "sina.com.cn", "weibo.com",
		"douyin.com", "tiktok.com", "bytedance.com", "zhihu.com",
		"bilibili.com", "youku.com", "iqiyi.com", "sohu.com",
		"meituan.com", "dianping.com", "ctrip.com", "ele.me",
		"pinduoduo.com", "xiaomi.com", "huawei.com", "oppo.com",
		"vivo.com", "mi.com", "honor.cn",
		// CDN & Cloud
		"aliyun.com", "aliyuncs.com", "alicdn.com",
		"qcloud.com", "myqcloud.com", "tencent.com",
		"baidubce.com", "bdimg.com", "bdstatic.com",
		"huaweicloud.com", "volcengine.com",
		// Services
		"csdn.net", "jianshu.com", "gitee.com", "oschina.net",
		"cnblogs.com", "segmentfault.com",
		"12306.cn", "gov.cn", "edu.cn",
		// Payment
		"unionpay.com", "95516.com",
		// Delivery
		"sf-express.com", "zto.com", "yto.net.cn",
		// Streaming
		"music.163.com", "kugou.com", "kuwo.cn",
		// China specific TLDs
		"中国", "中國", "公司", "网络",
	}
	for _, d := range domains {
		r.cnDomains[d] = true
	}
}

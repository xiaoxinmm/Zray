// Package admin provides a web management panel for the ZRay server.
package admin

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

var (
	TotalConns    int64
	ActiveConns   int64
	TotalUpload   int64
	TotalDownload int64
	StartTime     time.Time

	// Per-user stats
	userStats sync.Map // map[string]*UserStats
)

type UserStats struct {
	Hash       string `json:"hash"`
	Conns      int64  `json:"conns"`
	Upload     int64  `json:"upload"`
	Download   int64  `json:"download"`
	LastActive int64  `json:"last_active"`
}

type ServerStats struct {
	Uptime      string      `json:"uptime"`
	TotalConns  int64       `json:"total_conns"`
	ActiveConns int64       `json:"active_conns"`
	Upload      int64       `json:"upload"`
	Download    int64       `json:"download"`
	Users       []UserStats `json:"users"`
}

func RecordConn(userHash string, upload, download int64) {
	atomic.AddInt64(&TotalConns, 1)
	atomic.AddInt64(&TotalUpload, upload)
	atomic.AddInt64(&TotalDownload, download)

	val, _ := userStats.LoadOrStore(userHash, &UserStats{Hash: userHash})
	us := val.(*UserStats)
	atomic.AddInt64(&us.Conns, 1)
	atomic.AddInt64(&us.Upload, upload)
	atomic.AddInt64(&us.Download, download)
	atomic.StoreInt64(&us.LastActive, time.Now().Unix())
}

func getStats() *ServerStats {
	s := &ServerStats{
		Uptime:      time.Since(StartTime).Round(time.Second).String(),
		TotalConns:  atomic.LoadInt64(&TotalConns),
		ActiveConns: atomic.LoadInt64(&ActiveConns),
		Upload:      atomic.LoadInt64(&TotalUpload),
		Download:    atomic.LoadInt64(&TotalDownload),
	}
	userStats.Range(func(key, val interface{}) bool {
		us := val.(*UserStats)
		s.Users = append(s.Users, UserStats{
			Hash:       us.Hash[:8] + "...",
			Conns:      atomic.LoadInt64(&us.Conns),
			Upload:     atomic.LoadInt64(&us.Upload),
			Download:   atomic.LoadInt64(&us.Download),
			LastActive: atomic.LoadInt64(&us.LastActive),
		})
		return true
	})
	return s
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

const adminHTML = `<!DOCTYPE html>
<html><head>
<meta charset="utf-8"><title>ZRay Admin</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:system-ui;background:#0a0a1a;color:#eee;padding:24px}
.header{text-align:center;margin-bottom:32px}
.header h1{font-size:28px;color:#e94560}
.header p{color:#666;font-size:13px}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:16px;margin-bottom:24px}
.card{background:#16213e;border-radius:12px;padding:20px}
.card .label{color:#888;font-size:12px;text-transform:uppercase;letter-spacing:1px}
.card .value{font-size:28px;font-weight:bold;margin-top:8px}
.card .value.green{color:#53cf5e}
.card .value.red{color:#e94560}
.card .value.blue{color:#00b4d8}
.card .value.yellow{color:#ffd166}
table{width:100%;border-collapse:collapse;background:#16213e;border-radius:12px;overflow:hidden}
th{background:#0f3460;padding:12px 16px;text-align:left;font-size:13px;color:#aaa}
td{padding:12px 16px;border-top:1px solid #1a1a3e;font-family:monospace;font-size:14px}
.refresh{color:#666;font-size:12px;text-align:center;margin-top:16px}
</style>
</head><body>
<div class="header">
<h1>⚡ ZRay Server Admin</h1>
<p id="uptime">Loading...</p>
</div>
<div class="grid">
<div class="card"><div class="label">活跃连接</div><div class="value green" id="active">-</div></div>
<div class="card"><div class="label">总连接数</div><div class="value blue" id="total">-</div></div>
<div class="card"><div class="label">上传流量</div><div class="value yellow" id="upload">-</div></div>
<div class="card"><div class="label">下载流量</div><div class="value red" id="download">-</div></div>
</div>
<h3 style="color:#888;margin-bottom:12px;font-size:14px">用户统计</h3>
<table>
<thead><tr><th>用户</th><th>连接数</th><th>上传</th><th>下载</th><th>最后活跃</th></tr></thead>
<tbody id="users"><tr><td colspan="5" style="text-align:center;color:#666">加载中...</td></tr></tbody>
</table>
<div class="refresh">每 2 秒自动刷新</div>
<script>
function fmt(b){if(b>=1073741824)return(b/1073741824).toFixed(1)+' GB';if(b>=1048576)return(b/1048576).toFixed(1)+' MB';if(b>=1024)return(b/1024).toFixed(1)+' KB';return b+' B'}
function ago(ts){if(!ts)return'-';var s=Math.floor(Date.now()/1000-ts);if(s<60)return s+'s ago';if(s<3600)return Math.floor(s/60)+'m ago';return Math.floor(s/3600)+'h ago'}
function refresh(){
fetch('/api/stats').then(r=>r.json()).then(d=>{
document.getElementById('uptime').textContent='运行时间: '+d.uptime;
document.getElementById('active').textContent=d.active_conns;
document.getElementById('total').textContent=d.total_conns;
document.getElementById('upload').textContent=fmt(d.upload);
document.getElementById('download').textContent=fmt(d.download);
var h='';
if(d.users&&d.users.length){d.users.forEach(u=>{h+='<tr><td>'+u.hash+'</td><td>'+u.conns+'</td><td>'+fmt(u.upload)+'</td><td>'+fmt(u.download)+'</td><td>'+ago(u.last_active)+'</td></tr>'});}
else{h='<tr><td colspan="5" style="text-align:center;color:#666">暂无用户</td></tr>';}
document.getElementById('users').innerHTML=h;
}).catch(()=>{});}
refresh();setInterval(refresh,2000);
</script></body></html>`

// StartAdmin starts the admin web panel on the given port.
func StartAdmin(port int) error {
	StartTime = time.Now()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(adminHTML))
	})
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(getStats())
	})

	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return err
	}
	go http.Serve(ln, mux)
	return nil
}

# ZRay

轻量级加密代理隧道，TLS + HTTP 伪装 + uTLS 指纹 + 智能分流。

## 特性

- 🔐 **TLS 加密** + 自定义协议头（时间戳 + nonce 防重放）
- 🎭 **HTTP 伪装** — Chrome 风格请求头，对抗 DPI
- 🖐 **uTLS 指纹** — 模拟 Chrome TLS ClientHello
- 🎯 **智能分流** — 国内直连，国外走代理（geosite 规则）
- 🌐 **双端口模式** — 1080 分流 / 1081 全局
- ⚡ **TCP Fast Open** 可选支持
- 📱 **多平台** — Linux / Windows / macOS / Android

## 项目结构

```
Zray/
├── cmd/
│   ├── zray-server/     # 服务端
│   ├── zray-client/     # 命令行客户端
│   └── zray-gui/        # Windows GUI 客户端
├── pkg/
│   ├── protocol/        # ZRay 协议实现
│   ├── proxy/           # 双向转发
│   ├── routing/         # 分流路由引擎
│   └── camo/            # HTTP 伪装
├── android/
│   └── zraylib/         # Android gomobile 绑定
├── scripts/
│   ├── install-server-cn.sh    # 国内安装脚本
│   └── install-server-intl.sh  # 国际安装脚本
├── rules/
│   └── geosite-cn.txt   # 分流规则
├── configs/
│   ├── client.example.json
│   └── server.example.json
├── tools/
│   └── gen_cert.go      # 证书生成工具
└── .github/workflows/
    └── build.yml        # CI 自动编译
```

## 快速开始

### 服务端安装

**国际服务器（一键安装）：**
```bash
curl -sL https://raw.githubusercontent.com/xiaoxinmm/Zray/main/scripts/install-server-intl.sh | sudo bash
```

**国内服务器（使用镜像加速）：**
```bash
curl -sL https://ghproxy.cc/https://raw.githubusercontent.com/xiaoxinmm/Zray/main/scripts/install-server-cn.sh | sudo bash
```

### 客户端使用

1. 下载对应平台的客户端
2. 复制 `configs/client.example.json` 为 `config.json`
3. 填入服务器信息
4. 运行客户端

```json
{
    "smart_port": "127.0.0.1:1080",
    "global_port": "127.0.0.1:1081",
    "remote_host": "your-server-ip",
    "remote_port": 64433,
    "user_hash": "your-hash",
    "geosite_path": "rules/geosite-cn.txt"
}
```

### 双端口说明

| 端口 | 模式 | 用途 |
|------|------|------|
| 1080 | 智能分流 | 国内直连，国外走代理 |
| 1081 | 全局代理 | 所有流量走代理 |

### 分流逻辑

```
请求 → 解析域名/IP
  ├─ 命中 [cn] 规则 → 直连
  ├─ 命中 [proxy] 规则 → 代理
  ├─ 私有 IP / 局域网 → 直连
  ├─ 中国 IP 段 → 直连
  └─ 默认 → 代理（假定为境外）
```

## 编译

```bash
# 服务端
go build -o zray-server ./cmd/zray-server/

# 客户端
go build -o zray-client ./cmd/zray-client/

# Windows GUI
GOOS=windows go build -ldflags="-H windowsgui" -o zray-gui.exe ./cmd/zray-gui/
```

GitHub Actions 会自动编译所有平台版本并发布 Release。

## 协议

MIT License

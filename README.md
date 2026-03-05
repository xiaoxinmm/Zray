# ZRay

轻量加密代理。TLS + HTTP 伪装 + uTLS 指纹模拟 + 智能分流。

---

## 功能

- TLS 加密传输，自定义协议头（时间戳 + nonce 防重放）
- HTTP 伪装，模拟正常浏览器请求，对抗 DPI 检测
- uTLS 指纹模拟 Chrome TLS ClientHello
- 智能分流：国内直连，国外走代理（geosite 规则匹配）
- 双端口：1080 智能分流 / 1081 全局代理
- ZA:// 加密分享链接，一键导入配置
- TCP Fast Open 可选
- 跨平台：Linux / Windows

## 下载

[Releases 页面](https://github.com/xiaoxinmm/Zray/releases) 下载最新版。

| 文件 | 说明 |
|------|------|
| `ZRay-Windows.zip` | Windows 客户端（GUI + 命令行 + 服务端） |
| `zray-linux-amd64.tar.gz` | Linux x86_64 |
| `zray-linux-arm64.tar.gz` | Linux ARM64 |

## 快速开始

### 服务端

```bash
# 一键安装（海外服务器）
curl -sL https://raw.githubusercontent.com/xiaoxinmm/Zray/main/scripts/install-server-intl.sh | sudo bash

# 一键安装（国内服务器，镜像加速）
curl -sL https://ghproxy.cc/https://raw.githubusercontent.com/xiaoxinmm/Zray/main/scripts/install-server-cn.sh | sudo bash
```

或者手动：

```bash
tar xzf zray-linux-amd64.tar.gz
# 编辑 server-config.json，填入端口和密钥
# 生成证书
openssl req -x509 -newkey rsa:2048 -keyout server.key -out server.crt -days 3650 -nodes -subj "/CN=ZRay"
./zray-server-linux-amd64
```

服务端启动后会打印 ZA 分享链接，发给客户端即可。

### Windows 客户端

1. 解压 `ZRay-Windows.zip`
2. 双击 `ZRay-GUI.exe`
3. 粘贴 ZA 链接 → 点导入 → 点连接
4. 浏览器代理设置 SOCKS5 `127.0.0.1:1080`

或者手动填写服务器地址、端口、密钥。

### 命令行客户端

```bash
# 用 ZA 链接启动
./zray-client --link "ZA://xxxxx"

# 用配置文件启动
./zray-client
```

配置文件 `config.json`：

```json
{
    "smart_port": "127.0.0.1:1080",
    "global_port": "127.0.0.1:1081",
    "remote_host": "your-server-ip",
    "remote_port": 64433,
    "user_hash": "your-secret",
    "geosite_path": "rules/geosite-cn.txt"
}
```

## 分流逻辑

1080 端口自动分流：

- 命中国内域名/IP → 直连
- 私有地址、局域网 → 直连
- 其余 → 走代理

1081 端口全部走代理，不分流。

## ZA 链接

加密分享链接，AES-256-GCM 加密 + Base64URL 编码。服务端启动自动生成。

```
ZA://KQ48z5c-8YxDJfqH6fM1T4uapzUV...
```

客户端支持命令行 `--link` 参数导入，GUI 直接粘贴导入。导入后自动写入 `config.json`。

## 项目结构

```
cmd/
  zray-server/    服务端
  zray-client/    客户端
gui/
  ZRayClient/     Windows GUI (WPF)
pkg/
  protocol/       协议实现
  proxy/          双向转发
  routing/        分流引擎
  camo/           HTTP 伪装
  link/           ZA 链接编解码
  obfs/           流量混淆
  api/            客户端 HTTP API
configs/          配置模板
rules/            分流规则
scripts/          安装脚本
```

## 编译

依赖 Go 1.24+ 和 .NET 8 SDK（GUI）。

```bash
# 服务端
go build -ldflags="-s -w" -o zray-server ./cmd/zray-server/

# 客户端
go build -ldflags="-s -w" -o zray-client ./cmd/zray-client/

# GUI（Windows）
cd gui/ZRayClient
dotnet publish -c Release -r win-x64 --self-contained true -p:PublishSingleFile=true
```

推送 tag 自动触发 GitHub Actions 编译发布。

## 协议

MIT

## 相关项目

- [Zray for Android](https://github.com/xiaoxinmm/Zray-for-Android) — Android 客户端（Material Design 3）

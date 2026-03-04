#!/bin/bash
# ZRay Server 安装脚本 - 国内环境 (使用镜像加速)
set -e

COLOR_GREEN='\033[32m'
COLOR_YELLOW='\033[33m'
COLOR_RED='\033[31m'
COLOR_RESET='\033[0m'

info()  { echo -e "${COLOR_GREEN}[INFO]${COLOR_RESET} $1"; }
warn()  { echo -e "${COLOR_YELLOW}[WARN]${COLOR_RESET} $1"; }
error() { echo -e "${COLOR_RED}[ERROR]${COLOR_RESET} $1"; exit 1; }

INSTALL_DIR="/etc/zray"
BIN_DIR="/usr/local/bin"
SERVICE_NAME="zray-server"
REPO="xiaoxinmm/Zray"
PORT=${ZRAY_PORT:-64433}
USER_HASH=${ZRAY_HASH:-$(head -c 8 /dev/urandom | xxd -p)}

# 国内 GitHub 镜像列表
MIRRORS=(
    "https://ghproxy.cc/https://github.com"
    "https://gh-proxy.com/https://github.com"
    "https://mirror.ghproxy.com/https://github.com"
    "https://github.com"
)

check_root() {
    [ "$(id -u)" -eq 0 ] || error "请使用 root 用户运行"
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        armv7*) ARCH="armv7" ;;
        *) error "不支持的架构: $(uname -m)" ;;
    esac
    OS="linux"
    info "检测到架构: ${OS}/${ARCH}"
}

try_download() {
    local path="$1"
    local output="$2"
    for mirror in "${MIRRORS[@]}"; do
        local url="${mirror}/${path}"
        info "尝试下载: ${url}"
        if curl -sL --connect-timeout 10 --max-time 120 "$url" -o "$output" 2>/dev/null; then
            if [ -s "$output" ]; then
                info "下载成功"
                return 0
            fi
        fi
        warn "镜像不可用，尝试下一个..."
    done
    error "所有镜像均不可用"
}

install_binary() {
    info "下载 ZRay Server (国内加速)..."
    
    # 尝试获取最新版本
    LATEST="v2.0.0"
    for mirror in "${MIRRORS[@]}"; do
        VER=$(curl -sL --connect-timeout 5 "${mirror/github.com/api.github.com\/repos}/${REPO}/releases/latest" 2>/dev/null | grep -oP '"tag_name":\s*"v[^"]+' | grep -oP 'v[^"]+')
        if [ -n "$VER" ]; then
            LATEST="$VER"
            break
        fi
    done
    info "版本: ${LATEST}"
    
    TMP=$(mktemp -d)
    try_download "${REPO}/releases/download/${LATEST}/zray-server-${OS}-${ARCH}.tar.gz" "${TMP}/zray.tar.gz"
    tar xzf "${TMP}/zray.tar.gz" -C "${TMP}"
    
    install -m 755 "${TMP}/zray-server" "${BIN_DIR}/zray-server"
    rm -rf "$TMP"
    info "二进制安装完成"
}

generate_cert() {
    info "生成 TLS 证书..."
    mkdir -p "$INSTALL_DIR"
    openssl req -x509 -newkey rsa:2048 -keyout "${INSTALL_DIR}/server.key" \
        -out "${INSTALL_DIR}/server.crt" -days 3650 -nodes \
        -subj "/CN=ZRay/O=ZRay Corp" 2>/dev/null
    chmod 600 "${INSTALL_DIR}/server.key"
    info "证书已生成"
}

generate_config() {
    info "生成配置文件..."
    cat > "${INSTALL_DIR}/config.json" <<EOF
{
    "remote_port": ${PORT},
    "user_hash": "${USER_HASH}",
    "cert_file": "${INSTALL_DIR}/server.crt",
    "key_file": "${INSTALL_DIR}/server.key",
    "enable_tfo": false
}
EOF
}

install_service() {
    info "安装 systemd 服务..."
    cat > /etc/systemd/system/${SERVICE_NAME}.service <<EOF
[Unit]
Description=ZRay Proxy Server
After=network.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${BIN_DIR}/zray-server
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable ${SERVICE_NAME}
    systemctl start ${SERVICE_NAME}
    info "服务已启动"
}

show_info() {
    PUBLIC_IP=$(curl -s ifconfig.me 2>/dev/null || curl -s ip.sb 2>/dev/null || echo "unknown")
    echo ""
    echo "========================================"
    echo " ZRay Server 安装完成! (国内版)"
    echo "========================================"
    echo " 服务器地址: ${PUBLIC_IP}"
    echo " 端口: ${PORT}"
    echo " UserHash: ${USER_HASH}"
    echo " 配置目录: ${INSTALL_DIR}"
    echo ""
    echo " 客户端配置:"
    cat <<EOF
 {
   "remote_host": "${PUBLIC_IP}",
   "remote_port": ${PORT},
   "user_hash": "${USER_HASH}"
 }
EOF
    echo "========================================"
}

main() {
    info "ZRay Server 安装脚本 (国内版)"
    check_root
    detect_arch
    install_binary
    generate_cert
    generate_config
    install_service
    show_info
}

main "$@"

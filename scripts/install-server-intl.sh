#!/bin/bash
# ZRay Server 安装脚本 - 国际环境
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

install_binary() {
    info "下载 ZRay Server..."
    LATEST=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep tag_name | head -1 | grep -oP '"v[^"]+')
    [ -z "$LATEST" ] && LATEST="v2.0.0"
    URL="https://github.com/${REPO}/releases/download/${LATEST}/zray-server-${OS}-${ARCH}.tar.gz"
    
    TMP=$(mktemp -d)
    curl -sL "$URL" -o "${TMP}/zray.tar.gz" || error "下载失败"
    tar xzf "${TMP}/zray.tar.gz" -C "${TMP}"
    
    install -m 755 "${TMP}/zray-server" "${BIN_DIR}/zray-server"
    rm -rf "$TMP"
    info "二进制安装完成: ${BIN_DIR}/zray-server"
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
    info "配置文件: ${INSTALL_DIR}/config.json"
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
    PUBLIC_IP=$(curl -s ifconfig.me 2>/dev/null || echo "unknown")
    echo ""
    echo "========================================"
    echo " ZRay Server 安装完成!"
    echo "========================================"
    echo " 服务器地址: ${PUBLIC_IP}"
    echo " 端口: ${PORT}"
    echo " UserHash: ${USER_HASH}"
    echo " 配置目录: ${INSTALL_DIR}"
    echo ""
    echo " 管理命令:"
    echo "   systemctl status ${SERVICE_NAME}"
    echo "   systemctl restart ${SERVICE_NAME}"
    echo "   journalctl -u ${SERVICE_NAME} -f"
    echo ""
    echo " 客户端配置:"
    echo "   remote_host: ${PUBLIC_IP}"
    echo "   remote_port: ${PORT}"
    echo "   user_hash: ${USER_HASH}"
    echo "========================================"
}

main() {
    info "ZRay Server 安装脚本 (国际版)"
    check_root
    detect_arch
    install_binary
    generate_cert
    generate_config
    install_service
    show_info
}

main "$@"

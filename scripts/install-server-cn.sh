cat << 'EOF' > install-zray.sh && bash install-zray.sh
#!/bin/bash

echo "========================================="
echo " ZRay Server 自动化安装部署脚本 (修复加强版)"
echo "========================================="

# 1. 架构检测与变量设置
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)
        OS_ARCH="amd64"
        FILE_NAME="zray-linux-amd64.tar.gz"
        ;;
    aarch64|arm64)
        OS_ARCH="arm64"
        FILE_NAME="zray-linux-arm64.tar.gz"
        ;;
    *)
        echo "[ERROR] 不支持的架构: $ARCH"
        exit 1
        ;;
esac
echo "[INFO] 检测到架构: linux/$OS_ARCH"

# 2. 稳定获取最新版本号
echo "[INFO] 正在获取 ZRay 最新版本号..."
LATEST_URL=$(curl -Ls -o /dev/null -w %{url_effective} https://github.com/xiaoxinmm/Zray/releases/latest)
LATEST_VERSION=${LATEST_URL##*/}

if [ -z "$LATEST_VERSION" ] || [[ "$LATEST_VERSION" == "latest" ]]; then
    echo "[ERROR] 无法获取最新版本号，可能遭遇网络阻断。"
    exit 1
fi
echo "[INFO] 锁定最新版本: $LATEST_VERSION"

# 3. 安全下载到临时目录
DOWNLOAD_URL="https://github.com/xiaoxinmm/Zray/releases/download/${LATEST_VERSION}/${FILE_NAME}"
echo "[INFO] 正在下载: $DOWNLOAD_URL"
curl -sL -o "/tmp/${FILE_NAME}" "$DOWNLOAD_URL"

# 4. 强校验文件
if ! gzip -t "/tmp/${FILE_NAME}" 2>/dev/null; then
    echo "[ERROR] 下载失败！拉取到的文件不是合法的压缩包。"
    rm -f "/tmp/${FILE_NAME}"
    exit 1
fi

# 5. 解压与部署
INSTALL_DIR="/usr/local/zray"
echo "[INFO] 校验通过，正在部署到 $INSTALL_DIR ..."
mkdir -p $INSTALL_DIR
tar -xzf "/tmp/${FILE_NAME}" -C $INSTALL_DIR
rm -f "/tmp/${FILE_NAME}"

cd $INSTALL_DIR
chmod +x zray-server-linux-$OS_ARCH

# 6. 生成证书
if [ ! -f "server.crt" ]; then
    echo "[INFO] 正在生成 TLS 自签证书..."
    openssl req -x509 -newkey rsa:2048 -keyout server.key -out server.crt -days 3650 -nodes -subj "/CN=ZRay" 2>/dev/null
fi

# 7. 自动化生成配置文件
echo "[INFO] 正在初始化默认配置..."
# 随机生成 16 字节的 hex 字符串作为 UserHash
RANDOM_HASH=$(openssl rand -hex 16)
DEFAULT_PORT=64433

# 假设服务端的配置文件名为 server-config.json
cat << CONF > server-config.json
{
    "listen_port": $DEFAULT_PORT,
    "user_hash": "$RANDOM_HASH"
}
CONF

# 8. 注册 systemd 守护进程
echo "[INFO] 正在配置 systemd 守护进程..."
cat << SYS > /etc/systemd/system/zray.service
[Unit]
Description=ZRay Server Proxy Service
After=network.target
Wants=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/zray-server-linux-$OS_ARCH
Restart=on-failure
RestartSec=5s
# 提升网络并发性能的文件句柄限制
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
SYS

# 9. 启动并设置开机自启
echo "[INFO] 正在启动 ZRay 服务..."
systemctl daemon-reload
systemctl enable zray >/dev/null 2>&1
systemctl restart zray

echo "========================================="
echo " ZRay 服务端已成功安装并在后台运行！"
echo "========================================="
echo "【连接信息】"
echo " 服务器 IP : $(curl -s ifconfig.me)"
echo " 监听端口  : $DEFAULT_PORT"
echo " 认证密钥  : $RANDOM_HASH"
echo "========================================="
echo "【常用管理命令】"
echo " 查看状态: systemctl status zray"
echo " 查看日志: journalctl -u zray -f"
echo " 重启服务: systemctl restart zray"
echo " 停止服务: systemctl stop zray"
echo " 修改配置: nano $INSTALL_DIR/server-config.json"
echo "========================================="
EOF

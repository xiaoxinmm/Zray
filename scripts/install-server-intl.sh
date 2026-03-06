cat << 'EOF' > install-zray.sh && bash install-zray.sh
#!/bin/bash

echo "========================================="
echo " ZRay Server 安装脚本 (国际版) - 修复重写版"
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

# 2. 稳定获取最新版本号 (绕过 GitHub API Rate Limit)
echo "[INFO] 正在获取 ZRay 最新版本号..."
LATEST_URL=$(curl -Ls -o /dev/null -w %{url_effective} https://github.com/xiaoxinmm/Zray/releases/latest)
LATEST_VERSION=${LATEST_URL##*/}

if [ -z "$LATEST_VERSION" ] || [[ "$LATEST_VERSION" == "latest" ]]; then
    echo "[ERROR] 无法获取最新版本号，可能遭遇 GitHub 网络阻断。"
    exit 1
fi
echo "[INFO] 锁定最新版本: $LATEST_VERSION"

# 3. 安全下载到临时目录
DOWNLOAD_URL="https://github.com/xiaoxinmm/Zray/releases/download/${LATEST_VERSION}/${FILE_NAME}"
echo "[INFO] 正在下载: $DOWNLOAD_URL"
curl -sL -o "/tmp/${FILE_NAME}" "$DOWNLOAD_URL"

# 4. 核心修复：强校验文件是否为真实的 gzip 压缩包
if ! gzip -t "/tmp/${FILE_NAME}" 2>/dev/null; then
    echo "[ERROR] 下载失败！拉取到的文件不是合法的压缩包 (not in gzip format)。"
    echo "[DEBUG] 拉取到的错误内容前 5 行如下："
    head -n 5 "/tmp/${FILE_NAME}"
    rm -f "/tmp/${FILE_NAME}"
    exit 1
fi

# 5. 解压与部署
echo "[INFO] 校验通过，正在部署到 /usr/local/zray ..."
mkdir -p /usr/local/zray
tar -xzf "/tmp/${FILE_NAME}" -C /usr/local/zray
rm -f "/tmp/${FILE_NAME}"

# 6. 生成证书并赋予权限
cd /usr/local/zray
chmod +x zray-server-linux-$OS_ARCH

if [ ! -f "server.crt" ]; then
    echo "[INFO] 正在生成 TLS 自签证书..."
    openssl req -x509 -newkey rsa:2048 -keyout server.key -out server.crt -days 3650 -nodes -subj "/CN=ZRay" 2>/dev/null
fi

echo "========================================="
echo "[SUCCESS] 安装大功告成！"
echo "========================================="
echo "下一步操作："
echo "1. 进入目录: cd /usr/local/zray"
echo "2. 根据需要修改配置文件 (端口、密钥等)"
echo "3. 启动服务端: ./zray-server-linux-$OS_ARCH"
EOF

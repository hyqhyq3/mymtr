#!/bin/bash
set -e

# mymtr 一键安装脚本
# 用法: curl -fsSL https://raw.githubusercontent.com/hyqhyq3/mymtr/main/install.sh | bash

REPO="hyqhyq3/mymtr"
BINARY_NAME="mymtr"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# 检测操作系统
detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *) error "不支持的操作系统: $(uname -s)" ;;
    esac
}

# 检测架构
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *) error "不支持的架构: $(uname -m)" ;;
    esac
}

# 获取最新版本
get_latest_version() {
    local latest_url="https://api.github.com/repos/${REPO}/releases/latest"
    local version

    if command -v curl &> /dev/null; then
        version=$(curl -fsSL "$latest_url" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget &> /dev/null; then
        version=$(wget -qO- "$latest_url" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        error "需要 curl 或 wget"
    fi

    if [ -z "$version" ]; then
        error "无法获取最新版本，请检查网络或访问 https://github.com/${REPO}/releases"
    fi

    echo "$version"
}

# 下载并安装
install() {
    local os=$(detect_os)
    local arch=$(detect_arch)
    local version=${VERSION:-$(get_latest_version)}
    local version_num=${version#v}  # 移除 v 前缀

    info "检测到系统: ${os}/${arch}"
    info "安装版本: ${version}"

    # 构建下载 URL
    local ext="tar.gz"
    [ "$os" = "windows" ] && ext="zip"

    local filename="${BINARY_NAME}_${version_num}_${os}_${arch}.${ext}"
    local download_url="https://github.com/${REPO}/releases/download/${version}/${filename}"

    info "下载地址: ${download_url}"

    # 创建临时目录
    local tmp_dir=$(mktemp -d)
    trap "rm -rf $tmp_dir" EXIT

    # 下载
    info "正在下载..."
    if command -v curl &> /dev/null; then
        curl -fsSL "$download_url" -o "${tmp_dir}/${filename}" || error "下载失败"
    else
        wget -q "$download_url" -O "${tmp_dir}/${filename}" || error "下载失败"
    fi

    # 解压
    info "正在解压..."
    cd "$tmp_dir"
    if [ "$ext" = "tar.gz" ]; then
        tar -xzf "$filename"
    else
        unzip -q "$filename"
    fi

    # 安装
    local binary="${BINARY_NAME}"
    [ "$os" = "windows" ] && binary="${BINARY_NAME}.exe"

    if [ ! -f "$binary" ]; then
        error "解压后未找到二进制文件"
    fi

    info "正在安装到 ${INSTALL_DIR}..."

    # 检查是否需要 sudo
    if [ -w "$INSTALL_DIR" ]; then
        mv "$binary" "${INSTALL_DIR}/${binary}"
        chmod +x "${INSTALL_DIR}/${binary}"
    else
        warn "需要管理员权限安装到 ${INSTALL_DIR}"
        sudo mv "$binary" "${INSTALL_DIR}/${binary}"
        sudo chmod +x "${INSTALL_DIR}/${binary}"
    fi

    info "安装完成！"
    info "运行 '${BINARY_NAME} --help' 查看使用说明"

    # 验证安装
    if command -v "$BINARY_NAME" &> /dev/null; then
        info "版本信息:"
        "$BINARY_NAME" --help 2>&1 | head -3 || true
    else
        warn "${INSTALL_DIR} 可能不在 PATH 中，请手动添加或使用完整路径运行"
    fi
}

# 主函数
main() {
    info "开始安装 ${BINARY_NAME}..."
    install
}

main "$@"

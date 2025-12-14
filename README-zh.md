# mymtr

[![CI](https://github.com/hyqhyq3/mymtr/actions/workflows/ci.yml/badge.svg)](https://github.com/hyqhyq3/mymtr/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/hyqhyq3/mymtr)](https://github.com/hyqhyq3/mymtr/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/hyqhyq3/mymtr)](https://go.dev/)
[![License](https://img.shields.io/github/license/hyqhyq3/mymtr)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/hyqhyq3/mymtr)](https://goreportcard.com/report/github.com/hyqhyq3/mymtr)

[English](README.md)

带 IP 地理位置解析的多跳网络诊断工具（MTR 风格）。项目基于 Go + Bubble Tea TUI 构建，可在 CLI/一次性输出与实时 TUI 间自由切换，并支持自定义 GeoIP 数据源。

## 功能亮点

- ICMP/UDP 双协议探测，支持 IPv4/IPv6
- 轮次、超时、最大跳数等探测参数可调
- GeoIP 解析：默认 `ip2region` 离线库（自动下载到用户缓存目录），也可切换为 `cip` 在线接口或完全关闭；下载源可通过 `--geoip-ip2region-url`/`MYMTR_IP2REGION_URL` 自定义
- 反向 DNS、JSON 输出、TUI 实时视图
- 可扩展的 `internal/mtr` 探测器与 `internal/geoip` 解析器

更多设计背景与模块说明见 `docs/architecture.md`、`docs/api-design.md`、`docs/technical-design.md`。

## 安装

### 一键安装（推荐）

```bash
curl -fsSL https://raw.githubusercontent.com/hyqhyq3/mymtr/main/install.sh | bash
```

自定义安装目录：

```bash
curl -fsSL https://raw.githubusercontent.com/hyqhyq3/mymtr/main/install.sh | INSTALL_DIR=~/.local/bin bash
```

安装指定版本：

```bash
curl -fsSL https://raw.githubusercontent.com/hyqhyq3/mymtr/main/install.sh | VERSION=v0.1.0 bash
```

### 从 Release 下载

访问 [Releases](https://github.com/hyqhyq3/mymtr/releases) 页面下载对应平台的预编译二进制文件。

支持的平台：
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64, arm64)

### 从源码构建

```bash
git clone https://github.com/hyqhyq3/mymtr.git
cd mymtr

# 构建/测试
make build
make test

# 命令帮助
go run ./cmd/mymtr --help
```

典型用法（一次性输出模式）：

```bash
mymtr example.com --count 20 --interval 500ms --protocol udp --no-tui
```

## 自动化构建

仓库内置 GitHub Actions（`.github/workflows/ci.yml`），在 `main` 和 Pull Request 上自动完成：

1. 设置 Go 1.24 环境与缓存
2. 执行 `go test ./...`
3. 执行 `go build ./...`

## GeoIP 数据源说明

- `cip`：在线接口，带缓存，适合即时查询。
- `ip2region`（默认）：离线库缓存在用户缓存目录（例如 macOS 的 `~/Library/Caches/mymtr/ip2region.xdb`、Linux 的 `~/.cache/mymtr/ip2region.xdb`、Windows 的 `%LocalAppData%\\mymtr\\ip2region.xdb`）。若自动下载失败，可：
  - 显式指定文件路径 `--ip2region-db path/to/db`
  - 使用 `--geoip-ip2region-url <URL>` 或环境变量 `MYMTR_IP2REGION_URL` 指向自建镜像
  - 在非交互场景通过 `--geoip-download=yes`（或 `no`）提前应答下载提示

## 致谢

项目在构建过程中受益于以下优秀的开源项目与资源：
- [lionsoul2014/ip2region](https://github.com/lionsoul2014/ip2region)：提供高性能的 IP 数据库。
- [Charmbracelet Bubble Tea](https://github.com/charmbracelet/bubbletea) 及其生态：支撑 TUI 界面。
- [spf13/cobra](https://github.com/spf13/cobra)：提供命令行框架。

## 许可证

本项目遵循 MIT License，详见 `LICENSE`。

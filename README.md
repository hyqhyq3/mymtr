# mymtr

带 IP 地理位置解析的多跳网络诊断工具（MTR 风格）。项目基于 Go + Bubble Tea TUI 构建，可在 CLI/一次性输出与实时 TUI 间自由切换，并支持自定义 GeoIP 数据源。

## 功能亮点

- ICMP/UDP 双协议探测，支持 IPv4/IPv6
- 轮次、超时、最大跳数等探测参数可调
- GeoIP 解析：`cip` 在线接口、`ip2region` 离线数据库或完全关闭
- 反向 DNS、JSON 输出、TUI 实时视图
- 可扩展的 `internal/mtr` 探测器与 `internal/geoip` 解析器

更多设计背景与模块说明见 `docs/architecture.md`、`docs/api-design.md`、`docs/technical-design.md`。

## 快速开始

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
mymtr example.com --count 20 --interval 500ms --protocol udp --geoip ip2region --ip2region-db data/ip2region.xdb --no-tui
```

## 自动化构建

仓库内置 GitHub Actions（`.github/workflows/ci.yml`），在 `main` 和 Pull Request 上自动完成：

1. 设置 Go 1.24 环境与缓存
2. 执行 `go test ./...`
3. 执行 `go build ./...`

## 许可证

本项目遵循 MIT License，详见 `LICENSE`。

# mymtr

[![CI](https://github.com/hyqhyq3/mymtr/actions/workflows/ci.yml/badge.svg)](https://github.com/hyqhyq3/mymtr/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/hyqhyq3/mymtr)](https://github.com/hyqhyq3/mymtr/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/hyqhyq3/mymtr)](https://go.dev/)
[![License](https://img.shields.io/github/license/hyqhyq3/mymtr)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/hyqhyq3/mymtr)](https://goreportcard.com/report/github.com/hyqhyq3/mymtr)

[中文文档](README-zh.md)

A network diagnostic tool with IP geolocation (MTR-style). Built with Go + Bubble Tea TUI, supporting both CLI one-shot output and real-time TUI modes with customizable GeoIP data sources.

## Features

- ICMP/UDP dual-protocol probing with IPv4/IPv6 support
- Configurable probe parameters: rounds, timeout, max hops
- GeoIP resolution: `cip` online API, `ip2region` offline database, or disabled; auto-download ip2region database supported (`--geoip-auto-download`; customizable via `--geoip-ip2region-url` or `MYMTR_IP2REGION_URL`)
- Reverse DNS, JSON output, real-time TUI view
- Extensible `internal/mtr` probers and `internal/geoip` resolvers

For design background and module details, see `docs/architecture.md`, `docs/api-design.md`, `docs/technical-design.md`.

## Installation

### One-line Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/hyqhyq3/mymtr/main/install.sh | bash
```

Custom install directory:

```bash
curl -fsSL https://raw.githubusercontent.com/hyqhyq3/mymtr/main/install.sh | INSTALL_DIR=~/.local/bin bash
```

Install specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/hyqhyq3/mymtr/main/install.sh | VERSION=v0.1.0 bash
```

### Download from Release

Visit the [Releases](https://github.com/hyqhyq3/mymtr/releases) page to download pre-built binaries for your platform.

Supported platforms:
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64, arm64)

### Build from Source

```bash
git clone https://github.com/hyqhyq3/mymtr.git
cd mymtr

# Build/Test
make build
make test

# Help
go run ./cmd/mymtr --help
```

Typical usage (one-shot output mode):

```bash
mymtr example.com --count 20 --interval 500ms --protocol udp --geoip ip2region --ip2region-db data/ip2region.xdb --no-tui
```

## Internationalization (i18n)

mymtr supports automatic language detection based on your system locale. The following languages are supported:

- English (default)
- Chinese (Simplified)

The language is automatically selected based on your system's `LANG`, `LC_ALL`, `LC_MESSAGES`, or `LANGUAGE` environment variables.

## CI/CD

The repository includes GitHub Actions (`.github/workflows/ci.yml`) that automatically:

1. Set up Go 1.24 environment with caching
2. Run `go test ./...`
3. Run `go build ./...`

## GeoIP Data Sources

- `cip`: Default online API with caching, suitable for instant queries.
- `ip2region`: Requires local `.xdb` file, auto-download available on first run. If download fails:
  - Specify file path explicitly with `--ip2region-db path/to/db`
  - Use `--geoip-ip2region-url <URL>` or `MYMTR_IP2REGION_URL` environment variable to point to a custom mirror
  - Disable auto-download with `--geoip-auto-download=false` and provide the file manually

## License

This project is licensed under the MIT License. See `LICENSE` for details.

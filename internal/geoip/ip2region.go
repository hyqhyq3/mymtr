package geoip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

const (
	defaultIP2RegionURL       = "https://github.com/lionsoul2014/ip2region/raw/master/data/ip2region.xdb"
	ip2RegionDownloadTimeout  = 2 * time.Minute
	ip2RegionDefaultUserAgent = "mymtr/geoip-downloader"
)

var (
	ip2RegionDownloadURL = defaultIP2RegionURL
	ip2RegionHTTPClient  = &http.Client{Timeout: 30 * time.Second}
)

type IP2RegionResolver struct {
	dbPath   string
	searcher *xdb.Searcher
}

func NewIP2RegionResolver(dbPath string, autoDownload bool) (*IP2RegionResolver, error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return nil, errors.New("ip2region db 路径为空（请设置 --ip2region-db）")
	}

	if err := ensureIP2RegionDB(dbPath, autoDownload); err != nil {
		return nil, err
	}

	version, err := detectIPVersion(dbPath)
	if err != nil {
		return nil, err
	}

	searcher, err := xdb.NewWithFileOnly(version, dbPath)
	if err != nil {
		return nil, err
	}
	return &IP2RegionResolver{dbPath: dbPath, searcher: searcher}, nil
}

func (r *IP2RegionResolver) Source() string { return "ip2region" }

func (r *IP2RegionResolver) Close() error {
	if r.searcher == nil {
		return nil
	}
	r.searcher.Close()
	r.searcher = nil
	return nil
}

func (r *IP2RegionResolver) Resolve(ip net.IP) *GeoLocation {
	if ip == nil || r.searcher == nil {
		return nil
	}
	// ip2region v2 仅支持 IPv4
	if ip4 := ip.To4(); ip4 == nil {
		return nil
	}

	region, err := r.searcher.SearchByStr(ip.String())
	if err != nil || strings.TrimSpace(region) == "" {
		return nil
	}

	loc := parseIP2Region(region)
	if loc == nil {
		return nil
	}
	loc.Source = r.Source()
	loc.Raw = region
	return loc
}

func ensureIP2RegionDB(dbPath string, autoDownload bool) error {
	info, err := os.Stat(dbPath)
	if err == nil {
		if info.IsDir() {
			return fmt.Errorf("ip2region db 路径应为文件：%s", dbPath)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("ip2region db 不可用：%w", err)
	}
	if !autoDownload {
		return fmt.Errorf("ip2region db 不存在：%s（可启用 --geoip-auto-download 自动下载）", dbPath)
	}
	if err := downloadIP2RegionDB(dbPath); err != nil {
		return fmt.Errorf("自动下载 ip2region db 失败：%w", err)
	}
	return nil
}

func downloadIP2RegionDB(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("创建目录失败：%w", err)
		}
	}

	tmp := dbPath + ".download"
	if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), ip2RegionDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ip2RegionDownloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", ip2RegionDefaultUserAgent)

	resp, err := ip2RegionHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("下载 ip2region db 失败，状态码：%d", resp.StatusCode)
	}

	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	if err := os.Rename(tmp, dbPath); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func detectIPVersion(dbPath string) (*xdb.Version, error) {
	handle, err := os.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开 ip2region db 失败：%w", err)
	}
	defer handle.Close()

	if err := xdb.Verify(handle); err != nil {
		return nil, fmt.Errorf("ip2region db 校验失败：%w", err)
	}

	header, err := xdb.LoadHeader(handle)
	if err != nil {
		return nil, fmt.Errorf("读取 ip2region header 失败：%w", err)
	}

	version, err := xdb.VersionFromHeader(header)
	if err != nil {
		return nil, fmt.Errorf("解析 ip2region 版本失败：%w", err)
	}
	return version, nil
}

// region 格式通常为：国家|区域|省份|城市|ISP（未知项可能为 0）
func parseIP2Region(region string) *GeoLocation {
	parts := strings.Split(region, "|")
	if len(parts) < 5 {
		return nil
	}
	country := normalizeIP2R(parts[0])
	province := normalizeIP2R(parts[2])
	city := normalizeIP2R(parts[3])
	isp := normalizeIP2R(parts[4])
	if country == "" && province == "" && city == "" && isp == "" {
		return nil
	}
	return &GeoLocation{
		Country:  country,
		Province: province,
		City:     city,
		ISP:      isp,
	}
}

func normalizeIP2R(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return ""
	}
	return s
}

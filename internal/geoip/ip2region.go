package geoip

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

type IP2RegionResolver struct {
	dbPath  string
	searcher *xdb.Searcher
}

func NewIP2RegionResolver(dbPath string) (*IP2RegionResolver, error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return nil, errors.New("ip2region db 路径为空（请设置 --ip2region-db）")
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("ip2region db 不可用：%w", err)
	}
	searcher, err := xdb.NewWithFileOnly(dbPath)
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
	return r.searcher.Close()
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


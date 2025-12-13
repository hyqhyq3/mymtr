package geoip

import (
	"fmt"
	"net"
	"strings"
)

// GeoResolver IP 地理位置解析器接口。
// 解析失败时应返回 nil（不返回 error），调用方需按“无位置信息”处理。
type GeoResolver interface {
	Resolve(ip net.IP) *GeoLocation
	Source() string
	Close() error
}

type GeoLocation struct {
	Country  string `json:"country,omitempty"`
	Province string `json:"province,omitempty"`
	City     string `json:"city,omitempty"`
	ISP      string `json:"isp,omitempty"`
	Source   string `json:"source,omitempty"`
	Raw      string `json:"raw,omitempty"`
}

func (g *GeoLocation) String() string {
	if g == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	if g.Country != "" && g.Country != "0" {
		parts = append(parts, g.Country)
	}
	if g.Province != "" && g.Province != "0" {
		parts = append(parts, g.Province)
	}
	if g.City != "" && g.City != "0" {
		parts = append(parts, g.City)
	}
	if g.ISP != "" && g.ISP != "0" {
		parts = append(parts, g.ISP)
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	if raw := strings.TrimSpace(g.Raw); raw != "" {
		return sanitizeRawLocation(raw)
	}
	if g.Source != "" {
		return fmt.Sprintf("[%s]", g.Source)
	}
	return ""
}

func sanitizeRawLocation(raw string) string {
	// ip2region raw usually split by '|'
	raw = strings.ReplaceAll(raw, "|", " ")
	parts := strings.Fields(raw)
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" || p == "0" {
			continue
		}
		filtered = append(filtered, p)
	}
	if len(filtered) == 0 {
		return raw
	}
	return strings.Join(filtered, " ")
}

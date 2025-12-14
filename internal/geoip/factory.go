package geoip

import (
	"fmt"
	"strings"
)

type Options struct {
	IP2RegionDB  string
	IP2RegionURL string
	Download     DownloadOption
}

func NewResolver(source string, opts Options) (GeoResolver, error) {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "", "none", "noop", "off":
		return NewNoopResolver(), nil
	case "cip", "cip.cc":
		return NewCIPResolver(), nil
	case "ip2region":
		return NewIP2RegionResolver(opts.IP2RegionDB, opts.IP2RegionURL, opts.Download)
	default:
		return nil, fmt.Errorf("未知 geoip source：%s", source)
	}
}

package geoip

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type CIPResolver struct {
	baseURL string
	client  *http.Client

	mu    sync.Mutex
	cache map[string]cacheEntry

	ttlSuccess time.Duration
	ttlFailure time.Duration
	maxSize    int
}

type cacheEntry struct {
	loc      *GeoLocation
	expires  time.Time
	lastUsed time.Time
}

func NewCIPResolver() *CIPResolver {
	return &CIPResolver{
		baseURL: "https://cip.cc",
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
		cache:      make(map[string]cacheEntry, 2048),
		ttlSuccess: 24 * time.Hour,
		ttlFailure: 5 * time.Minute,
		maxSize:    5000,
	}
}

func (r *CIPResolver) Source() string { return "cip.cc" }

func (r *CIPResolver) Close() error { return nil }

func (r *CIPResolver) Resolve(ip net.IP) *GeoLocation {
	if ip == nil {
		return nil
	}
	key := ip.String()

	now := time.Now()
	if loc, ok := r.getCached(now, key); ok {
		return loc
	}

	loc := r.fetchAndParse(context.Background(), key)
	r.setCached(now, key, loc)
	return loc
}

func (r *CIPResolver) getCached(now time.Time, key string) (*GeoLocation, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ent, ok := r.cache[key]
	if !ok {
		return nil, false
	}
	if now.After(ent.expires) {
		delete(r.cache, key)
		return nil, false
	}
	ent.lastUsed = now
	r.cache[key] = ent
	return ent.loc, true
}

func (r *CIPResolver) setCached(now time.Time, key string, loc *GeoLocation) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.cache) >= r.maxSize {
		r.evict(now)
	}
	ttl := r.ttlSuccess
	if loc == nil {
		ttl = r.ttlFailure
	}
	r.cache[key] = cacheEntry{
		loc:      loc,
		expires:  now.Add(ttl),
		lastUsed: now,
	}
}

func (r *CIPResolver) evict(now time.Time) {
	// 先清理过期，再按近似 LRU 删除一批
	for k, ent := range r.cache {
		if now.After(ent.expires) {
			delete(r.cache, k)
		}
	}
	if len(r.cache) < r.maxSize {
		return
	}

	type kv struct {
		k string
		t time.Time
	}
	items := make([]kv, 0, len(r.cache))
	for k, ent := range r.cache {
		items = append(items, kv{k: k, t: ent.lastUsed})
	}
	// 删除最老的 10%
	n := len(items) / 10
	if n < 1 {
		n = 1
	}
	// 选择 n 个最小 lastUsed
	for i := 0; i < n; i++ {
		min := i
		for j := i + 1; j < len(items); j++ {
			if items[j].t.Before(items[min].t) {
				min = j
			}
		}
		items[i], items[min] = items[min], items[i]
		delete(r.cache, items[i].k)
	}
}

func (r *CIPResolver) fetchAndParse(ctx context.Context, ip string) *GeoLocation {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s", r.baseURL, ip), nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "mymtr/1.0")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil
	}
	loc, err := parseCIP(string(body))
	if err != nil {
		return nil
	}
	loc.Source = r.Source()
	loc.Raw = strings.TrimSpace(string(body))
	return loc
}

func parseCIP(s string) (*GeoLocation, error) {
	// 典型格式（tab 分隔）：
	// IP	: 8.8.8.8
	// 地址	: 美国 加利福尼亚州 圣克拉拉
	// 运营商	: CloudFlare公共DNS服务器
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 1024), 64*1024)

	var address, isp string
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		k, v, ok := splitKV(line)
		if !ok {
			continue
		}
		switch k {
		case "地址":
			address = v
		case "运营商":
			isp = v
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if address == "" && isp == "" {
		return nil, errors.New("no fields")
	}

	// 地址字段通常用空格分隔：国家 省/州 城市
	parts := strings.Fields(address)
	loc := &GeoLocation{
		ISP: isp,
	}
	if len(parts) > 0 {
		loc.Country = parts[0]
	}
	if len(parts) > 1 {
		loc.Province = parts[1]
	}
	if len(parts) > 2 {
		loc.City = strings.Join(parts[2:], " ")
	}
	return loc, nil
}

func splitKV(line string) (key, value string, ok bool) {
	// 兼容 "地址\t: xxx" / "地址 : xxx" / "地址: xxx"
	if i := strings.Index(line, ":"); i >= 0 {
		key = strings.TrimSpace(strings.ReplaceAll(line[:i], "\t", " "))
		value = strings.TrimSpace(line[i+1:])
		key = strings.TrimSpace(key)
		if key == "" {
			return "", "", false
		}
		return key, value, true
	}
	return "", "", false
}

package geoip

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestParseIP2Region(t *testing.T) {
	loc := parseIP2Region("中国|0|上海|上海|电信")
	if loc == nil {
		t.Fatalf("expected non-nil")
	}
	if loc.Country != "中国" || loc.Province != "上海" || loc.City != "上海" || loc.ISP != "电信" {
		t.Fatalf("unexpected: %#v", loc)
	}
}

func TestDownloadIP2RegionDB(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "fake-xdb-content")
	}))
	t.Cleanup(srv.Close)

	originalURL := ip2RegionDownloadURL
	ip2RegionDownloadURL = srv.URL
	t.Cleanup(func() { ip2RegionDownloadURL = originalURL })

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "ip2region.xdb")
	if err := downloadIP2RegionDB(target); err != nil {
		t.Fatalf("download failed: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "fake-xdb-content" {
		t.Fatalf("unexpected file content: %s", data)
	}
}

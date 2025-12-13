package geoip

import (
	"fmt"
	"io"
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

func TestDownloadIP2RegionDBWithCustomURL(t *testing.T) {
	t.Parallel()

	origWriter := progressOutput
	progressOutput = io.Discard
	t.Cleanup(func() { progressOutput = origWriter })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "fake-xdb-content")
	}))
	t.Cleanup(srv.Close)

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "ip2region.xdb")
	if err := downloadIP2RegionDB(target, srv.URL); err != nil {
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

func TestDownloadIP2RegionDBFallback(t *testing.T) {
	t.Parallel()

	origWriter := progressOutput
	progressOutput = io.Discard
	t.Cleanup(func() { progressOutput = origWriter })

	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "nope")
	}))
	successSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	}))
	t.Cleanup(failSrv.Close)
	t.Cleanup(successSrv.Close)

	origSources := ip2RegionDownloadSources
	ip2RegionDownloadSources = []string{failSrv.URL, successSrv.URL}
	t.Cleanup(func() { ip2RegionDownloadSources = origSources })

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "ip2region.xdb")
	if err := downloadIP2RegionDB(target, ""); err != nil {
		t.Fatalf("download with fallback failed: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "ok" {
		t.Fatalf("unexpected file content: %s", data)
	}
}

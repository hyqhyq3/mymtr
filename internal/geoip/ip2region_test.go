package geoip

import "testing"

func TestParseIP2Region(t *testing.T) {
	loc := parseIP2Region("中国|0|上海|上海|电信")
	if loc == nil {
		t.Fatalf("expected non-nil")
	}
	if loc.Country != "中国" || loc.Province != "上海" || loc.City != "上海" || loc.ISP != "电信" {
		t.Fatalf("unexpected: %#v", loc)
	}
}

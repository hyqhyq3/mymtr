package geoip

import "testing"

func TestGeoLocationStringFallback(t *testing.T) {
	loc := &GeoLocation{
		Raw:    "0|0|0|内网IP|内网IP",
		Source: "ip2region",
	}
	if got := loc.String(); got != "内网IP 内网IP" {
		t.Fatalf("unexpected raw fallback: %q", got)
	}

	locEmpty := &GeoLocation{Source: "cip"}
	want := "[cip]"
	if got := locEmpty.String(); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

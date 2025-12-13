package geoip

import "testing"

func TestParseCIP_US(t *testing.T) {
	in := "IP\t: 8.8.8.8\n地址\t: 美国 加利福尼亚州 圣克拉拉\n\n数据二\t: 美国加利福尼亚州圣克拉拉 | 谷歌公司DNS服务器\n"
	loc, err := parseCIP(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if loc.Country != "美国" || loc.Province != "加利福尼亚州" || loc.City != "圣克拉拉" {
		t.Fatalf("unexpected location: %#v", loc)
	}
}

func TestParseCIP_WithISP(t *testing.T) {
	in := "IP\t: 1.1.1.1\n地址\t: 澳大利亚\n运营商\t: CloudFlare公共DNS服务器\n"
	loc, err := parseCIP(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if loc.Country != "澳大利亚" {
		t.Fatalf("unexpected country: %#v", loc)
	}
	if loc.ISP != "CloudFlare公共DNS服务器" {
		t.Fatalf("unexpected isp: %#v", loc)
	}
}

func TestParseCIP_CN(t *testing.T) {
	in := "IP\t: 59.111.160.244\n地址\t: 中国 浙江 杭州\n运营商\t: 网易\n"
	loc, err := parseCIP(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if loc.Country != "中国" || loc.Province != "浙江" || loc.City != "杭州" || loc.ISP != "网易" {
		t.Fatalf("unexpected location: %#v", loc)
	}
	if got := loc.String(); got != "中国 浙江 杭州 网易" {
		t.Fatalf("unexpected string: %q", got)
	}
}

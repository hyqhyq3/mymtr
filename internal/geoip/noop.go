package geoip

import "net"

type NoopResolver struct{}

func NewNoopResolver() *NoopResolver { return &NoopResolver{} }

func (r *NoopResolver) Resolve(ip net.IP) *GeoLocation { return nil }

func (r *NoopResolver) Source() string { return "noop" }

func (r *NoopResolver) Close() error { return nil }

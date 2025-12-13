package mtr

import "time"

type Config struct {
	Target    string
	TargetIP  string
	MaxHops   int
	Count     int
	Interval  time.Duration
	Timeout   time.Duration
	Protocol  Protocol
	IPVersion int
	EnableDNS bool
}

type Protocol string

const (
	ProtocolICMP Protocol = "icmp"
	ProtocolUDP  Protocol = "udp"
)

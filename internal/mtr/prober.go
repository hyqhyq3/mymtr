package mtr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

type Prober interface {
	Probe(ctx context.Context, ttl int, seq int) (*ProbeResult, error)
	SetTarget(ip net.IP) error
	Close() error
}

type ProbeResult struct {
	TTL       int
	Seq       int
	IP        net.IP
	RTT       time.Duration
	Type      ResponseType
	Timestamp time.Time
}

type ResponseType int

const (
	ResponseTypeTimeout ResponseType = iota
	ResponseTypeEchoReply
	ResponseTypeTimeExceeded
	ResponseTypeDestUnreach
)

func NewProber(protocol Protocol, ipVersion int, timeout time.Duration) (Prober, error) {
	switch protocol {
	case ProtocolICMP:
		return NewICMPProber(ipVersion, timeout)
	case ProtocolUDP:
		return NewUDPProber(ipVersion, timeout)
	default:
		return nil, fmt.Errorf("未知 protocol：%s", protocol)
	}
}

var ErrProtocolNotImplemented = errors.New("协议暂未实现")

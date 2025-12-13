package mtr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type ICMPProber struct {
	ipVersion int
	timeout   time.Duration

	conn   *icmp.PacketConn
	target net.IP
	id     int

	payload []byte
}

func NewICMPProber(ipVersion int, timeout time.Duration) (*ICMPProber, error) {
	if timeout <= 0 {
		timeout = time.Second
	}
	network := "ip4:icmp"
	addr := "0.0.0.0"
	if ipVersion == 6 {
		network = "ip6:ipv6-icmp"
		addr = "::"
	}

	conn, err := icmp.ListenPacket(network, addr)
	if err != nil {
		if looksLikePermission(err) {
			return nil, fmt.Errorf("创建原始套接字失败（需要更高权限运行）：%w", err)
		}
		return nil, err
	}

	p := &ICMPProber{
		ipVersion: ipVersion,
		timeout:   timeout,
		conn:      conn,
		id:        os.Getpid() & 0xffff,
		payload:   []byte("mymtr"),
	}
	return p, nil
}

func (p *ICMPProber) SetTarget(ip net.IP) error {
	if ip == nil {
		return errors.New("target ip 不能为空")
	}
	p.target = ip
	return nil
}

func (p *ICMPProber) Close() error {
	if p.conn == nil {
		return nil
	}
	return p.conn.Close()
}

func (p *ICMPProber) Probe(ctx context.Context, ttl int, seq int) (*ProbeResult, error) {
	if p.target == nil {
		return nil, errors.New("尚未设置 target ip")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	now := time.Now()
	if err := p.setTTL(ttl); err != nil {
		return nil, err
	}

	msg, proto, err := p.echoMessage(seq)
	if err != nil {
		return nil, err
	}
	b, err := msg.Marshal(nil)
	if err != nil {
		return nil, err
	}

	if _, err := p.conn.WriteTo(b, &net.IPAddr{IP: p.target}); err != nil {
		return nil, err
	}

	deadline := now.Add(p.timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}

	_ = p.conn.SetReadDeadline(deadline)
	unblock := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = p.conn.SetReadDeadline(time.Now())
		case <-unblock:
		}
	}()
	defer close(unblock)

	buf := make([]byte, 1500)
	for {
		n, peer, err := p.conn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return &ProbeResult{
					TTL:       ttl,
					Seq:       seq,
					Type:      ResponseTypeTimeout,
					Timestamp: now,
				}, nil
			}
			if isTimeout(err) {
				return &ProbeResult{
					TTL:       ttl,
					Seq:       seq,
					Type:      ResponseTypeTimeout,
					Timestamp: now,
				}, nil
			}
			return nil, err
		}

		rm, err := icmp.ParseMessage(proto, buf[:n])
		if err != nil {
			continue
		}

		typ := p.classifyReply(proto, rm, seq)
		switch typ {
		case ResponseTypeEchoReply, ResponseTypeTimeExceeded:
			ip := extractPeerIP(peer)
			return &ProbeResult{
				TTL:       ttl,
				Seq:       seq,
				IP:        ip,
				RTT:       time.Since(now),
				Type:      typ,
				Timestamp: now,
			}, nil
		default:
			continue
		}
	}
}

func (p *ICMPProber) setTTL(ttl int) error {
	if ttl <= 0 {
		ttl = 1
	}
	if p.ipVersion == 4 {
		return p.conn.IPv4PacketConn().SetTTL(ttl)
	}
	return p.conn.IPv6PacketConn().SetHopLimit(ttl)
}

func (p *ICMPProber) echoMessage(seq int) (icmp.Message, int, error) {
	if p.ipVersion == 4 {
		return icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: &icmp.Echo{ID: p.id, Seq: seq, Data: p.payload},
		}, 1, nil
	}
	return icmp.Message{
		Type: ipv6.ICMPTypeEchoRequest,
		Code: 0,
		Body: &icmp.Echo{ID: p.id, Seq: seq, Data: p.payload},
	}, 58, nil
}

func (p *ICMPProber) classifyReply(proto int, rm *icmp.Message, seq int) ResponseType {
	if rm == nil {
		return ResponseTypeTimeout
	}

	switch rm.Type {
	case ipv4.ICMPTypeEchoReply, ipv6.ICMPTypeEchoReply:
		if echo, ok := rm.Body.(*icmp.Echo); ok && echo.ID == p.id && echo.Seq == seq {
			return ResponseTypeEchoReply
		}
	case ipv4.ICMPTypeTimeExceeded, ipv6.ICMPTypeTimeExceeded:
		if p.matchesQuoted(proto, rm.Body, seq) {
			return ResponseTypeTimeExceeded
		}
	}
	return ResponseTypeTimeout
}

func (p *ICMPProber) matchesQuoted(proto int, body icmp.MessageBody, seq int) bool {
	var data []byte
	switch b := body.(type) {
	case *icmp.TimeExceeded:
		data = b.Data
	default:
		return false
	}
	if len(data) == 0 {
		return false
	}

	if p.ipVersion == 4 {
		h, err := ipv4.ParseHeader(data)
		if err != nil || h.Len <= 0 || len(data) < h.Len+8 {
			return false
		}
		inner, err := icmp.ParseMessage(proto, data[h.Len:])
		if err != nil {
			return false
		}
		echo, ok := inner.Body.(*icmp.Echo)
		return ok && echo.ID == p.id && echo.Seq == seq
	}

	if _, err := ipv6.ParseHeader(data); err != nil {
		return false
	}
	const ipv6HeaderLen = 40
	if len(data) < ipv6HeaderLen+8 {
		return false
	}
	inner, err := icmp.ParseMessage(proto, data[ipv6HeaderLen:])
	if err != nil {
		return false
	}
	echo, ok := inner.Body.(*icmp.Echo)
	return ok && echo.ID == p.id && echo.Seq == seq
}

func extractPeerIP(peer net.Addr) net.IP {
	if peer == nil {
		return nil
	}
	if ipAddr, ok := peer.(*net.IPAddr); ok {
		return ipAddr.IP
	}
	if udpAddr, ok := peer.(*net.UDPAddr); ok {
		return udpAddr.IP
	}
	return nil
}

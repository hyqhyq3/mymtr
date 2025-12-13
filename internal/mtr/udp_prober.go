package mtr

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type UDPProber struct {
	ipVersion int
	timeout   time.Duration
	target    net.IP

	icmpConn  *icmp.PacketConn
	basePort  int
	localAddr net.IP
}

func NewUDPProber(ipVersion int, timeout time.Duration) (*UDPProber, error) {
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

	return &UDPProber{
		ipVersion: ipVersion,
		timeout:   timeout,
		icmpConn:  conn,
		basePort:  33434,
	}, nil
}

func (p *UDPProber) SetTarget(ip net.IP) error {
	if ip == nil {
		return errors.New("target ip 不能为空")
	}
	p.target = ip
	return nil
}

func (p *UDPProber) Close() error {
	if p.icmpConn == nil {
		return nil
	}
	return p.icmpConn.Close()
}

func (p *UDPProber) Probe(ctx context.Context, ttl int, seq int) (*ProbeResult, error) {
	if p.target == nil {
		return nil, errors.New("尚未设置 target ip")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	destPort := p.basePort + (seq % 10000)
	udpConn, localPort, err := p.dialUDP(destPort)
	if err != nil {
		return nil, err
	}
	defer udpConn.Close()

	if err := p.setUDPTTL(udpConn, ttl); err != nil {
		return nil, err
	}

	payload := make([]byte, 8)
	copy(payload[:4], []byte("mymt"))
	binary.BigEndian.PutUint32(payload[4:], uint32(seq))

	start := time.Now()
	if _, err := udpConn.Write(payload); err != nil {
		return nil, err
	}

	deadline := start.Add(p.timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = p.icmpConn.SetReadDeadline(deadline)
	unblock := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = p.icmpConn.SetReadDeadline(time.Now())
		case <-unblock:
		}
	}()
	defer close(unblock)

	proto := 1
	if p.ipVersion == 6 {
		proto = 58
	}

	buf := make([]byte, 1500)
	for {
		n, peer, err := p.icmpConn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return &ProbeResult{
					TTL:       ttl,
					Seq:       seq,
					Type:      ResponseTypeTimeout,
					Timestamp: start,
				}, nil
			}
			if isTimeout(err) {
				return &ProbeResult{
					TTL:       ttl,
					Seq:       seq,
					Type:      ResponseTypeTimeout,
					Timestamp: start,
				}, nil
			}
			return nil, err
		}

		rm, err := icmp.ParseMessage(proto, buf[:n])
		if err != nil {
			continue
		}

		typ, ok := p.classifyUDPReply(rm, localPort, destPort)
		if !ok {
			continue
		}

		return &ProbeResult{
			TTL:       ttl,
			Seq:       seq,
			IP:        extractPeerIP(peer),
			RTT:       time.Since(start),
			Type:      typ,
			Timestamp: start,
		}, nil
	}
}

func (p *UDPProber) dialUDP(destPort int) (*net.UDPConn, int, error) {
	network := "udp4"
	if p.ipVersion == 6 {
		network = "udp6"
	}
	raddr := &net.UDPAddr{IP: p.target, Port: destPort}
	conn, err := net.DialUDP(network, nil, raddr)
	if err != nil {
		return nil, 0, err
	}
	localPort := 0
	if la, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		localPort = la.Port
	}
	return conn, localPort, nil
}

func (p *UDPProber) setUDPTTL(conn *net.UDPConn, ttl int) error {
	if ttl <= 0 {
		ttl = 1
	}
	if p.ipVersion == 4 {
		return ipv4.NewPacketConn(conn).SetTTL(ttl)
	}
	return ipv6.NewPacketConn(conn).SetHopLimit(ttl)
}

func (p *UDPProber) classifyUDPReply(rm *icmp.Message, localPort, destPort int) (ResponseType, bool) {
	if rm == nil {
		return ResponseTypeTimeout, false
	}

	switch rm.Type {
	case ipv4.ICMPTypeTimeExceeded, ipv6.ICMPTypeTimeExceeded:
		if p.matchesQuotedUDP(rm.Body, localPort, destPort) {
			return ResponseTypeTimeExceeded, true
		}
	case ipv4.ICMPTypeDestinationUnreachable, ipv6.ICMPTypeDestinationUnreachable:
		if !p.matchesQuotedUDP(rm.Body, localPort, destPort) {
			return ResponseTypeTimeout, false
		}

		// 到达目标时，UDP traceroute 通常会收到“端口不可达”，这里映射为 EchoReply 以便 Controller 提前结束。
		if isPortUnreachable(rm) {
			return ResponseTypeEchoReply, true
		}
		return ResponseTypeDestUnreach, true
	}

	return ResponseTypeTimeout, false
}

func (p *UDPProber) matchesQuotedUDP(body icmp.MessageBody, localPort, destPort int) bool {
	var data []byte
	switch b := body.(type) {
	case *icmp.TimeExceeded:
		data = b.Data
	case *icmp.DstUnreach:
		data = b.Data
	default:
		return false
	}
	if len(data) == 0 {
		return false
	}

	udpHeader, ok := extractQuotedTransport(data, p.ipVersion)
	if !ok || len(udpHeader) < 8 {
		return false
	}
	src := int(binary.BigEndian.Uint16(udpHeader[0:2]))
	dst := int(binary.BigEndian.Uint16(udpHeader[2:4]))

	if destPort != 0 && dst != destPort {
		return false
	}
	// localPort 在极少数平台下可能读不到，读不到时不作为强校验。
	if localPort != 0 && src != localPort {
		return false
	}
	return true
}

func extractQuotedTransport(data []byte, ipVersion int) ([]byte, bool) {
	if ipVersion == 4 {
		h, err := ipv4.ParseHeader(data)
		if err != nil || h.Len <= 0 || len(data) < h.Len+8 {
			return nil, false
		}
		return data[h.Len:], true
	}

	// IPv6 header 固定 40 字节（忽略 extension header 的复杂性，MVP 足够）
	const ipv6HeaderLen = 40
	if len(data) < ipv6HeaderLen+8 {
		return nil, false
	}
	return data[ipv6HeaderLen:], true
}

func isPortUnreachable(rm *icmp.Message) bool {
	if rm == nil {
		return false
	}
	switch rm.Type {
	case ipv4.ICMPTypeDestinationUnreachable:
		// IPv4 code=3: port unreachable
		return rm.Code == 3
	case ipv6.ICMPTypeDestinationUnreachable:
		// IPv6 code=4: port unreachable
		return rm.Code == 4
	default:
		return false
	}
}

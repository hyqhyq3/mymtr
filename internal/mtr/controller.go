package mtr

import (
	"context"
	"errors"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hyqhyq3/mymtr/internal/geoip"
	"github.com/hyqhyq3/mymtr/internal/i18n"
)

type Controller struct {
	config   *Config
	prober   Prober
	resolver geoip.GeoResolver

	mu     sync.RWMutex
	hops   map[int]*Hop
	events chan Event
}

func NewController(cfg *Config, prober Prober, resolver geoip.GeoResolver) (*Controller, error) {
	if cfg == nil {
		return nil, errors.New(i18n.T("err.cfgEmpty"))
	}
	if prober == nil {
		return nil, errors.New(i18n.T("err.proberEmpty"))
	}
	if strings.TrimSpace(cfg.Target) == "" {
		return nil, errors.New(i18n.T("err.targetEmpty"))
	}
	if cfg.MaxHops <= 0 {
		cfg.MaxHops = 30
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = time.Second
	}
	if cfg.Interval <= 0 {
		cfg.Interval = time.Second
	}
	if cfg.IPVersion != 4 && cfg.IPVersion != 6 {
		return nil, errors.New(i18n.Tf("err.ipVersionInvalid", map[string]interface{}{"Version": cfg.IPVersion}))
	}
	if cfg.Protocol == "" {
		cfg.Protocol = ProtocolICMP
	}

	return &Controller{
		config:   cfg,
		prober:   prober,
		resolver: resolver,
		hops:     make(map[int]*Hop, cfg.MaxHops),
		events:   make(chan Event, 256),
	}, nil
}

func (c *Controller) Events() <-chan Event {
	return c.events
}

func (c *Controller) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	defer func() {
		if c.events != nil {
			close(c.events)
		}
	}()

	targetIP, err := resolveTargetIP(ctx, c.config.Target, c.config.IPVersion)
	if err != nil {
		c.emit(Event{Type: EventTypeError, Err: err})
		return err
	}
	c.mu.Lock()
	c.config.TargetIP = targetIP.String()
	c.mu.Unlock()
	if err := c.prober.SetTarget(targetIP); err != nil {
		c.emit(Event{Type: EventTypeError, Err: err})
		return err
	}

	rounds := c.config.Count
	if rounds == 0 {
		rounds = -1
	}

	for round := 0; rounds < 0 || round < rounds; round++ {
		if err := ctx.Err(); err != nil {
			c.emit(Event{Type: EventTypeError, Err: err})
			return err
		}

		for ttl := 1; ttl <= c.config.MaxHops; ttl++ {
			seq := round*c.config.MaxHops + ttl
			res, probeErr := c.prober.Probe(ctx, ttl, seq)
			if probeErr != nil {
				c.emit(Event{Type: EventTypeError, Err: probeErr})
				return probeErr
			}
			c.applyResult(ctx, ttl, res)
			c.emit(Event{Type: EventTypeHopUpdated, TTL: ttl, Round: round})
			if res != nil && res.Type == ResponseTypeEchoReply {
				break
			}
		}

		c.emit(Event{Type: EventTypeRoundCompleted, Round: round})
		if rounds < 0 || round != rounds-1 {
			select {
			case <-ctx.Done():
				c.emit(Event{Type: EventTypeError, Err: ctx.Err()})
				return ctx.Err()
			case <-time.After(c.config.Interval):
			}
		}
	}

	c.emit(Event{Type: EventTypeDone})
	return nil
}

func (c *Controller) applyResult(ctx context.Context, ttl int, res *ProbeResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	hop := c.hops[ttl]
	if hop == nil {
		hop = NewHop(ttl)
		c.hops[ttl] = hop
	}

	hop.Stats.Sent++
	if res == nil || res.Type == ResponseTypeTimeout || res.IP == nil {
		hop.Lost = true
		hop.Stats.UpdateLoss()
		return
	}

	hop.Lost = false
	ipChanged := hop.IP == nil || !hop.IP.Equal(res.IP)
	hop.IP = res.IP
	hop.Stats.Received++
	hop.Stats.AddRTT(res.RTT)
	hop.Stats.UpdateLoss()

	if c.config.EnableDNS {
		if hop.Hostname == "" || ipChanged {
			hop.Hostname = reverseDNS(ctx, res.IP)
		}
	}

	if ipChanged {
		hop.Location = nil
	}
	if c.resolver != nil && hop.Location == nil {
		hop.Location = c.resolver.Resolve(res.IP)
	}
}

func (c *Controller) Snapshot() *Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hops := make([]*Hop, 0, len(c.hops))
	for _, hop := range c.hops {
		hops = append(hops, hop)
	}
	sort.Slice(hops, func(i, j int) bool { return hops[i].TTL < hops[j].TTL })

	out := make([]SnapshotHop, 0, len(hops))
	for _, hop := range hops {
		out = append(out, hop.ToSnapshot())
	}

	return &Snapshot{
		SchemaVersion: 1,
		Target:        c.config.Target,
		TargetIP:      c.config.TargetIP,
		Protocol:      string(c.config.Protocol),
		MaxHops:       c.config.MaxHops,
		Count:         c.config.Count,
		Hops:          out,
	}
}

func (c *Controller) emit(e Event) {
	if c.events == nil {
		return
	}
	select {
	case c.events <- e:
	default:
	}
}

func resolveTargetIP(ctx context.Context, target string, ipVersion int) (net.IP, error) {
	ipAddr, err := net.DefaultResolver.LookupIPAddr(ctx, target)
	if err != nil {
		return nil, errors.New(i18n.Tf("err.resolveTarget", map[string]interface{}{"Error": err.Error()}))
	}
	for _, a := range ipAddr {
		if (ipVersion == 4 && a.IP.To4() != nil) || (ipVersion == 6 && a.IP.To4() == nil && a.IP.To16() != nil) {
			return a.IP, nil
		}
	}
	return nil, errors.New(i18n.Tf("err.ipNotFound", map[string]interface{}{"Version": ipVersion, "Target": target}))
}

func reverseDNS(ctx context.Context, ip net.IP) string {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	names, err := net.DefaultResolver.LookupAddr(ctx, ip.String())
	if err != nil || len(names) == 0 {
		return ""
	}
	name := strings.TrimSuffix(names[0], ".")
	return name
}

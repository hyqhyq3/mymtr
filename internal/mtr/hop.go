package mtr

import (
	"fmt"
	"math"
	"net"
	"time"

	"github.com/yangqihuang/mymtr/internal/geoip"
)

type Hop struct {
	TTL      int
	IP       net.IP
	Hostname string
	Location *geoip.GeoLocation
	Stats    *HopStats
	Lost     bool
}

func NewHop(ttl int) *Hop {
	return &Hop{
		TTL:   ttl,
		Stats: NewHopStats(),
		Lost:  true,
	}
}

type HopStats struct {
	Sent     int           `json:"sent"`
	Received int           `json:"received"`
	Loss     float64       `json:"loss"`
	Last     time.Duration `json:"last"`
	Best     time.Duration `json:"best"`
	Worst    time.Duration `json:"worst"`
	Avg      time.Duration `json:"avg"`
	StdDev   time.Duration `json:"stddev"`
	History  []time.Duration

	mean float64
	m2   float64
	n    int
}

func NewHopStats() *HopStats {
	return &HopStats{
		History: make([]time.Duration, 0, 10),
	}
}

func (s *HopStats) AddRTT(rtt time.Duration) {
	s.Last = rtt
	if s.Best == 0 || rtt < s.Best {
		s.Best = rtt
	}
	if rtt > s.Worst {
		s.Worst = rtt
	}

	s.n++
	x := float64(rtt.Nanoseconds())
	delta := x - s.mean
	s.mean += delta / float64(s.n)
	s.m2 += delta * (x - s.mean)
	s.Avg = time.Duration(int64(s.mean))
	if s.n > 1 {
		variance := s.m2 / float64(s.n-1)
		if variance < 0 {
			variance = 0
		}
		s.StdDev = time.Duration(int64(math.Sqrt(variance))).Truncate(time.Nanosecond)
	}

	s.appendHistory(rtt)
}

func (s *HopStats) appendHistory(rtt time.Duration) {
	const max = 10
	if len(s.History) < max {
		s.History = append(s.History, rtt)
		return
	}
	copy(s.History, s.History[1:])
	s.History[len(s.History)-1] = rtt
}

func (s *HopStats) UpdateLoss() {
	if s.Sent <= 0 {
		s.Loss = 0
		return
	}
	s.Loss = (1.0 - float64(s.Received)/float64(s.Sent)) * 100.0
	if s.Loss < 0 {
		s.Loss = 0
	}
	if s.Loss > 100 {
		s.Loss = 100
	}
}

type Snapshot struct {
	SchemaVersion int           `json:"schema_version"`
	Target        string        `json:"target"`
	TargetIP      string        `json:"target_ip"`
	Protocol      string        `json:"protocol"`
	MaxHops       int           `json:"max_hops"`
	Count         int           `json:"count"`
	Hops          []SnapshotHop `json:"hops"`
}

type SnapshotHop struct {
	TTL      int                `json:"ttl"`
	IP       string             `json:"ip,omitempty"`
	Hostname string             `json:"hostname,omitempty"`
	Lost     bool               `json:"lost"`
	Location *geoip.GeoLocation `json:"location,omitempty"`
	Stats    SnapshotHopSta     `json:"stats"`
}

type SnapshotHopSta struct {
	Sent     int     `json:"sent"`
	Received int     `json:"received"`
	Loss     float64 `json:"loss"`
	LastMs   int64   `json:"last_ms"`
	AvgMs    int64   `json:"avg_ms"`
	BestMs   int64   `json:"best_ms"`
	WorstMs  int64   `json:"worst_ms"`
	StdDevMs int64   `json:"stddev_ms"`

	HistoryMs []int64 `json:"history_ms,omitempty"`

	Last   string `json:"last,omitempty"`
	Best   string `json:"best,omitempty"`
	Worst  string `json:"worst,omitempty"`
	Avg    string `json:"avg,omitempty"`
	StdDev string `json:"stddev,omitempty"`
}

func (h *Hop) ToSnapshot() SnapshotHop {
	ip := ""
	if h.IP != nil {
		ip = h.IP.String()
	}

	historyMs := make([]int64, 0, len(h.Stats.History))
	for _, d := range h.Stats.History {
		historyMs = append(historyMs, durationMs(d))
	}
	return SnapshotHop{
		TTL:      h.TTL,
		IP:       ip,
		Hostname: h.Hostname,
		Lost:     h.Lost,
		Location: h.Location,
		Stats: SnapshotHopSta{
			Sent:      h.Stats.Sent,
			Received:  h.Stats.Received,
			Loss:      h.Stats.Loss,
			LastMs:    durationMs(h.Stats.Last),
			AvgMs:     durationMs(h.Stats.Avg),
			BestMs:    durationMs(h.Stats.Best),
			WorstMs:   durationMs(h.Stats.Worst),
			StdDevMs:  durationMs(h.Stats.StdDev),
			HistoryMs: historyMs,

			Last:   durationStringMs(h.Stats.Last),
			Best:   durationStringMs(h.Stats.Best),
			Worst:  durationStringMs(h.Stats.Worst),
			Avg:    durationStringMs(h.Stats.Avg),
			StdDev: durationStringMs(h.Stats.StdDev),
		},
	}
}

func durationStringMs(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	return fmt.Sprintf("%dms", durationMs(d))
}

func durationMs(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	return d.Round(time.Millisecond).Milliseconds()
}

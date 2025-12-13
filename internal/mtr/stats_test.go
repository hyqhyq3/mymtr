package mtr

import (
	"math"
	"testing"
	"time"
)

func TestHopStats_AddRTTAndLoss(t *testing.T) {
	s := NewHopStats()

	s.Sent = 3
	s.Received = 0
	s.UpdateLoss()
	if s.Loss != 100 {
		t.Fatalf("expected loss=100, got=%v", s.Loss)
	}

	s.AddRTT(10 * time.Millisecond)
	s.Received++
	s.Sent++
	s.UpdateLoss()
	if s.Best != 10*time.Millisecond || s.Worst != 10*time.Millisecond || s.Last != 10*time.Millisecond {
		t.Fatalf("unexpected best/worst/last: %v %v %v", s.Best, s.Worst, s.Last)
	}
	if s.Avg <= 0 {
		t.Fatalf("expected avg > 0")
	}
	if s.Loss < 0 || s.Loss > 100 {
		t.Fatalf("unexpected loss range: %v", s.Loss)
	}
}

func TestHopStats_StdDev(t *testing.T) {
	s := NewHopStats()
	s.AddRTT(10 * time.Millisecond)
	s.AddRTT(20 * time.Millisecond)

	// sample stddev for [10,20]ms is sqrt(((5ms)^2 + (5ms)^2)/(2-1)) ~= 7.071067ms
	want := 7.071067 * float64(time.Millisecond)
	got := float64(s.StdDev)
	if math.Abs(got-want) > 0.2*float64(time.Millisecond) {
		t.Fatalf("stddev too far: got=%v want≈%v", s.StdDev, time.Duration(want))
	}
}

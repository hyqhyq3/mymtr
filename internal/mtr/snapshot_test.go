package mtr

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSnapshot_JSONSchema(t *testing.T) {
	h := NewHop(1)
	h.IP = []byte{8, 8, 8, 8}
	h.Stats.Sent = 2
	h.Stats.Received = 2
	h.Stats.AddRTT(10 * time.Millisecond)
	h.Stats.AddRTT(20 * time.Millisecond)
	h.Stats.UpdateLoss()

	s := &Snapshot{
		SchemaVersion: 1,
		Target:        "example.com",
		TargetIP:      "8.8.8.8",
		Protocol:      "udp",
		MaxHops:       30,
		Count:         1,
		Hops:          []SnapshotHop{h.ToSnapshot()},
	}

	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["schema_version"] != float64(1) {
		t.Fatalf("expected schema_version=1, got=%v", m["schema_version"])
	}

	hops, ok := m["hops"].([]any)
	if !ok || len(hops) != 1 {
		t.Fatalf("expected hops array size 1, got=%T len=%d", m["hops"], len(hops))
	}
	hop, ok := hops[0].(map[string]any)
	if !ok {
		t.Fatalf("expected hop object, got=%T", hops[0])
	}
	stats, ok := hop["stats"].(map[string]any)
	if !ok {
		t.Fatalf("expected stats object, got=%T", hop["stats"])
	}
	if _, ok := stats["last_ms"]; !ok {
		t.Fatalf("expected last_ms in stats")
	}
	if _, ok := stats["history_ms"]; !ok {
		t.Fatalf("expected history_ms in stats")
	}
}

package engine

import (
	"testing"
)

func TestFindDestinationRecord(t *testing.T) {
	snap := &StateSnapshot{
		Destinations: []DestinationRecord{
			{IP: "8.8.8.8", Domain: "dns.google"},
			{IP: "1.1.1.1"},
		},
	}
	if r := FindDestinationRecord(snap, "8.8.8.8"); r == nil || r.Domain != "dns.google" {
		t.Fatalf("by IP: %+v", r)
	}
	if r := FindDestinationRecord(snap, "dns.google"); r == nil {
		t.Fatal("by domain")
	}
	if FindDestinationRecord(snap, "9.9.9.9") != nil {
		t.Fatal("expected nil")
	}
}

package memlog

import (
	"testing"
)

func TestRing_WriteAndLastN(t *testing.T) {
	r := NewRing(10)
	r.Write("info", "msg1")
	r.Write("error", "msg2")
	entries := r.LastN(5)
	if len(entries) != 2 {
		t.Errorf("LastN(5) = %d", len(entries))
	}
	if entries[0].Message != "msg2" {
		t.Errorf("last message = %s", entries[0].Message)
	}
}

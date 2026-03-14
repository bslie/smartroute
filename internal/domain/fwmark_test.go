package domain

import (
	"testing"
)

func TestComposeMark(t *testing.T) {
	// tunnel 1, class 0 -> 0x01
	got := ComposeMark(1, 0)
	if got != 1 {
		t.Errorf("ComposeMark(1,0) = %d, want 1", got)
	}
	// tunnel 2, class 1 (game) -> 0x02 | 0x0100 = 0x0102
	got = ComposeMark(2, 1)
	if got != 0x102 {
		t.Errorf("ComposeMark(2,1) = %x, want 0x102", got)
	}
}

func TestParseMark(t *testing.T) {
	tunnel, class := ParseMark(0x102)
	if tunnel != 2 || class != 1 {
		t.Errorf("ParseMark(0x102) = %d, %d; want 2, 1", tunnel, class)
	}
}

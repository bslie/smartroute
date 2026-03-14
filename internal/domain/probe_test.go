package domain

import (
	"testing"
)

func TestNegativeSignalFactor(t *testing.T) {
	if NegativeSignalFactor(ErrorNone) != 1.0 {
		t.Error("ErrorNone want 1.0")
	}
	if NegativeSignalFactor(ErrorHTTP403) != 0.1 {
		t.Error("ErrorHTTP403 want 0.1")
	}
	if NegativeSignalFactor(ErrorTimeout) != 0.4 {
		t.Error("ErrorTimeout want 0.4")
	}
}

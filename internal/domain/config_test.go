package domain

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.TickIntervalMs != 2000 {
		t.Errorf("TickIntervalMs want 2000, got %d", cfg.TickIntervalMs)
	}
	if cfg.ClientSubnet != "" {
		t.Errorf("ClientSubnet want empty default, got %q", cfg.ClientSubnet)
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ClientSubnet = "10.0.0.0/24"
	cfg.Tunnels = []TunnelConfig{{Name: "wg0", Endpoint: "1.2.3.4:51820", PrivateKeyFile: "/dev/null"}}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.Tunnels = nil
	if err := cfg.Validate(); err != ErrNoTunnels {
		t.Errorf("want ErrNoTunnels, got %v", err)
	}
}

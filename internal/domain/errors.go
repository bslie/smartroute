package domain

import "errors"

var (
	ErrInvalidConfig   = errors.New("invalid config")
	ErrImmutableChange = errors.New("immutable field changed on reload")
	ErrNoTunnels       = errors.New("no tunnels configured")
)

//go:build !linux

package cli

// daemonizeDetach на не-Linux — пустая реализация (демонизация только на Linux).
func daemonizeDetach() {}

//go:build !linux

package probe

import (
	"syscall"
)

// bindToDevice на не-Linux — заглушка (SO_BINDTODEVICE недоступен).
func bindToDevice(iface string) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		return nil
	}
}

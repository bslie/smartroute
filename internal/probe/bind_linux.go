//go:build linux

package probe

import (
	"syscall"
)

// bindToDevice возвращает Control func для net.Dialer, устанавливающую SO_BINDTODEVICE.
func bindToDevice(iface string) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		var setSockOptErr error
		err := c.Control(func(fd uintptr) {
			setSockOptErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, iface)
		})
		if err != nil {
			return err
		}
		return setSockOptErr
	}
}

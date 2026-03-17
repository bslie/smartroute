//go:build linux

package cli

import (
	"os"
	"syscall"
)

// daemonizeDetach отвязывает процесс от терминала (setsid) и перенаправляет stdio в /dev/null.
func daemonizeDetach() {
	if sid, err := syscall.Setsid(); err == nil && sid >= 0 {
		_ = sid
	}
	_ = os.Chdir("/")
	devNull, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
	if err != nil {
		return
	}
	defer devNull.Close()
	_ = syscall.Dup2(int(devNull.Fd()), 0)
	_ = syscall.Dup2(int(devNull.Fd()), 1)
	_ = syscall.Dup2(int(devNull.Fd()), 2)
}

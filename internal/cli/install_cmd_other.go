//go:build !linux

package cli

import "os/exec"

func setInstallCmdSysProc(cmd *exec.Cmd) {}

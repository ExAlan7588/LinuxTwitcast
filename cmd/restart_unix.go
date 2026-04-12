//go:build !windows

package cmd

import (
	"os"
	"syscall"
)

func restartProcess(executable string, args []string) error {
	return syscall.Exec(executable, append([]string{executable}, args...), os.Environ())
}

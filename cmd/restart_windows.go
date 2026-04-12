//go:build windows

package cmd

import (
	"os"
	"os/exec"
)

func restartProcess(executable string, args []string) error {
	command := exec.Command(executable, args...)
	command.Dir = "."
	command.Env = os.Environ()
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Start()
}

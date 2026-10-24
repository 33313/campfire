//go:build !windows

package main

import (
	"os"
	"syscall"
)

// restart replaces this process with a fresh run of the same program. The
// process ID stays, so a supervisor or a container sees no exit.
func restart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return syscall.Exec(exe, os.Args, os.Environ())
}

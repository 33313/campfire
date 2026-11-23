//go:build !windows

package updates

import "os"

// Replace keeps the old inode under a second name so the executable path is
// never missing, even if the process is interrupted during installation.
func Replace(exe, staged string) error {
	previous := exe + ".previous"
	if err := os.Remove(previous); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Link(exe, previous); err != nil {
		return err
	}
	return os.Rename(staged, exe)
}

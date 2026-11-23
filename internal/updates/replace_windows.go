package updates

import (
	"errors"
	"os"
)

// Windows allows renaming a running executable out of the way but does not
// allow replacing it in place.
func Replace(exe, staged string) error {
	previous := exe + ".previous"
	if err := os.Remove(previous); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(exe, previous); err != nil {
		return err
	}
	if err := os.Rename(staged, exe); err != nil {
		return errors.Join(err, os.Rename(previous, exe))
	}
	return nil
}

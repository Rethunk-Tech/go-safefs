package safefs

import (
	"errors"
	"io/fs"
	"os"
)

// CheckWritable fails fast when dir refuses new files, as a folder under Program Files does for a non-elevated
// process, instead of letting the first real write fail halfway through. A missing dir is nil: creating it is the
// caller's job. Any other failure is the probe's *fs.PathError, which wraps fs.ErrPermission when that is why.
func CheckWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".safefs-probe-*")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	name := f.Name()
	_ = f.Close()
	return Remove(name)
}

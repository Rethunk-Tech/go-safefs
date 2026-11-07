package safefs

import (
	"errors"

	"golang.org/x/sys/unix"
)

// syncFS flushes every dirty page and journal entry of the filesystem holding dir with one syncfs(2).
func syncFS(dir string) error {
	f, err := Open(dir)
	if err != nil {
		return err
	}
	return errors.Join(unix.Syncfs(int(f.Fd())), f.Close())
}

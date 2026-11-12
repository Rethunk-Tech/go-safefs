//go:build !windows

package safefs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"syscall"
)

// checkNoMountsInside refuses a folder that has a mount point below it. The mount table is authoritative where the
// kernel offers it, since a bind mount of the same filesystem shares its device id; elsewhere a directory on a
// different device from the root stands for a mount.
func checkNoMountsInside(path string) error {
	// A missing path, a file or a link has nothing below it; os.RemoveAll reports or removes it.
	if info, err := os.Lstat(path); err == nil && info.IsDir() { // #nosec G703 -- the folder the caller asked to remove
		return checkDir(path, info)
	}
	return nil
}

// mountedDir reports whether p is a mount point under src: listed in the mount table when that is readable, else a
// directory whose device id differs from src's.
func mountedDir(src, p string) bool {
	if p == src {
		return false
	}
	mounts, err := mountsBelow(src)
	if err == nil {
		abs, aerr := filepath.Abs(p)
		if aerr != nil {
			return false
		}
		return slices.Contains(mounts, abs)
	}
	srcDev, e1 := Device(src)
	pDev, e2 := Device(p)
	return e1 == nil && e2 == nil && srcDev != pDev
}

func checkDir(path string, info fs.FileInfo) error {
	if mounts, err := mountsBelow(path); err == nil {
		if len(mounts) > 0 {
			return fmt.Errorf("%w: %s under %s", ErrMountInside, mounts[0], path)
		}
		return nil
	}
	return deviceChangesBelow(path, info)
}

func deviceChangesBelow(path string, root fs.FileInfo) error {
	rootStat, ok := root.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	return filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error { // #nosec G703 -- the folder the caller asked to remove
		if errors.Is(err, fs.ErrNotExist) {
			return nil // removed meanwhile
		}
		if err != nil || !d.IsDir() || p == path {
			return err
		}
		if info, err := d.Info(); err == nil {
			if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Dev != rootStat.Dev {
				return fmt.Errorf("%w: %s under %s", ErrMountInside, p, path)
			}
		}
		return nil
	})
}

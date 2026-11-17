//go:build unix

package diskcache

import (
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func readEntry(dir, name string) ([]byte, error) {
	parent, base, closer, err := parentAt(dir, name, false)
	if err != nil {
		return nil, err
	}
	defer closer()
	// O_NOFOLLOW: a planted symlink at the entry is a miss. The fd's Stat is what we read, not a path that can change.
	fd, err := unix.Openat(parent, base, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), base)
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		if err != nil {
			return nil, err
		}
		return nil, os.ErrInvalid
	}
	return io.ReadAll(f)
}

// Temp create and renameat share the parent fd so a name swapped for a symlink cannot redirect the write.
func replaceEntry(dir, name string, data []byte) error {
	parent, base, closer, err := parentAt(dir, name, true)
	if err != nil {
		return err
	}
	defer closer()
	tmp := base + "." + rand.Text() + ".tmp"
	fd, err := unix.Openat(parent, tmp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), tmp)
	_, werr := f.Write(data)
	if err := f.Close(); werr != nil || err != nil {
		_ = unix.Unlinkat(parent, tmp, 0)
		if werr != nil {
			return werr
		}
		return err
	}
	if err := unix.Renameat(parent, tmp, parent, base); err != nil {
		_ = unix.Unlinkat(parent, tmp, 0)
		return err
	}
	return nil
}

func parentAt(dir, name string, mkdir bool) (parent int, base string, closer func(), err error) {
	dirs, base, err := splitName(name)
	if err != nil {
		return 0, "", func() {}, err
	}
	dfd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0) // #nosec G304 G703 -- Cache.Dir, the folder the caller named
	if err != nil {
		return 0, "", func() {}, err
	}
	parent = dfd
	fail := func(err error) (int, string, func(), error) {
		if parent != dfd {
			_ = unix.Close(parent)
		}
		_ = unix.Close(dfd)
		return 0, "", func() {}, err
	}
	for _, d := range dirs {
		fd, err := unix.Openat(parent, d, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err != nil && mkdir && errors.Is(err, unix.ENOENT) {
			if err := unix.Mkdirat(parent, d, 0o700); err != nil && !errors.Is(err, unix.EEXIST) {
				return fail(err)
			}
			fd, err = unix.Openat(parent, d, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		}
		if parent != dfd {
			_ = unix.Close(parent)
		}
		if err != nil {
			_ = unix.Close(dfd)
			return 0, "", func() {}, err
		}
		parent = fd
	}
	return parent, base, func() {
		if parent != dfd {
			_ = unix.Close(parent)
		}
		_ = unix.Close(dfd)
	}, nil
}

func splitName(name string) (dirs []string, base string, err error) {
	rel := filepath.Clean(filepath.FromSlash(name))
	if rel == "." || !filepath.IsLocal(rel) {
		return nil, "", os.ErrInvalid
	}
	base = filepath.Base(rel)
	parent := filepath.Dir(rel)
	if parent == "." {
		return nil, base, nil
	}
	return strings.Split(parent, string(filepath.Separator)), base, nil
}

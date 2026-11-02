package safefs

import (
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

// Within runs fn with an os.Root on path's parent directory and path's final element, so a symlinked final element
// cannot lead out of the parent. A trailing separator is dropped first: Dir keeps it as the last element while Base
// strips it, which would name that element twice.
func Within[T any](path string, fn func(dir *os.Root, name string) (T, error)) (T, error) {
	path = filepath.Clean(path)
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		var zero T
		return zero, err
	}
	defer func() { _ = root.Close() }()
	return fn(root, filepath.Base(path))
}

// Open opens path for reading.
func Open(path string) (*os.File, error) {
	return Within(path, func(r *os.Root, name string) (*os.File, error) { return r.Open(name) })
}

// ErrNotRegular is OpenRegular's refusal of a directory, device, pipe or other non-regular file.
var ErrNotRegular = errors.New("not a regular file")

// OpenRegular opens path for reading, refusing anything but a regular file.
func OpenRegular(path string) (*os.File, error) {
	f, err := Open(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = ErrNotRegular
	}
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// Create creates or truncates path.
func Create(path string) (*os.File, error) {
	return Within(path, func(r *os.Root, name string) (*os.File, error) { return r.Create(name) })
}

// OpenFile opens path with flag and perm, still through the parent root.
func OpenFile(path string, flag int, perm fs.FileMode) (*os.File, error) {
	return Within(path, func(r *os.Root, name string) (*os.File, error) {
		return r.OpenFile(name, flag, perm)
	})
}

// CreateExcl creates path with perm and fails if it already exists.
func CreateExcl(path string, perm fs.FileMode) (*os.File, error) {
	return Within(path, func(r *os.Root, name string) (*os.File, error) {
		return r.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	})
}

// Chmod sets path's permission bits.
func Chmod(path string, perm fs.FileMode) error {
	_, err := Within(path, func(r *os.Root, name string) (struct{}, error) {
		return struct{}{}, r.Chmod(name, perm)
	})
	return err
}

// ReadFile reads all of path.
func ReadFile(path string) ([]byte, error) {
	return Within(path, func(r *os.Root, name string) ([]byte, error) { return r.ReadFile(name) })
}

// ReadFileIfExists is ReadFile for a file that may not be there: a missing path gives nil, false, nil, and any other
// failure is returned.
func ReadFileIfExists(path string) ([]byte, bool, error) {
	data, err := ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// ReadDirIfExists lists the directory path, sorted by name like os.ReadDir, through the parent root. A missing path
// gives nil, false, nil, and any other failure is returned.
func ReadDirIfExists(path string) ([]os.DirEntry, bool, error) {
	missing := false
	entries, err := Within(path, func(r *os.Root, name string) ([]os.DirEntry, error) {
		// Windows reports opening or reading a file as a directory as not-exist, so the kind is checked first.
		info, err := r.Lstat(name)
		if errors.Is(err, fs.ErrNotExist) {
			missing = true
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, &fs.PathError{Op: "readdir", Path: path, Err: syscall.ENOTDIR}
		}
		d, err := r.Open(name)
		if err != nil {
			return nil, err
		}
		defer func() { _ = d.Close() }()
		list, err := d.ReadDir(-1)
		slices.SortFunc(list, func(a, b os.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
		return list, err
	})
	if missing || errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return entries, true, nil
}

// WriteFile writes data to path, creating it with perm if needed.
func WriteFile(path string, data []byte, perm fs.FileMode) error {
	_, err := Within(path, func(r *os.Root, name string) (struct{}, error) {
		return struct{}{}, r.WriteFile(name, data, perm)
	})
	return err
}

// HashFile is the hex digest of path's contents under h.
func HashFile(path string, h hash.Hash) (string, error) {
	f, err := Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// IsFile is true when path resolves to a regular file. Unlike IsDir it uses os.Stat, so a symlink whose target
// is outside path's folder (an executable linked from another library) still counts.
func IsFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

// Stat describes path.
func Stat(path string) (fs.FileInfo, error) {
	return Within(path, func(r *os.Root, name string) (fs.FileInfo, error) { return r.Stat(name) })
}

// IsDir is true when path resolves to a directory. Like IsFile it follows a final symlink out of path's folder, so a
// symlinked folder still counts.
func IsDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

package safefs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// WalkFiles calls fn with the slash path below root, the full path and the size of every regular file under root, in
// walk order.
func WalkFiles(root string, fn func(rel, path string, size int64) error) error {
	return fs.WalkDir(os.DirFS(root), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return fn(rel, filepath.Join(root, filepath.FromSlash(rel)), info.Size())
	})
}

// WalkRegular calls fn with the path and entry of every regular file under root, in walk order. A missing root is no
// error. Symlinks are not followed and not reported, so a link cannot lead the walk out of root.
func WalkRegular(root string, fn func(path string, d fs.DirEntry) error) error {
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		return fn(path, d)
	})
	if errors.Is(err, fs.ErrNotExist) && !pathExists(root) {
		return nil
	}
	return err
}

func pathExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

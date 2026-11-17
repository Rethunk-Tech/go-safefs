//go:build !unix

package diskcache

import (
	"crypto/rand"
	"os"
	"path/filepath"

	"github.com/Rethunk-Tech/go-safefs"
)

func readEntry(dir, name string) ([]byte, error) {
	return safefs.ReadFile(filepath.Join(dir, name))
}

func replaceEntry(dir, name string, data []byte) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if d := filepath.ToSlash(filepath.Dir(filepath.FromSlash(name))); d != "." {
		if err := root.MkdirAll(d, 0o700); err != nil {
			return err
		}
	}
	tmp := name + "." + rand.Text() + ".tmp"
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	if err := f.Close(); werr != nil || err != nil {
		_ = root.Remove(tmp)
		if werr != nil {
			return werr
		}
		return err
	}
	if err := root.Rename(tmp, name); err != nil {
		_ = root.Remove(tmp)
		return err
	}
	return nil
}

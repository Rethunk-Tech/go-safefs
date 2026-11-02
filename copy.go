package safefs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// CopyProgress counts files and bytes while CopyTree runs.
type CopyProgress struct {
	Files      int
	TotalFiles int
	Bytes      int64
	TotalBytes int64
}

// CopyOptions tunes CopyFile, CopyTree and CopyTreeFollowingLinks. The zero value is the default for every field.
type CopyOptions struct {
	// TempSuffix names CopyFile's temp file, dst+TempSuffix. CopyFile removes a stale file of that name before
	// writing, so the suffix must be specific enough never to name a file anyone keeps. Zero means ".safefs-tmp".
	TempSuffix string
	// First names a top-level entry of src that CopyTree copies before the rest.
	First string
	// Progress is called after each file with the running totals.
	Progress func(CopyProgress)
	// Put writes one file; from is where its bytes live, to where they go, rel its slash path below src. Nil means
	// CopyFile, which reads through the walk's root; a Put opens from by path, outside that guarantee.
	Put func(from, to, rel string) error
	// Skip names entries CopyTreeFollowingLinks leaves out.
	Skip func(name string) bool
}

func (o CopyOptions) tempSuffix() string {
	if o.TempSuffix == "" {
		return ".safefs-tmp"
	}
	return o.TempSuffix
}

// CopyTree copies the regular files and folders under src into dst (which may exist), file by file. Directory
// junctions, mount points and symlink directories are not followed, and are returned in skipped. Symlink files are
// copied by content when they still resolve under src, and are an error when they escape.
func CopyTree(src, dst string, opts CopyOptions) (skipped []string, err error) {
	r, err := os.OpenRoot(src)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	root, err := EvalSymlinks(src)
	if err != nil {
		return nil, err
	}
	var progress CopyProgress
	if opts.Progress != nil {
		progress.TotalBytes, err = Size(src)
		if err != nil {
			return nil, err
		}
		err = filepath.WalkDir(src, func(_ string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.Type().IsRegular() {
				progress.TotalFiles++
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	passes := 1
	if opts.First != "" {
		passes = 2
	}
	for pass := 0; pass < passes && err == nil; pass++ {
		err = walkCopy(r, src, dst, opts, pass, &progress, opts.Put, root, &skipped)
	}
	return skipped, err
}

// LinkedDir reports an entry that leads to a directory without being one: a symlink to a folder, or on Windows a
// junction or mount point, which Go reports as an irregular file.
func LinkedDir(p string, info os.FileInfo) bool {
	if info.IsDir() {
		return false
	}
	if info.Mode()&os.ModeSymlink == 0 && info.Mode().IsRegular() {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// openIn opens the file rel (a slash path) below r, refusing a link that leads out of it. A Windows runner has
// answered not-exist for a file that is there, so that one answer is checked against the plain path.
func openIn(r *os.Root, src, rel string) (*os.File, error) {
	f, err := r.Open(rel)
	if errors.Is(err, fs.ErrNotExist) {
		if p := filepath.Join(src, filepath.FromSlash(rel)); pathExists(p) {
			return Open(p)
		}
	}
	return f, err
}

// openLinkTarget opens the file the link p leads to when the root refused the link itself: os.Root treats an
// absolute link target as an escape even when it lands inside, so the target is resolved and reopened by its path
// below the real source folder, which the root still confines. A target outside is the "escapes" refusal.
func openLinkTarget(r *os.Root, src, root, p string) (*os.File, error) {
	resolved, err := EvalSymlinks(p)
	if err != nil {
		return nil, err
	}
	if !UnderRoot(root, resolved) {
		return nil, fmt.Errorf("%s escapes %s", p, src)
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return nil, err
	}
	return openIn(r, src, filepath.ToSlash(rel))
}

// walkCopy is one walk of CopyTree through the root r on src, so no entry is opened by a path a swapped link could
// redirect. In a two-pass copy, pass 0 takes only the first entry and pass 1 the rest.
func walkCopy(r *os.Root, src, dst string, opts CopyOptions, pass int, progress *CopyProgress, put func(from, to, rel string) error, root string, skipped *[]string) error {
	return fs.WalkDir(r.FS(), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o750)
		}
		if opts.First != "" && (strings.SplitN(rel, "/", 2)[0] == opts.First) != (pass == 0) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		p := filepath.Join(src, filepath.FromSlash(rel))
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if info.IsDir() {
			if !RealDirUnder(root, p) {
				return fs.SkipDir
			}
			return os.MkdirAll(target, 0o750)
		}
		if LinkedDir(p, info) {
			*skipped = append(*skipped, p)
			return nil
		}
		isLink := info.Mode()&os.ModeSymlink != 0
		if !isLink && !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", p)
		}
		f, err := openIn(r, src, rel)
		if err != nil && isLink {
			f, err = openLinkTarget(r, src, root, p)
		}
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		st, err := f.Stat()
		if err != nil {
			return err
		}
		if !st.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", p)
		}
		if put == nil {
			if err := copyFrom(f, target, opts); err != nil {
				return err
			}
		} else {
			// Put takes a path, so a custom Put reopens it outside the root; the open above only vets the entry.
			from := p
			if isLink {
				if from, err = EvalSymlinks(p); err != nil {
					return err
				}
			}
			if err := put(from, target, filepath.FromSlash(rel)); err != nil {
				return err
			}
		}
		if opts.Progress != nil {
			progress.Files++
			progress.Bytes += st.Size()
			opts.Progress(*progress)
		}
		return nil
	})
}

// UnderRoot reports whether p is root or a path still inside it after both have been cleaned.
func UnderRoot(root, p string) bool {
	root = filepath.Clean(root)
	p = filepath.Clean(p)
	rel, err := filepath.Rel(root, p)
	return err == nil && filepath.IsLocal(rel)
}

// RealDirUnder reports whether p is a directory whose own path still resolves under root, not a symlink
// directory or a junction/reparse that points somewhere else.
func RealDirUnder(root, p string) bool {
	info, err := os.Lstat(p)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	resolved, err := EvalSymlinks(p)
	if err != nil {
		return false
	}
	parent, err := EvalSymlinks(filepath.Dir(p))
	if err != nil {
		return false
	}
	if filepath.Clean(resolved) != filepath.Clean(filepath.Join(parent, filepath.Base(p))) {
		return false
	}
	return UnderRoot(root, resolved)
}

// CopyFile copies the regular file src to dst, replacing what is there. The bytes go to a temporary file beside dst,
// are flushed to disk, and only then renamed into place, so a crash leaves dst whole or absent, never truncated.
func CopyFile(src, dst string, opts CopyOptions) (err error) {
	in, err := Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	return copyFrom(in, dst, opts)
}

// copyFrom is CopyFile's body over an already open source.
func copyFrom(in *os.File, dst string, opts CopyOptions) (err error) {
	info, err := in.Stat()
	if err != nil {
		return err
	}
	tmp := dst + opts.tempSuffix()
	_ = Remove(tmp)
	// The copy keeps the source's exec and read bits but never grants write to group or others. The owner always
	// keeps write: a read-only copy is a read-only attribute on Windows, and renaming over it later fails.
	out, err := CreateExcl(tmp, info.Mode().Perm()&0o755|0o200)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = Remove(tmp)
		}
	}()
	if _, err = io.Copy(out, in); err == nil {
		err = out.Sync()
	}
	if err = errors.Join(err, out.Close()); err != nil {
		return err
	}
	return Rename(tmp, dst)
}

// DanglingLinkError is CopyTreeFollowingLinks' refusal of a file link whose target is gone.
type DanglingLinkError struct {
	// Rel is the link's path below src.
	Rel string
	Err error
}

func (e *DanglingLinkError) Error() string {
	return fmt.Sprintf("%s links to a file that no longer exists: %v", e.Rel, e.Err)
}

func (e *DanglingLinkError) Unwrap() error { return e.Err }

// CopyTreeFollowingLinks copies src to dst like CopyTree but writes the file a file link points to wherever it lives.
// It is for a folder the user chose on their own disk, filled with links by another tool; linked folders are still
// skipped and anything that is not a regular file is refused. A link to a missing file is a *DanglingLinkError.
func CopyTreeFollowingLinks(src, dst string, opts CopyOptions) error {
	r, err := os.OpenRoot(src)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	root, err := EvalSymlinks(src)
	if err != nil {
		return err
	}
	return fs.WalkDir(r.FS(), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o750)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		p := filepath.Join(src, filepath.FromSlash(rel))
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if info.IsDir() {
			if !RealDirUnder(root, p) {
				return fs.SkipDir
			}
			return os.MkdirAll(target, 0o750)
		}
		if LinkedDir(p, info) || (opts.Skip != nil && opts.Skip(info.Name())) {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// A file link is followed wherever it points, by design, so it is read by path.
			from, err := EvalSymlinks(p)
			if errors.Is(err, fs.ErrNotExist) {
				return &DanglingLinkError{Rel: filepath.FromSlash(rel), Err: err}
			} else if err != nil {
				return err
			}
			if info, err = os.Stat(from); err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("%s is not a regular file", p)
			}
			return CopyFile(from, target, opts)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", p)
		}
		f, err := openIn(r, src, rel)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		return copyFrom(f, target, opts)
	})
}

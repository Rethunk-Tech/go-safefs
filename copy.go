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
	// Put writes one file; from is where its bytes live, to where they go, rel its path below src. Nil means CopyFile.
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
	put := opts.Put
	if put == nil {
		put = func(from, to, _ string) error { return CopyFile(from, to, opts) }
	}
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
		err = walkCopy(src, dst, opts.First, pass, &progress, opts.Progress, put, root, &skipped)
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

// walkCopy is one walk of CopyTree. In a two-pass copy, pass 0 takes only the first entry and pass 1 the rest.
func walkCopy(src, dst, first string, pass int, progress *CopyProgress, report func(CopyProgress), put func(from, to, rel string) error, root string, skipped *[]string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if first != "" && p != src {
			top, relErr := filepath.Rel(src, p)
			if relErr != nil {
				return relErr
			}
			if (strings.SplitN(filepath.ToSlash(top), "/", 2)[0] == first) != (pass == 0) {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
		}
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if p != src && !RealDirUnder(root, p) {
				return fs.SkipDir
			}
			rel, err := filepath.Rel(src, p)
			if err != nil {
				return err
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o750)
		}
		if LinkedDir(p, info) {
			*skipped = append(*skipped, p)
			return nil
		}
		resolved, err := EvalSymlinks(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.Mode()&os.ModeSymlink != 0 {
			if !UnderRoot(root, resolved) {
				return fmt.Errorf("%s escapes %s", p, src)
			}
			if err := put(resolved, target, rel); err != nil {
				return err
			}
			if report != nil {
				progress.Files++
				info, err := os.Stat(resolved)
				if err != nil {
					return err
				}
				progress.Bytes += info.Size()
				report(*progress)
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", p)
		}
		if !UnderRoot(root, resolved) {
			return fmt.Errorf("%s escapes %s", p, src)
		}
		if err := put(p, target, rel); err != nil {
			return err
		}
		if report != nil {
			progress.Files++
			progress.Bytes += info.Size()
			report(*progress)
		}
		return nil
	})
}

// UnderRoot reports whether p is root or a path still inside it after both have been cleaned.
func UnderRoot(root, p string) bool {
	root = filepath.Clean(root)
	p = filepath.Clean(p)
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
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
	root, err := EvalSymlinks(src)
	if err != nil {
		return err
	}
	return filepath.WalkDir(src, func(p string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			if p != src && !RealDirUnder(root, p) {
				return fs.SkipDir
			}
			return os.MkdirAll(target, 0o750)
		}
		if LinkedDir(p, info) || (opts.Skip != nil && opts.Skip(info.Name())) {
			return nil
		}
		from := p
		if info.Mode()&os.ModeSymlink != 0 {
			if from, err = EvalSymlinks(p); errors.Is(err, fs.ErrNotExist) {
				return &DanglingLinkError{Rel: rel, Err: err}
			} else if err != nil {
				return err
			}
			if info, err = os.Stat(from); err != nil {
				return err
			}
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", p)
		}
		return CopyFile(from, target, opts)
	})
}

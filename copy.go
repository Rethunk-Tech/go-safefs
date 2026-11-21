package safefs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// CopyProgress counts files and bytes while CopyTree runs.
type CopyProgress struct {
	Files      int
	TotalFiles int
	Bytes      int64
	TotalBytes int64
}

// CopyOptions tunes CopyFile, CopyTree and CopyTreeFollowingLinks. The zero value is the default for every field.
// Each function reads only the fields it names: CopyFile uses TempSuffix; CopyTree uses TempSuffix, First, Progress
// and Put; CopyTreeFollowingLinks uses TempSuffix and Skip, and ignores First, Progress and Put.
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
	// Skip names entries CopyTreeFollowingLinks leaves out; CopyTree ignores it.
	Skip func(name string) bool
}

// ErrNestedCopy is CopyTree's and CopyTreeFollowingLinks' refusal of a destination inside the source, which would
// recurse until the disk is full, and of one equal to the source when Put is set.
var ErrNestedCopy = errors.New("destination is inside the source")

// resolveAhead resolves p's links through its deepest existing ancestor, so a destination that does not exist yet
// still compares against the source by where it will land.
func resolveAhead(p string) (string, error) {
	p, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	var tail []string
	for {
		if r, err := EvalSymlinks(p); err == nil {
			return filepath.Join(append([]string{r}, tail...)...), nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", fs.ErrNotExist
		}
		tail = append([]string{filepath.Base(p)}, tail...)
		p = parent
	}
}

// checkNotNested returns ErrNestedCopy when dst lies inside root (the resolved source), or equals it with put set.
func checkNotNested(root, dst string, put bool) error {
	d, err := resolveAhead(dst)
	if err != nil {
		return err
	}
	if UnderRoot(root, d) && (put || FoldCase(filepath.Clean(root)) != FoldCase(filepath.Clean(d))) {
		return fmt.Errorf("%s into %s: %w", root, dst, ErrNestedCopy)
	}
	return nil
}

func (o CopyOptions) tempSuffix() string {
	if o.TempSuffix == "" {
		return ".safefs-tmp"
	}
	return o.TempSuffix
}

// CopyTree copies the regular files and folders under src into dst (which may exist), file by file. Each file is
// still renamed into place whole, but none is fsynced on its own: the copy is flushed once at the end (syncfs on
// Linux, else each written file and folder once), which is what makes a thousand small files cost one flush. Until
// then dst holds CopyMarker, so a crash is told from a finished copy by CopyInterrupted(dst); the same call again
// replaces every file and clears it. Put's own writes are flushed by that final syncfs on Linux only.
// Directory junctions, mount points and symlink directories are not followed, and are returned in skipped; so is a
// top-level entry named CopyMarker, which the copy's own marker owns. Symlink files are copied by content when they
// still resolve under src, and are an error when they escape.
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
	if err := checkNotNested(root, dst, opts.Put != nil); err != nil {
		return nil, err
	}
	var progress CopyProgress
	if opts.Progress != nil {
		progress.TotalBytes, err = SizeEachName(src)
		if err != nil {
			return nil, err
		}
		err = filepath.WalkDir(src, func(_ string, d fs.DirEntry, walkErr error) error { //nolint:gosec // G703: the caller names the path; taking one is this package's contract
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
	var batch syncBatch
	dstr, err := beginCopy(dst, &batch)
	if err != nil {
		return nil, err
	}
	defer func() { _ = dstr.Close() }()
	passes := 1
	if opts.First != "" {
		passes = 2
	}
	for pass := 0; pass < passes && err == nil; pass++ {
		err = walkCopy(r, dstr, src, dst, opts, pass, &progress, opts.Put, root, &skipped, &batch)
	}
	if err == nil {
		err = batch.finish(dst)
	}
	return skipped, err
}

// beginCopy creates dst and starts the batch that flushes the copy into it. The returned root is how every nested
// dest write is confined: a symlink folder already in dst cannot redirect them out of it.
func beginCopy(dst string, batch *syncBatch) (*os.Root, error) {
	if err := os.MkdirAll(dst, 0o750); err != nil { //nolint:gosec // G703: the caller names the path; taking one is this package's contract
		return nil, err
	}
	if err := batch.begin(dst); err != nil {
		return nil, err
	}
	return os.OpenRoot(dst)
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
	st, err := os.Stat(p) //nolint:gosec // G703: the caller names the path; taking one is this package's contract
	return err == nil && st.IsDir()
}

// openIn opens the file rel (a slash path) below r, refusing a link that leads out of it. A Windows runner has
// answered not-exist for a file that is there, so that one answer is checked against the plain path. O_NONBLOCK keeps
// a pipe swapped in after the walk's check from blocking the open; the caller checks the handle's type.
func openIn(r *os.Root, src, rel string) (*os.File, error) {
	f, err := r.OpenFile(rel, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if errors.Is(err, fs.ErrNotExist) {
		if p := filepath.Join(src, filepath.FromSlash(rel)); pathExists(p) {
			return openUnder(src, p)
		}
	}
	return f, err
}

// openUnder opens p by path, which the root cannot confine, and then checks that the file it holds lies under src: a
// folder swapped for a link around the open is refused instead of read.
func openUnder(src, p string) (*os.File, error) {
	f, err := OpenRegular(p)
	if err != nil {
		return nil, err
	}
	if !heldUnder(src, p, f) {
		_ = f.Close()
		return nil, fmt.Errorf("%s escapes %s", p, src)
	}
	return f, nil
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
func walkCopy(r, dstr *os.Root, src, dst string, opts CopyOptions, pass int, progress *CopyProgress, put func(from, to, rel string) error, root string, skipped *[]string, batch *syncBatch) error {
	return fs.WalkDir(r.FS(), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if rel == "." {
			return nil // beginCopy made it
		}
		if rel == CopyMarker {
			// dst's own marker owns that name; copying the source's entry would be undone by finish's removal.
			*skipped = append(*skipped, filepath.Join(src, CopyMarker))
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
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
			if mountedDir(src, p) {
				*skipped = append(*skipped, p)
				return fs.SkipDir
			}
			if !RealDirUnder(root, p) {
				return fs.SkipDir
			}
			batch.dir(target)
			return dstr.MkdirAll(rel, 0o750)
		}
		if LinkedDir(p, info) {
			*skipped = append(*skipped, p)
			return nil
		}
		return copyEntry(r, dstr, src, root, rel, p, target, info, opts, progress, put, batch)
	})
}

// copyEntry copies one non-directory entry of walkCopy: the open vets it, then it is written by copyFrom or put.
func copyEntry(r, dstr *os.Root, src, root, rel, p, target string, info fs.FileInfo, opts CopyOptions, progress *CopyProgress, put func(from, to, rel string) error, batch *syncBatch) error {
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
		if err := copyFromRoot(dstr, f, rel, target, opts, batch); err != nil {
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
}

// UnderRoot reports whether p is root or a path still inside it after both have been cleaned.
func UnderRoot(root, p string) bool {
	root = filepath.Clean(root)
	p = filepath.Clean(p)
	rel, err := filepath.Rel(root, p)
	return err == nil && filepath.IsLocal(rel)
}

// RealDirUnder reports whether p is a directory whose own path still resolves under root (itself resolved), not a
// symlink directory or a junction/reparse that points somewhere else.
func RealDirUnder(root, p string) bool {
	info, err := os.Lstat(p) //nolint:gosec // G703: the caller names the path; taking one is this package's contract
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
	realRoot, err := EvalSymlinks(root)
	return err == nil && UnderRoot(realRoot, resolved)
}

// CopyFile copies the regular file src to dst, replacing what is there. The bytes go to a temporary file beside dst,
// are flushed to disk, and only then renamed into place, so a crash leaves dst whole or absent, never truncated.
func CopyFile(src, dst string, opts CopyOptions) (err error) {
	in, err := OpenRegular(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	return copyFromRoot(nil, in, "", dst, opts, nil)
}

// copyFromRoot is CopyFile's body over an already open source. A non-nil dstr writes rel under that dest root so a
// symlink folder in dst cannot redirect the temp file or the rename out of it; nil dstr writes dst by path, as
// CopyFile does. A non-nil batch skips the per-file fsync and records the file for the batch's final flush instead.
func copyFromRoot(dstr *os.Root, in *os.File, rel, dst string, opts CopyOptions, batch *syncBatch) (err error) {
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s: %w", in.Name(), ErrNotRegular)
	}
	tmp := dst + opts.tempSuffix()
	tmpRel := rel + opts.tempSuffix()
	removeTmp := func() {
		if dstr != nil {
			_ = dstr.Remove(tmpRel)
			return
		}
		_ = Remove(tmp)
	}
	removeTmp()
	// The copy keeps the source's exec and read bits but never grants write to group or others. The owner always
	// keeps write: a read-only copy is a read-only attribute on Windows, and renaming over it later fails.
	perm := info.Mode().Perm()&0o755 | 0o200
	var out *os.File
	if dstr != nil {
		out, err = dstr.OpenFile(tmpRel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	} else {
		out, err = CreateExcl(tmp, perm)
	}
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			removeTmp()
		}
	}()
	if _, err = io.Copy(out, in); err == nil && batch == nil {
		err = out.Sync()
	}
	if err = errors.Join(err, out.Close()); err != nil {
		return err
	}
	if dstr != nil {
		err = dstr.Rename(tmpRel, rel)
	} else {
		err = Rename(tmp, dst)
	}
	if err == nil && batch != nil {
		batch.file(dst)
	}
	return err
}

// CopyFileNoSync is CopyFile without the fsync: dst is renamed into place whole but its bytes may not be on disk
// until the caller runs Flush over it, so after a crash it can be short or empty. For a caller that places many files,
// records progress only after one Flush, and redoes whatever the record does not cover.
func CopyFileNoSync(src, dst string, opts CopyOptions) error {
	in, err := OpenRegular(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	return copyFromRoot(nil, in, "", dst, opts, &syncBatch{})
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
// skipped, returned in skipped as CopyTree does, and anything that is not a regular file is refused. A link to a
// missing file is a *DanglingLinkError.
func CopyTreeFollowingLinks(src, dst string, opts CopyOptions) (skipped []string, err error) {
	r, err := os.OpenRoot(src)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	root, err := EvalSymlinks(src)
	if err != nil {
		return nil, err
	}
	if err := checkNotNested(root, dst, false); err != nil {
		return nil, err
	}
	var batch syncBatch
	dstr, err := beginCopy(dst, &batch)
	if err != nil {
		return nil, err
	}
	defer func() { _ = dstr.Close() }()
	err = fs.WalkDir(r.FS(), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if rel == "." {
			return nil // beginCopy made it
		}
		if rel == CopyMarker {
			// dst's own marker owns that name; copying the source's entry would be undone by finish's removal.
			skipped = append(skipped, filepath.Join(src, CopyMarker))
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
			if mountedDir(src, p) {
				skipped = append(skipped, p)
				return fs.SkipDir
			}
			if !RealDirUnder(root, p) {
				return fs.SkipDir
			}
			batch.dir(target)
			return dstr.MkdirAll(rel, 0o750)
		}
		if opts.Skip != nil && opts.Skip(info.Name()) {
			return nil
		}
		if LinkedDir(p, info) {
			skipped = append(skipped, p)
			return nil
		}
		return copyFollowing(r, dstr, src, rel, p, target, info, opts, &batch)
	})
	if err == nil {
		err = batch.finish(dst)
	}
	return skipped, err
}

// copyFollowing copies one non-directory entry of CopyTreeFollowingLinks, following a file link by path.
func copyFollowing(r, dstr *os.Root, src, rel, p, target string, info fs.FileInfo, opts CopyOptions, batch *syncBatch) error {
	if info.Mode()&os.ModeSymlink != 0 {
		// A file link is followed wherever it points, by design, so it is read by path.
		from, err := EvalSymlinks(p)
		if errors.Is(err, fs.ErrNotExist) {
			return &DanglingLinkError{Rel: filepath.FromSlash(rel), Err: err}
		} else if err != nil {
			return err
		}
		in, err := OpenRegular(from)
		if errors.Is(err, ErrNotRegular) {
			return fmt.Errorf("%s is not a regular file", p)
		} else if err != nil {
			return err
		}
		defer func() { _ = in.Close() }()
		return copyFromRoot(dstr, in, rel, target, opts, batch)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", p)
	}
	f, err := openIn(r, src, rel)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return copyFromRoot(dstr, f, rel, target, opts, batch)
}

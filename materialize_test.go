package safefs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
)

func seedTree(t *testing.T, dir string) {
	t.Helper()
	files := map[string]string{
		"dll/mod.dll":          "dll-bytes",
		"A/config.json":        `{"a":1}`,
		"A/data/save.json":     `{"s":1}`,
		"A/saves/slot.dat":     "slot",
		"A/readme.txt":         "readme",
		"A/nested/config.json": `{"n":1}`,
		"A/mod.dll.old":        "old",
	}
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		writeAt(t, p, body)
	}
}

func readAll(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		var b []byte
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, linkErr := os.Readlink(p)
			if linkErr != nil {
				return linkErr
			}
			b, err = ReadFile(target)
		} else {
			b, err = ReadFile(p)
		}
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func linked(t *testing.T, a, b string) bool {
	t.Helper()
	ia, err := os.Lstat(a)
	if err != nil {
		t.Fatal(err)
	}
	ib, err := os.Lstat(b)
	if err != nil {
		t.Fatal(err)
	}
	if ia.Mode()&os.ModeSymlink != 0 || ib.Mode()&os.ModeSymlink != 0 {
		return true
	}
	return os.SameFile(ia, ib)
}

// shareable keeps config.json and the folders named data off the link tiers, as a caller's policy hook would.
func shareable(rel string) bool {
	return !strings.HasSuffix(rel, "config.json") && !strings.Contains(rel, "data/") && !strings.HasSuffix(rel, ".txt")
}

func opsFor(t *testing.T, tier Tier) (Materializer, tierOps) {
	t.Helper()
	m := Materializer{Shareable: shareable}
	fail := func(string, string) error { return syscall.EOPNOTSUPP }
	ops := m.defaultOps()
	ops.clone, ops.hardlink, ops.symlink = fail, fail, fail
	switch tier {
	case TierClone:
		m.Tiers, ops.clone = []Tier{TierClone}, func(s, d string) error { return CopyFile(s, d, m.Copy) }
	case TierHardlink:
		m.Tiers, ops.hardlink = []Tier{TierHardlink, TierCopy}, os.Link
	case TierSymlink:
		m.Tiers, ops.symlink = []Tier{TierSymlink, TierCopy}, m.defaultOps().symlink
	case TierCopy:
		m.Tiers = []Tier{TierCopy}
	default:
		t.Fatalf("tier %d", tier)
	}
	return m, ops
}

// treeWith is Materializer.Tree with injected tier funcs and the capability memory off.
func treeWith(m Materializer, ops tierOps, src, dst string) error {
	_, err := CopyTree(src, dst, CopyOptions{Put: func(from, to, rel string) error { return m.put(ops, false, from, to, rel) }})
	return err
}

func TestMaterializeTiersMatchContent(t *testing.T) {
	src := t.TempDir()
	seedTree(t, src)
	want := readAll(t, src)
	for _, tier := range []Tier{TierClone, TierHardlink, TierSymlink, TierCopy} {
		dst := t.TempDir()
		m, ops := opsFor(t, tier)
		if err := treeWith(m, ops, src, dst); err != nil {
			t.Fatalf("tier %d: %v", tier, err)
		}
		got := readAll(t, dst)
		if len(got) != len(want) {
			t.Fatalf("tier %d files %d want %d", tier, len(got), len(want))
		}
		for k, v := range want {
			if got[k] != v {
				t.Fatalf("tier %d %s = %q want %q", tier, k, got[k], v)
			}
		}
	}
}

func TestShareableFilesNeverHardlinkedOrSymlinked(t *testing.T) {
	src := t.TempDir()
	seedTree(t, src)
	for _, tier := range []Tier{TierHardlink, TierSymlink} {
		dst := t.TempDir()
		m, ops := opsFor(t, tier)
		if err := treeWith(m, ops, src, dst); err != nil {
			t.Fatal(err)
		}
		for _, rel := range []string{"A/config.json", "A/nested/config.json", "A/data/save.json", "A/readme.txt"} {
			from := filepath.Join(src, filepath.FromSlash(rel))
			to := filepath.Join(dst, filepath.FromSlash(rel))
			if linked(t, from, to) {
				t.Fatalf("tier %d linked %s", tier, rel)
			}
		}
		if !linked(t, filepath.Join(src, "dll", "mod.dll"), filepath.Join(dst, "dll", "mod.dll")) {
			t.Fatalf("tier %d did not link the dll", tier)
		}
	}
}

func TestWritingACopyLeavesSourceAndPeerUnchanged(t *testing.T) {
	src := t.TempDir()
	seedTree(t, src)
	orig := readAll(t, src)
	for _, tier := range []Tier{TierClone, TierHardlink, TierSymlink, TierCopy} {
		a, b := t.TempDir(), t.TempDir()
		m, ops := opsFor(t, tier)
		if err := treeWith(m, ops, src, a); err != nil {
			t.Fatal(err)
		}
		if err := treeWith(m, ops, src, b); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(a, "A", "config.json"), []byte(`{"a":99}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := readAll(t, src); got["A/config.json"] != orig["A/config.json"] {
			t.Fatalf("tier %d mutated the source", tier)
		}
		if got := readAll(t, b); got["A/config.json"] != orig["A/config.json"] {
			t.Fatalf("tier %d mutated the peer", tier)
		}
	}
}

func TestMaterializeFallbackChain(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, "f.bin", "x")
	var seq []string
	track := func(name string, err error) func(string, string) error {
		return func(s, d string) error {
			seq = append(seq, name)
			if err != nil {
				return err
			}
			return CopyFile(s, d, CopyOptions{})
		}
	}
	m := Materializer{Shareable: shareable}
	ops := tierOps{
		clone:    track("clone", errNoClone),
		hardlink: track("hardlink", errCrossDevice),
		symlink:  track("symlink", errBadLink),
		copy:     track("copy", nil),
	}
	if err := treeWith(m, ops, src, dst); err != nil {
		t.Fatal(err)
	}
	if string(mustRead(t, filepath.Join(dst, "f.bin"))) != "x" {
		t.Fatal("missing copy")
	}
	if want := []string{"clone", "hardlink", "symlink", "copy"}; !slices.Equal(seq, want) {
		t.Fatalf("seq %v want %v", seq, want)
	}
	cfgSrc := t.TempDir()
	writeFile(t, cfgSrc, "config.json", "{}")
	seq = nil
	if err := treeWith(m, ops, cfgSrc, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	for _, s := range seq {
		if s == "hardlink" || s == "symlink" {
			t.Fatalf("an unshareable file used %s: %v", s, seq)
		}
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestMaterializeFallbackNonFallbackError(t *testing.T) {
	src := t.TempDir()
	writeFile(t, src, "f.bin", "x")
	want := errors.New("disk")
	ops := tierOps{clone: func(string, string) error { return want }}
	err := treeWith(Materializer{Tiers: []Tier{TierClone}}, ops, src, t.TempDir())
	if !errors.Is(err, want) {
		t.Fatalf("err %v", err)
	}
}

func TestMaterializerRemembersWhatADeviceRefused(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	writeFile(t, src, "a.bin", "a")
	writeFile(t, src, "b.bin", "b")
	var cloneCalls int
	ops := tierOps{
		clone: func(string, string) error { cloneCalls++; return errNoClone },
		copy:  func(s, d string) error { return CopyFile(s, d, CopyOptions{}) },
	}
	m := Materializer{Tiers: []Tier{TierClone, TierCopy}}
	for _, rel := range []string{"a.bin", "b.bin"} {
		if err := m.put(ops, true, filepath.Join(src, rel), filepath.Join(dst, rel), rel); err != nil {
			t.Fatal(err)
		}
	}
	if cloneCalls != 1 {
		t.Fatalf("clone tried %d times; the refusal was not remembered", cloneCalls)
	}
}

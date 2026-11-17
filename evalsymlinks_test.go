package safefs

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExtendedPathOnlyPrefixesLongDrivePaths(t *testing.T) {
	long := `C:\Users\u\AppData\Local\App\blobs\` + strings.Repeat("a", 64) + `\` + strings.Repeat(`d\`, 80) + "f.dll"
	for _, tc := range []struct{ in, want string }{
		{`C:\short\path`, `C:\short\path`},
		{`\\server\share\` + strings.Repeat("x", 300), `\\server\share\` + strings.Repeat("x", 300)},
		{`\\?\C:\` + strings.Repeat("x", 300), `\\?\C:\` + strings.Repeat("x", 300)},
		{long, `\\?\` + long},
	} {
		if got := ExtendedPath(tc.in); got != tc.want {
			t.Errorf("ExtendedPath(%.40q) = %.40q, want %.40q", tc.in, got, tc.want)
		}
	}
}

func TestSamePath(t *testing.T) {
	if !SamePath(`/libs/My Folder/`, `/libs/My Folder`) {
		t.Fatal("a trailing separator made a different path")
	}
	differentCase := SamePath(`D:\Library\apps\Tool One`, `d:\library\apps\tool one`)
	if differentCase != (runtime.GOOS == "windows") {
		t.Fatalf("case-only difference: same = %v on %s", differentCase, runtime.GOOS)
	}
}

func TestTrimExtendedKeepsUNCAbsolute(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`\\?\C:\Users\u`, `C:\Users\u`},
		{`\\?\UNC\server\share\dir`, `\\server\share\dir`},
		{`C:\plain`, `C:\plain`},
		{`\\server\share`, `\\server\share`},
	} {
		if got := trimExtended(tc.in); got != tc.want {
			t.Errorf("trimExtended(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func FuzzSamePath(f *testing.F) {
	for _, pair := range [][2]string{
		{`/libs/My Folder/`, `/libs/My Folder`},
		{`D:\Library\apps\Tool One`, `d:\library\apps\tool one`},
		{"", ""},
		{".", ""},
		{"a/b/../c", "a/c"},
		{`\\server\share\dir`, `\\server\share\dir\`},
	} {
		f.Add(pair[0], pair[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		if !SamePath(a, a) {
			t.Fatalf("SamePath(%q, same) is false", a)
		}
		if SamePath(a, b) != SamePath(b, a) {
			t.Fatalf("SamePath not symmetric: %q %q", a, b)
		}
		if !SamePath(a, filepath.Clean(a)) {
			t.Fatalf("SamePath(%q, Clean) is false", a)
		}
		if SamePath(a, b) != (FoldCase(filepath.Clean(a)) == FoldCase(filepath.Clean(b))) {
			t.Fatalf("SamePath(%q, %q) disagrees with folded Clean", a, b)
		}
	})
}

func FuzzExtendedPath(f *testing.F) {
	long := `C:\Users\u\AppData\Local\App\blobs\` + strings.Repeat("a", 64) + `\` + strings.Repeat(`d\`, 80) + "f.dll"
	for _, s := range []string{`C:\short\path`, `\\server\share\` + strings.Repeat("x", 300), `\\?\C:\` + strings.Repeat("x", 300), long, "", `C:\`, strings.Repeat("a", 248)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, abs string) {
		got := ExtendedPath(abs)
		if strings.HasPrefix(abs, `\\`) || len(abs) < 248 {
			if got != abs {
				t.Fatalf("ExtendedPath(%q) = %q, want unchanged", abs, got)
			}
			return
		}
		if got != `\\?\`+abs {
			t.Fatalf("ExtendedPath of a long drive path = %q", got)
		}
		if trimExtended(got) != abs {
			t.Fatalf("trimExtended(ExtendedPath(%q)) = %q", abs, trimExtended(got))
		}
	})
}

func FuzzTrimExtended(f *testing.F) {
	for _, s := range []string{`\\?\C:\Users\u`, `\\?\UNC\server\share\dir`, `C:\plain`, `\\server\share`, `\\?\UNC\`, `\\?\`, ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		got := trimExtended(in)
		switch {
		case strings.HasPrefix(in, `\\?\UNC\`):
			if want := `\\` + strings.TrimPrefix(in, `\\?\UNC\`); got != want {
				t.Fatalf("trimExtended(%q) = %q, want %q", in, got, want)
			}
		case strings.HasPrefix(in, `\\?\`):
			if got != strings.TrimPrefix(in, `\\?\`) {
				t.Fatalf("trimExtended(%q) = %q", in, got)
			}
		default:
			if got != in {
				t.Fatalf("trimExtended(%q) = %q, want unchanged", in, got)
			}
		}
	})
}

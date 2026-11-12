package diskcache_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Rethunk-Tech/go-safefs/diskcache"
)

func FuzzPeek(f *testing.F) {
	now := epoch.Format(time.RFC3339)
	for _, body := range []string{
		`{"fetched":"2026-01-01T12:00:00Z","value":[1,2]}`,
		`{"fetched":"` + now + `","releases":[{"tag_name":"v0"}]}`,
		`{"format":2,"fetched":"` + now + `","releases":[]}`,
		`{"notes":"text"}`,
		`{"id":7,"name":"x"}`,
		`{"fetched":"` + now + `","version":"4.7.0"}`,
		`{"fetched":"` + now + `","val`,
		`{"value":"x"}`,
		`{"fetched":"` + now + `","value":null}`,
		`{"fetched":"` + now + `","value":"ok","build":"v1"}`,
	} {
		f.Add(body)
	}
	f.Fuzz(func(t *testing.T, body string) {
		c := &diskcache.Cache{Dir: t.TempDir(), Now: func() time.Time { return epoch }}
		path := filepath.Join(c.Dir, "entry")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		e, ok := diskcache.Peek[json.RawMessage](c, "entry")
		if !ok {
			return
		}
		if e.Fetched.IsZero() {
			t.Fatal("present entry with a zero fetched")
		}
		if len(e.Value)+len(e.Stamp) > len(body) {
			t.Fatalf("parsed fields longer than input: value %d stamp %d body %d", len(e.Value), len(e.Stamp), len(body))
		}
		if err := diskcache.PutEntry(c, "round", e); err != nil {
			t.Fatal(err)
		}
		got, ok := diskcache.Peek[json.RawMessage](c, "round")
		if !ok {
			t.Fatal("round-trip missed a present entry")
		}
		if !got.Fetched.Equal(e.Fetched) || got.Stamp != e.Stamp || !sameJSON(got.Value, e.Value) {
			t.Fatalf("round-trip = %+v, want %+v", got, e)
		}
	})
}

// sameJSON compares values by meaning: encoding/json rewrites a RawMessage's HTML characters as \u escapes on write.
func sameJSON(a, b json.RawMessage) bool {
	if string(a) == string(b) {
		return true
	}
	var x, y any
	return json.Unmarshal(a, &x) == nil && json.Unmarshal(b, &y) == nil && reflect.DeepEqual(x, y)
}

func FuzzPath(f *testing.F) {
	for _, name := range []string{"../escape.json", "sub/ok.json", "", "ok", "a/b/c", "/abs.json", `..\..\escape`} {
		f.Add(name)
	}
	f.Fuzz(func(t *testing.T, name string) {
		parent := t.TempDir()
		dir := filepath.Join(parent, "cache")
		c := &diskcache.Cache{Dir: dir}
		p := c.Path(name)
		if p != "" {
			rel, err := filepath.Rel(dir, p)
			if err != nil || !filepath.IsLocal(rel) || strings.HasPrefix(p, parent+string(os.PathSeparator)) && !strings.HasPrefix(p, dir+string(os.PathSeparator)) && p != dir {
				t.Fatalf("Path(%q) = %q escaped %q", name, p, dir)
			}
			if filepath.IsAbs(name) {
				t.Fatalf("Path(%q) = %q for an absolute name", name, p)
			}
		}
		_ = diskcache.Put(c, name, 1)
		entries, err := os.ReadDir(parent)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Name() != "cache" {
				t.Fatalf("Put(%q) wrote %s beside the cache", name, e.Name())
			}
		}
	})
}

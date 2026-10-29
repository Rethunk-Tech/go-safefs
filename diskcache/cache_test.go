package diskcache_test

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Rethunk-Tech/go-safefs/diskcache"
)

var epoch = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newCache(t *testing.T) (*diskcache.Cache, *clock) {
	t.Helper()
	clk := &clock{t: epoch}
	return &diskcache.Cache{Dir: t.TempDir(), Now: clk.now}, clk
}

func counting(v string, err error) (func() (string, error), *int) {
	n := 0
	return func() (string, error) { n++; return v, err }, &n
}

func TestCachedServesAYoungEntryAndFetchesAfterTTL(t *testing.T) {
	t.Parallel()
	c, clk := newCache(t)
	fetch, calls := counting("one", nil)
	for range 2 {
		if v, err := diskcache.Cached(c, "k", time.Hour, fetch); err != nil || v != "one" {
			t.Fatalf("got %q, %v", v, err)
		}
	}
	if *calls != 1 {
		t.Fatalf("fetched %d times, want 1", *calls)
	}
	clk.t = epoch.Add(2 * time.Hour)
	if _, err := diskcache.Cached(c, "k", time.Hour, fetch); err != nil || *calls != 2 {
		t.Fatalf("stale entry not refetched: %d, %v", *calls, err)
	}
}

func TestFailedFetchFallsBackToTheStaleEntryElseErrors(t *testing.T) {
	t.Parallel()
	c, clk := newCache(t)
	boom := errors.New("offline")
	failing, _ := counting("", boom)
	if _, err := diskcache.Cached(c, "k", time.Hour, failing); !errors.Is(err, boom) {
		t.Fatalf("no entry: err = %v, want the fetch error", err)
	}
	diskcache.Put(c, "k", "old")
	clk.t = epoch.Add(2 * time.Hour)
	if v, err := diskcache.Cached(c, "k", time.Hour, failing); err != nil || v != "old" {
		t.Fatalf("stale fallback: %q, %v", v, err)
	}
}

func TestStampMismatchRefetchesYetStandsIn(t *testing.T) {
	t.Parallel()
	c, _ := newCache(t)
	if _, err := diskcache.CachedStamped(c, "k", "v1", time.Hour, func() (string, error) { return "a", nil }); err != nil {
		t.Fatal(err)
	}
	fetch, calls := counting("b", nil)
	if v, _ := diskcache.CachedStamped(c, "k", "v2", time.Hour, fetch); v != "b" || *calls != 1 {
		t.Fatalf("mismatch served %q after %d fetches", v, *calls)
	}
	// Cached is stamp "": the "v2" entry is refetched, and serves when the fetch fails.
	failing, _ := counting("", errors.New("offline"))
	if v, err := diskcache.Cached(c, "k", time.Hour, failing); err != nil || v != "b" {
		t.Fatalf("stamped entry did not stand in: %q, %v", v, err)
	}
	if e, ok := diskcache.Peek[string](c, "k"); !ok || e.Stamp != "v2" || e.Value != "b" {
		t.Fatalf("Peek = %+v, %v", e, ok)
	}
}

func TestFutureStampedEntryIsNotYoung(t *testing.T) {
	t.Parallel()
	c, _ := newCache(t)
	diskcache.PutEntry(c, "k", diskcache.Entry[string]{Fetched: epoch.Add(time.Hour), Value: "future"})
	if diskcache.Fresh[string](c, "k", 24*time.Hour) {
		t.Fatal("an entry from the future counted as fresh")
	}
	fetch, calls := counting("now", nil)
	if v, _ := diskcache.Cached(c, "k", 24*time.Hour, fetch); v != "now" || *calls != 1 {
		t.Fatalf("served %q after %d fetches", v, *calls)
	}
}

func TestEmptyDirAlwaysFetches(t *testing.T) {
	t.Parallel()
	c := &diskcache.Cache{}
	fetch, calls := counting("x", nil)
	for range 2 {
		if _, err := diskcache.Cached(c, "k", time.Hour, fetch); err != nil {
			t.Fatal(err)
		}
	}
	diskcache.Put(c, "k", "x")
	if _, ok := diskcache.Peek[string](c, "k"); ok || *calls != 2 || c.Path("k") != "" {
		t.Fatalf("disabled cache kept state: calls %d, path %q", *calls, c.Path("k"))
	}
}

func TestPutPeekRoundTripWritesTheContractKeys(t *testing.T) {
	t.Parallel()
	c, _ := newCache(t)
	diskcache.Put(c, "sub/k", []int{1, 2})
	e, ok := diskcache.Peek[[]int](c, "sub/k")
	if !ok || len(e.Value) != 2 || !e.Fetched.Equal(epoch) || e.Stamp != "" {
		t.Fatalf("Peek = %+v, %v", e, ok)
	}
	b, err := os.ReadFile(c.Path("sub/k"))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"fetched":"2026-01-01T12:00:00Z","value":[1,2]}`; string(b) != want {
		t.Fatalf("file = %s, want %s", b, want)
	}
}

func TestOtherShapesReadAsAbsentAndNullAsPresent(t *testing.T) {
	t.Parallel()
	c, _ := newCache(t)
	now := epoch.Format(time.RFC3339)
	for name, body := range map[string]string{
		"releases-old":    `{"fetched":"` + now + `","releases":[{"tag_name":"v0"}]}`,
		"releases-format": `{"format":2,"fetched":"` + now + `","releases":[]}`,
		"notes":           `{"notes":"text"}`,
		"bare":            `{"id":7,"name":"x"}`,
		"version":         `{"fetched":"` + now + `","version":"4.7.0"}`,
		"torn":            `{"fetched":"` + now + `","val`,
		"no-fetched":      `{"value":"x"}`,
	} {
		if err := os.WriteFile(filepath.Join(c.Dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if e, ok := diskcache.Peek[string](c, name); ok {
			t.Errorf("%s read as present: %+v", name, e)
		}
	}
	if err := os.WriteFile(filepath.Join(c.Dir, "null"), []byte(`{"fetched":"`+now+`","value":null}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := diskcache.Peek[[]string](c, "null"); !ok {
		t.Error(`a stored "value": null must read as present`)
	}
}

func TestConcurrentUse(t *testing.T) {
	t.Parallel()
	c, _ := newCache(t)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for range 20 {
				diskcache.Put(c, "shared", i)
				_, _ = diskcache.Cached(c, "shared", time.Hour, func() (int, error) { return i, nil })
				diskcache.Peek[int](c, "shared")
			}
		})
	}
	wg.Wait()
}

func TestANameOutsideDirNeverReachesAFile(t *testing.T) {
	parent := t.TempDir()
	c := &diskcache.Cache{Dir: filepath.Join(parent, "cache")}
	for _, name := range []string{"../escape.json", filepath.Join(parent, "abs.json"), ""} {
		if p := c.Path(name); p != "" {
			t.Fatalf("Path(%q) = %q, want \"\"", name, p)
		}
		diskcache.Put(c, name, 1)
	}
	if p := c.Path("sub/ok.json"); p != filepath.Join(parent, "cache", "sub", "ok.json") {
		t.Fatalf("a nested local name = %q", p)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("a write escaped the cache: %v %v", entries, err)
	}
}

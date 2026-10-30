// Package diskcache keeps fetched answers as JSON files in a directory. A crash or an unreadable file only costs a
// refetch: the cache itself never returns an error, and a stale entry stands in when a fetch fails.
package diskcache

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Rethunk-Tech/go-safefs"
)

// Entry is one cached answer. The JSON keys are a format contract: other tools read "fetched" to expire files.
type Entry[T any] struct {
	Fetched time.Time `json:"fetched"`
	Value   T         `json:"value"`
	// Stamp is who wrote the entry (a build, a format number); CachedStamped refetches on a mismatch.
	Stamp string `json:"build,omitempty"`
}

// ErrNoDir is returned by Cached, CachedStamped, Put and PutEntry for a non-nil Cache whose Dir is empty. That is a
// caller bug (an unresolved data folder), not a request for no caching.
var ErrNoDir = errors.New("diskcache: Cache.Dir is empty")

// Cache is a directory of entries, one file per name. A nil *Cache is the way to run without a cache: Cached and
// CachedStamped always fetch, Peek and Fresh find nothing, Put and PutEntry do nothing.
type Cache struct {
	// Dir is where entries live. It must not be empty; Peek finds nothing and the writers return ErrNoDir otherwise.
	Dir string
	// Now is the clock; nil means time.Now().UTC().
	Now func() time.Time
}

func (c *Cache) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now().UTC()
}

// Path is the file holding name, or "" for a nil cache or an empty Dir. A name that is not a local path under Dir
// (filepath.IsLocal: absolute, or leading out with "..") also gets "", so a key built from untrusted input can
// cost a refetch but never reach a file outside the cache.
func (c *Cache) Path(name string) string {
	if c == nil || c.Dir == "" || !filepath.IsLocal(name) {
		return ""
	}
	return filepath.Join(c.Dir, name)
}

func read[T any](c *Cache, name string) (Entry[T], bool) {
	path := c.Path(name)
	if path == "" {
		return Entry[T]{}, false
	}
	b, err := safefs.ReadFile(path)
	if err != nil {
		return Entry[T]{}, false
	}
	// An entry with no "value" key is another shape under the same name (an older version's, or a foreign file), and
	// decoding it would serve a zero value as a fresh answer. A stored "value": null is still a value.
	var raw struct {
		Fetched time.Time       `json:"fetched"`
		Value   json.RawMessage `json:"value"`
		Stamp   string          `json:"build"`
	}
	if json.Unmarshal(b, &raw) != nil || raw.Fetched.IsZero() || raw.Value == nil {
		return Entry[T]{}, false
	}
	e := Entry[T]{Fetched: raw.Fetched, Stamp: raw.Stamp}
	if json.Unmarshal(raw.Value, &e.Value) != nil {
		return Entry[T]{}, false
	}
	return e, true
}

// write replaces the file without fsync: a cache that cannot be written only costs a refetch, and one a crash cuts
// short fails to parse and reads as absent, while a cold start writes hundreds of entries at once.
func write[T any](c *Cache, name string, e Entry[T]) {
	path := c.Path(name)
	if path == "" {
		return
	}
	b, err := json.Marshal(e)
	if err != nil || os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	_ = safefs.WriteStreamNoSync(path, 0o600, func(w io.Writer) error {
		_, err := w.Write(b)
		return err
	})
}

// Cached returns the entry at name while it is younger than ttl, else fetches; a failed fetch falls back to a stale
// entry, and errors only when there is none. A file that is missing, unparsable, has no "fetched" or no "value" key
// reads as absent. An entry stamped in the future (a clock set back) is not young. Cached is CachedStamped with stamp
// "": an entry written with a stamp is refetched, and still stands in when the fetch fails.
func Cached[T any](c *Cache, name string, ttl time.Duration, fetch func() (T, error)) (T, error) {
	return CachedStamped(c, name, "", ttl, fetch)
}

// CachedStamped is Cached that also refetches an entry whose Stamp is not stamp; that entry still stands in when the
// fetch fails.
func CachedStamped[T any](c *Cache, name, stamp string, ttl time.Duration, fetch func() (T, error)) (T, error) {
	if c == nil {
		return fetch()
	}
	if c.Dir == "" {
		var zero T
		return zero, ErrNoDir
	}
	if c.Path(name) == "" {
		return fetch()
	}
	old, ok := read[T](c, name)
	if ok && c.young(old.Fetched, ttl) && old.Stamp == stamp {
		return old.Value, nil
	}
	v, err := fetch()
	if err != nil {
		if ok {
			return old.Value, nil
		}
		var zero T
		return zero, err
	}
	write(c, name, Entry[T]{Fetched: c.now(), Value: v, Stamp: stamp})
	return v, nil
}

// Peek returns the entry at name however old, without fetching.
func Peek[T any](c *Cache, name string) (Entry[T], bool) { return read[T](c, name) }

// Fresh reports whether the entry at name exists and is younger than ttl.
func Fresh[T any](c *Cache, name string, ttl time.Duration) bool {
	e, ok := read[T](c, name)
	return ok && c.young(e.Fetched, ttl)
}

// young reports whether an entry fetched at t is younger than ttl; one stamped in the future (a clock set back) is not.
func (c *Cache) young(t time.Time, ttl time.Duration) bool {
	age := c.now().Sub(t)
	return age >= 0 && age < ttl
}

// Put stores value at name as if it had just been fetched, unstamped. It returns ErrNoDir for an empty Dir; a nil
// cache is a no-op, and a write that fails only costs a refetch.
func Put[T any](c *Cache, name string, value T) error {
	if c == nil {
		return nil
	}
	return PutEntry(c, name, Entry[T]{Fetched: c.now(), Value: value})
}

// PutEntry stores e at name as it is, for a caller that stamps or dates its own entry. Errors as Put.
func PutEntry[T any](c *Cache, name string, e Entry[T]) error {
	if c == nil {
		return nil
	}
	if c.Dir == "" {
		return ErrNoDir
	}
	write(c, name, e)
	return nil
}

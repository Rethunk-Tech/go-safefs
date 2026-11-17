package safefs

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReadCappedTakesInputUpToTheLimitAndRefusesOneByteMore(t *testing.T) {
	t.Parallel()
	if b, err := ReadCapped(strings.NewReader("abcd"), 4); err != nil || string(b) != "abcd" {
		t.Fatalf("at the limit: %q, %v", b, err)
	}
	if b, err := ReadCapped(strings.NewReader("abcde"), 4); !errors.Is(err, ErrTooLarge) || b != nil {
		t.Fatalf("one byte over: %q, %v", b, err)
	}
	if b, err := ReadCapped(strings.NewReader(""), 0); err != nil || len(b) != 0 {
		t.Fatalf("empty at a zero limit: %q, %v", b, err)
	}
}

func TestCapReaderFailsOnceTheInputRunsOver(t *testing.T) {
	t.Parallel()
	if b, err := io.ReadAll(CapReader(strings.NewReader("abcd"), 4)); err != nil || string(b) != "abcd" {
		t.Fatalf("at the limit: %q, %v", b, err)
	}
	if _, err := io.ReadAll(CapReader(strings.NewReader("abcde"), 4)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("one byte over: %v", err)
	}
}

func FuzzReadCapped(f *testing.F) {
	f.Add([]byte("abcd"), int64(4))
	f.Add([]byte("abcde"), int64(4))
	f.Add([]byte(""), int64(0))
	f.Add([]byte("x"), int64(0))
	f.Add([]byte("x"), int64(-1))
	f.Fuzz(func(t *testing.T, data []byte, limit int64) {
		if len(data) > 1<<20 {
			data = data[:1<<20]
		}
		if limit > 1<<20 {
			limit = 1 << 20
		}
		b, err := ReadCapped(bytes.NewReader(data), limit)
		held := int64(len(b))
		if limit >= 0 && held > limit {
			t.Fatalf("held %d bytes with limit %d", held, limit)
		}
		if limit < 0 {
			if len(data) > 0 && !errors.Is(err, ErrTooLarge) {
				t.Fatalf("negative limit with %d bytes: %v", len(data), err)
			}
			if held != 0 {
				t.Fatalf("negative limit held %d bytes", held)
			}
			return
		}
		if int64(len(data)) > limit {
			if !errors.Is(err, ErrTooLarge) || b != nil {
				t.Fatalf("over: %q, %v", b, err)
			}
			return
		}
		if err != nil || !bytes.Equal(b, data) {
			t.Fatalf("at or under limit: %q, %v want %q", b, err, data)
		}
	})
}

# HUMANS.md

Run and use `go-safefs`. Internals: [AGENTS.md](AGENTS.md).

## Quick start

```bash
go get github.com/Rethunk-Tech/go-safefs
```

```go
package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Rethunk-Tech/go-safefs"
	"github.com/Rethunk-Tech/go-safefs/diskcache"
)

func main() {
	// Read a file someone else placed: at most 1 MiB, and a symlinked last element is refused.
	f, err := safefs.OpenRegular("config.json")
	if err != nil {
		fmt.Println(err)
		return
	}
	data, err := safefs.ReadCapped(f, 1<<20)
	_ = f.Close()
	if errors.Is(err, safefs.ErrTooLarge) {
		fmt.Println("config.json is over 1 MiB")
		return
	}
	if err != nil {
		fmt.Println(err)
		return
	}

	// Replace a file so a crash leaves the old one or the new one, never half of either.
	if err := safefs.WriteFileAtomic("state.json", data, 0o600); err != nil {
		fmt.Println(err)
		return
	}

	// Cache a fetched answer for an hour; a failed fetch falls back to the stale entry.
	cache := &diskcache.Cache{Dir: os.TempDir()}
	version, err := diskcache.Cached(cache, "latest.json", time.Hour, func() (string, error) {
		return "1.2.3", nil
	})
	fmt.Println(version, err)
}
```

## Contracts

Every function takes OS paths; `rel` arguments are slash paths below a tree's root. A zero options field means the
default.

| Call | Contract |
| --- | --- |
| `Within`, `Open`, `OpenFile`, `Create`, `CreateExcl`, `ReadFile`, `WriteFile`, `Stat`, `Chmod`, `HashFile` | Run through an `os.Root` on the parent: a symlinked last element leading out of the parent is refused. `WriteFile` is not atomic. |
| `OpenRegular` | As `Open`, and fails with `ErrNotRegular` for a directory, device or pipe. |
| `IsFile`, `IsDir` | Follow a final symlink anywhere (`os.Stat`), unlike the calls above. |
| `ReadCapped`, `CapReader` | Hold at most `limit` bytes plus one probe byte; more input fails with `ErrTooLarge`. |
| `Rename`, `Remove`, `RemoveAll` | On Windows retry access-denied, sharing and lock violations for up to 2 s; elsewhere they are the `os` calls. |
| `WriteFileAtomic`, `WriteStreamAtomic` | Temp file `<base>.<random>.tmp` beside the path, fsync, rename. On failure the temp is removed and the path is untouched. The temp name is a contract. |
| `WriteStreamNoSync` | The same without fsync; a crash may leave the file torn, so read it as absent. Every atomic writer skips its fsyncs under `go test`. |
| `CopyFile` | Writes `dst+TempSuffix`, fsyncs, renames over `dst`, first removing a stale temp. Keeps the source's read and exec bits, never grants group or other write, always keeps owner write. |
| `CopyTree` | Never follows a directory link (returned in `skipped`); copies a file link by content only when it resolves under `src`, else errors. `First` copies one top-level entry first. |
| `CopyTreeFollowingLinks` | Copies a file link from wherever it points, skips linked folders and `Skip` names, refuses non-regular files; a missing target is a `*DanglingLinkError`. |
| `Clone`, `LinkUnsupported` | Reflink (Linux, Windows ReFS); `dst` is created exclusively with mode 0o600. Elsewhere `Clone` fails and `LinkUnsupported` says to fall back to a copy. |
| `Materializer` | Falls through its tiers on `LinkUnsupported`, never clones or hard-links across devices, and keeps files `Shareable` rejects off the link tiers; `Shareable` gets slash-separated paths on every OS. The refusal memory is process-wide. |
| `Size`, `SizeEachName` | Total bytes of regular files, hard links once or every name in full; unreadable entries are skipped. |
| `ResolveFold` | Exact name first, then case variants in sorted order, backtracking; at most 8 directory reads per element; `fs.ErrInvalid` unless `fs.ValidPath`, `fs.ErrNotExist` when nothing matches. |
| `Trash` | Freedesktop trash or the Recycle Bin; `errors.ErrUnsupported` on darwin. |
| `CheckWritable` | `nil` for a missing directory, else the probe's `*fs.PathError`. |
| `diskcache.Cached`, `CachedStamped` | Fetch when the entry is missing, old, from the future or stamped differently; a failed fetch falls back to the stale entry. Never returns a cache error. A name that is not local to `Dir` (absolute, or `..`) is never read or written. |
| `diskcache.Peek`, `Fresh`, `Put`, `PutEntry` | A file with no `"fetched"` or no `"value"` key reads as absent; `"value": null` is present. Write errors are dropped. |

## Requirements

Go 1.27.3 or newer, plus `lefthook` and `gitleaks` on PATH if you want the git hooks.

```bash
lefthook install
```

That is per clone; git does not commit hooks. Pre-commit scans staged changes for secrets, and
pre-push runs `gate`.

## Verify

```bash
gate
```

# AGENTS.md

Package `safefs` is file I/O for a program that writes into folders it does not own and reads files it did not write;
subpackage `diskcache` is a typed JSON cache on top of it. It imports no consumer package, no
logging or settings, and only `golang.org/x/sys` beyond the standard library.

## Gate

`gate` is the command. Each override in `.gate.toml` is there because a runner
cannot resolve that check from PATH. The `windows-latest` test job is the only place
Windows-only behaviour runs: `copy_windows_test.go`, `inode_windows_test.go`, the junction path in
`link_windows.go` and the retry in `rename_windows.go`. A real reflink needs btrfs or XFS and ReFS ids need ReFS,
so those tests probe and skip on the runners; CI proves the fallback only.

## Shape

- **Rooted wrappers mirror `os` or add a refusal.** `Open`, `OpenFile`, `Create`, `ReadFile`, `WriteFile`, `Stat`,
  `Chmod` mirror an `os` function; `OpenRegular`, `CreateExcl` add a refusal. Anything else rooted goes through
  `Within` at the call site, so the API does not grow a wrapper per `os.Root` method. Another hash needs no export:
  `HashFile` takes any `hash.Hash`.
- **`IsFile`, `IsDir`, `Size` and `Shared` follow links.** Everything else in the rooted family refuses a symlinked
  last element. Do not unify them: callers rely on each behaviour.
- **`os.Remove`, `os.RemoveAll` and `os.Rename` are forbidden** outside `rename*.go` and tests (forbidigo). Windows
  antivirus and indexer locks make a plain call fail intermittently; the retry lives in one place.
- **The atomic writers' temp name `<base>.<random>.tmp` is a contract.** Callers sweep leftovers by that shape, so it
  is not an option. `CopyFile`'s temp is `dst+TempSuffix`, removed first when stale, so a suffix must never name a
  real file.
- **`syncWrites` is off under `go test`** (`testing.Testing()`): a test cannot observe an fsync, and thousands of
  them cost seconds. It is a variable only so one test can turn it on.
- **`CapReader` probes one byte past the limit.** `io.LimitReader` ends as if the input were complete, which hides
  truncation; `ErrTooLarge` is the only signal.
- **`Materializer` keeps its device memory in a package-level `sync.Map`** (`capCache`), shared by every value, so a
  device that refused a tier is learned once per process. Tests bypass it through the unexported `put`; do not export
  that switch.
- **`ResolveFold`'s budget (8 `ReadDir` per element) is a bound, not policy.** In a tree each directory is read once;
  only links make a walk long. Exact name first, then sorted case variants, so a collision resolves the same way
  every time.
- **`diskcache` never returns a cache error.** A missing, torn, foreign or older-shape file is a miss, and an entry
  with no `"value"` key is a miss even when `"fetched"` parses. The JSON keys `fetched`, `value` and `build` are
  read by other tools; the Go field is `Stamp`, the key stays `build`.
- **The darwin build is untested here**: `Trash` returns `errors.ErrUnsupported`, and `Clone` uses `clonefile` (no
  Linux or CI run exercises it), failing with an error `LinkUnsupported` accepts so `Materializer` falls through to a copy.

## Files

| File | Holds |
| --- | --- |
| `rooted.go` | `Within` and the rooted wrappers, `HashFile`, `IsFile`, `IsDir` |
| `readcapped.go` | `ReadCapped`, `CapReader`, `ErrTooLarge` |
| `rename.go`, `rename_other.go`, `rename_windows.go` | Lock-tolerant `Rename`, `Remove`, `RemoveAll` and the retry loop |
| `samepath.go`, `foldcase_*.go`, `evalsymlinks*.go` | `SamePath`, `FoldCase`, `EvalSymlinks`, `ExtendedPath` |
| `foldpath.go` | `ResolveFold` |
| `walkfiles.go` | `WalkFiles` |
| `write.go`, `syncdir_*.go` | Atomic and no-sync writers, `SyncDir` |
| `copy.go` | `CopyOptions`, `CopyFile`, `CopyTree`, `CopyTreeFollowingLinks`, `UnderRoot`, `RealDirUnder`, `LinkedDir` |
| `clone_linux.go`, `clone_darwin.go`, `clone_windows.go`, `clone_stub.go` | `Clone`, `LinkUnsupported`, `Device` per OS |
| `inode_*.go` | `HardLinkID` and `FileID` |
| `links.go`, `links_unix.go`, `links_windows.go` | `Shared`, `Unshare` and the link-count read |
| `link_unix.go`, `link_windows.go` | `LinkDir`: symlink, or an unprivileged junction on Windows |
| `materialize.go` | `Materializer`, `Tier` and the per-device refusal memory |
| `size.go`, `free_*.go` | `Size`, `SizeEachName`, `FreeBytes` |
| `trash_freedesktop.go`, `trash_windows.go`, `trash_darwin.go` | `Trash` per OS |
| `writable.go` | `CheckWritable` |
| `diskcache/cache.go` | `Cache`, `Entry` and the cached-answer functions |

# Changelog

Notable changes to `go-safefs`. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2026-11-02

### Added

- `Clone` uses `clonefile(2)` on macOS, sharing blocks on APFS; a volume or pair of paths that cannot clone still falls back to a copy through `LinkUnsupported`.

## [0.2.0] - 2026-10-30

### Changed

- `diskcache`: an empty `Cache.Dir` no longer means "caching off", which silently disabled caches whose data folder failed to resolve. Run without a cache with a nil `*Cache` (every function accepts it). A non-nil `Cache` with an empty `Dir` makes `Cached`, `CachedStamped`, `Put` and `PutEntry` return the new `ErrNoDir`, and `Peek` find nothing. `Put` and `PutEntry` now return an `error`.

## [0.1.0] - 2026-10-29

### Added

- Parent-rooted opens and reads (`Within`, `Open`, `OpenRegular`, `ReadFile`, `HashFile`, and the rest) that refuse a symlinked last element leading out of the parent.
- `ReadCapped` and `CapReader`, byte-capped reads that fail with `ErrTooLarge` instead of ending short.
- `Rename`, `Remove` and `RemoveAll`, retrying on Windows while a scanner or indexer holds the file.
- Path helpers (`SamePath`, `FoldCase`, `EvalSymlinks`, `ExtendedPath`, `WalkFiles`) and `ResolveFold`, a backtracking case-insensitive lookup over an `fs.FS`.
- `WriteFileAtomic`, `WriteStreamAtomic`, `WriteStreamNoSync` and `SyncDir`; `Trash` and `CheckWritable`.
- `CopyFile`, `CopyTree` and `CopyTreeFollowingLinks` (with `CopyOptions` and `DanglingLinkError`), and the `UnderRoot`, `RealDirUnder` and `LinkedDir` path checks they use.
- `Clone`, `LinkUnsupported`, `Device`, `HardLinkID`, `Shared`, `Unshare` and `LinkDir`; `Materializer`, which puts files at a destination by the cheapest sharing it allows.
- `Size`, `SizeEachName` and `FreeBytes`.
- Subpackage `diskcache`: typed JSON cache entries (`Cached`, `CachedStamped`, `Peek`, `Fresh`, `Put`, `PutEntry`) where a failed fetch falls back to a stale entry and the cache itself never errors. A name that would lead outside the cache directory is treated as uncached.

[Unreleased]: https://github.com/Rethunk-Tech/go-safefs/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/Rethunk-Tech/go-safefs/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/Rethunk-Tech/go-safefs/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Rethunk-Tech/go-safefs/releases/tag/v0.1.0

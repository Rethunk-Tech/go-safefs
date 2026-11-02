# Changelog

Notable changes to `go-safefs`. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- `UnderRoot` uses `filepath.IsLocal`, so on Windows a reserved device name (`NUL`, `COM1`) below the root is no longer under it.
- `CopyTree` and `CopyTreeFollowingLinks` read the source through one `os.Root`, so a link swapped in during the copy cannot lead a read out of `src`. A file link that escapes `src` is still refused by `CopyTree`; a custom `Put` still opens `from` by path. `CopyTreeFollowingLinks` still follows file links wherever they point.
- `WalkFiles` walks with `fs.WalkDir`; a root that is a symlink is followed.

### Fixed

- `ReadDirIfExists` returns an error for a file at the path on Windows too, instead of reporting it missing.

## [0.5.1] - 2026-11-02

### Fixed

- `ReadDirIfExists` returns an error for a file at the path on Windows too, instead of reporting it missing.

## [0.5.0] - 2026-11-02

### Added

- `ReadDirIfExists`, a sorted directory listing through the parent root where a missing directory is a normal answer.

## [0.4.0] - 2026-11-02

### Added

- `ReadFileIfExists`, `ReadFile` where a missing file is a normal answer, and `WalkRegular`, a walk over the regular files under a root that tolerates a missing root and never follows links.

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

[Unreleased]: https://github.com/Rethunk-Tech/go-safefs/compare/v0.5.1...HEAD
[0.5.1]: https://github.com/Rethunk-Tech/go-safefs/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/Rethunk-Tech/go-safefs/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/Rethunk-Tech/go-safefs/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/Rethunk-Tech/go-safefs/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/Rethunk-Tech/go-safefs/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Rethunk-Tech/go-safefs/releases/tag/v0.1.0

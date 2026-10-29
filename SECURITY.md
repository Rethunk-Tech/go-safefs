# Security

## Supported versions

The latest tagged release. There are no long-lived release branches, so a fix ships in the next tag
rather than being backported.

## Reporting a vulnerability

Use GitHub's **private vulnerability reporting** on this repository (Security → Report a
vulnerability). It stays private until a fix ships.

Please do not open a public issue for a suspected vulnerability first.

## What is in scope

- Paths naming a symlink: `Open`, `ReadFile`, `Stat`, `Within` and the other rooted calls must not follow a symlinked
  last element out of its parent folder. `CopyTree` must not follow a directory link, and must refuse a file link
  that resolves outside the source.
- Input of unknown length: `ReadCapped` and `CapReader` must hold at most `limit` bytes plus one probe byte, and must
  fail with `ErrTooLarge` rather than end as if the input were complete.
- The `rel` string of `ResolveFold`: it must not return a path outside `fsys`, loop, or read more than 8 directories
  per element; anything that fails `fs.ValidPath` is refused.
- Files in a cache directory: `diskcache` must treat a torn, foreign or older-shape file as absent, never as a value.
- Files being replaced: `WriteFileAtomic`, `WriteStreamAtomic` and `CopyFile` must leave the destination whole or
  untouched on a crash or failure.

## What is out of scope

- The caller chooses `path` and the roots. `Within` confines only the last element: a symlink in an earlier element
  is followed, as the operating system does. Confine a whole tree with `os.Root`.
- `IsFile`, `IsDir`, `Shared` and `Size` follow links by design, and `CopyTreeFollowingLinks` copies a linked file
  from wherever it points.
- `CopyFile` removes a stale file named `dst+TempSuffix` before writing; choose a suffix no real file carries.
- `diskcache` drops write errors and trusts the files in its own directory to be written by one program.
- `ResolveFold` over `os.DirFS` follows links as `os.ReadDir` does; pass `os.Root.FS()` to confine it.

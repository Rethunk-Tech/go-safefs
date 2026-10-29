<h1 align="center">go-safefs</h1>

<div align="center">

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.27.3+-00ADD8?logo=go&logoColor=white)](go.mod)

</div>

---

Bounded, crash-safe file I/O for untrusted paths: capped reads, atomic writes, rooted copies, case-folded lookup and a small disk cache. Package `safefs`, module `github.com/Rethunk-Tech/go-safefs`.

For a program that writes into folders it does not own and reads files it did not write. Each call either does what
it says or fails without leaving a half-written file, a followed link, or an unbounded read behind.

## Quick start

```bash
go get github.com/Rethunk-Tech/go-safefs
```

A worked example and the contracts: [HUMANS.md](HUMANS.md).

## Features

- Opens and reads through the parent folder, so a symlinked last path element cannot lead out of it.
- Reads capped in bytes, failing with `ErrTooLarge` where `io.LimitReader` would end short.
- Rename and remove that retry on Windows while a scanner or indexer holds the file.
- Writes, copies and clones that a crash leaves whole or absent, and a materialiser that picks the cheapest sharing.
- `ResolveFold`, which finds a path written for Windows on a case-sensitive file system.
- `diskcache`, typed JSON cache files where a crash or a stale shape only costs a refetch.

`Materializer` remembers which link tiers a device refused for the whole process. Under `go test` the atomic writers
skip their fsyncs, because a test cannot observe one.

## Documentation

| Doc | Audience |
| --- | --- |
| [HUMANS.md](HUMANS.md) | Running and using the package |
| [AGENTS.md](AGENTS.md) | Internals, invariants, and why |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Process — commits, tests, docs |
| [SECURITY.md](SECURITY.md) | Vulnerability disclosure |
| [CHANGELOG.md](CHANGELOG.md) | Release notes |

## License

MIT — see [LICENSE](LICENSE). © 2026 Rethunk.Tech, LLC.

<h1 align="center">go-larian</h1>

<div align="center">

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.27.3+-00ADD8?logo=go&logoColor=white)](go.mod)

</div>

---

Readers for Larian Studios mod formats (LSPK .pak archives, meta.lsx, modsettings.lsx) as Baldur's Gate 3 uses them. Package `larian`, module `github.com/Rethunk-Tech/go-larian`.

Reads Baldur's Gate 3 mod packages without extracting them to disk, and writes the load-order file the game reads. Every size, offset, count and path in a package is treated as hostile, so a crafted `.pak` is an error, never a panic or an unbounded allocation.

## Quick start

```bash
go get github.com/Rethunk-Tech/go-larian
```

A worked example and the contracts: [HUMANS.md](HUMANS.md).

## Features

- Open an LSPK `.pak` (versions 15, 16, 18), list its files and read one by path (stored, zlib or LZ4).
- Read a mod's `Mods/<Folder>/meta.lsx` into a `ModuleInfo`.
- Split and build Larian's packed `Version64`.
- Parse and write `modsettings.lsx` load orders, base-game entries included.
- Reject unsafe stored paths (`..`, absolute, drive-lettered) when the package is opened.

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

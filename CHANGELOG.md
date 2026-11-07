# Changelog

Notable changes to `go-larian`. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.2] - 2026-11-07

### Added

- Fuzz seeds for `Open` and a `FuzzParseMeta` over `ParseMeta` and `ParseModSettings`.

### Fixed

- `pak` accepts deletion marks, refuses names Windows cannot hold and FIFOs, and bounds declared sizes.

## [0.1.1] - 2026-11-02

### Fixed

- `Read` refuses an LZ4 entry whose stored size exceeds what LZ4 can produce for its uncompressed size, before reading it.

## [0.1.0] - 2026-11-02

### Added

- Read LSPK `.pak` packages (versions 15, 16, 18; stored, zlib and LZ4 entries): `Open`, `OpenFile`, `Pak.Entries`, `Pak.Read`.
- Read a mod's `meta.lsx` into `ModuleInfo` (`ParseMeta`, `Pak.ReadModuleInfo`).
- `Version64` packing and splitting.
- Parse and write `modsettings.lsx` load orders (`ParseModSettings`, `FormatModSettings`).
- `lariantest.BuildPak`: build in-memory LSPK v15, v16 and v18 fixtures for tests.

[Unreleased]: https://github.com/Rethunk-Tech/go-larian/compare/v0.1.2...HEAD
[0.1.2]: https://github.com/Rethunk-Tech/go-larian/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/Rethunk-Tech/go-larian/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/Rethunk-Tech/go-larian/releases/tag/v0.1.0

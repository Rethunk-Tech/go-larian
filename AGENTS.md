# AGENTS.md

Package `larian` reads Larian Studios' mod formats (LSPK paks, `meta.lsx`, `modsettings.lsx`) for Baldur's Gate 3 and imports no consumer package. It holds format knowledge only; install policy, ordering decisions and filesystem layout belong to callers.

## Gate

`gate` is the command. Each override in `.gate.toml` is there because a runner
cannot resolve that check from PATH.

## Shape

- A package is hostile input: every offset, size and count is checked against the file size and the `Max*` limits before it sizes an allocation or a read. Limits bound memory, not the format.
- Paths are validated once, in `cleanPath` at `Open`; no entry with an unsafe path ever exists, so `Read` and callers can rely on it.
- `Pak.Read` is the single decompression path and always checks the output length against the recorded size.
- `FormatModSettings` is pure and never injects base entries: policy about GustavDev stays with the caller.
- `.lsx` input is size-capped before `encoding/xml` sees it; attribute values are escaped on output.

## Files

| File | Holds |
| --- | --- |
| `pak.go` | LSPK header, file table, entry reading, path validation |
| `version.go` | `Version64` packing |
| `lsx.go` | `meta.lsx` and `modsettings.lsx` parsing and writing |

# HUMANS.md

Run and use `go-larian`. Internals: [AGENTS.md](AGENTS.md).

## Quick start

```bash
go get github.com/Rethunk-Tech/go-larian
```

```go
package main

import (
	"fmt"
	"log"

	"github.com/Rethunk-Tech/go-larian"
)

func main() {
	p, err := larian.OpenFile("MyMod.pak")
	if err != nil {
		log.Fatal(err)
	}
	defer p.Close()

	m, err := p.ReadModuleInfo()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(m.Name, m.Version64)

	mods := []larian.ModuleInfo{{UUID: larian.GustavDevUUID, Name: "GustavDev", Folder: "GustavDev"}, m}
	_ = larian.FormatModSettings(mods) // bytes for modsettings.lsx
}
```

## Contracts

| Behaviour | Contract |
| --- | --- |
| `Open`, `OpenFile` | Accept LSPK 15, 16, 18; other versions give `ErrUnsupported`, malformed data `ErrInvalid`. |
| Stored paths | Slash-normalised; `..`, `.`, empty segments, absolute and drive-lettered names fail `Open`. |
| `Pak.Read` | Exact slash path; `ErrNotFound` if absent; decompressed size must match the recorded size; larger than `MaxEntrySize` fails. |
| Multi-part, Zstd entries | Listed, but `Read` returns `ErrUnsupported`. A solid package (header flag 0x04) fails `Open` with `ErrUnsupported`; patch deletion entries are skipped. |
| `ReadModuleInfo` | Uses the first `Mods/<Folder>/meta.lsx` (case-insensitive); `ErrNotFound` without one. |
| `Version64` | 9/8/16/31 bits major/minor/revision/build; `NewVersion64` truncates each part. |
| `ParseModSettings` | Load order from the `Mods` node's `ModuleShortDesc` children, in file order; base entries returned like any other. |
| `FormatModSettings` | Pure; writes exactly the mods given, so include the base module (`GustavDevUUID` or `GustavXUUID`) yourself.
| `lariantest.BuildPak` | Test helper: builds an in-memory LSPK v15, v16 or v18 from a path-to-bytes map. | |

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

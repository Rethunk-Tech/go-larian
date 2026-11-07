# Security

## Supported versions

The latest tagged release. There are no long-lived release branches, so a fix ships in the next tag
rather than being backported.

## Reporting a vulnerability

Use GitHub's **private vulnerability reporting** on this repository (Security → Report a
vulnerability). It stays private until a fix ships.

Please do not open a public issue for a suspected vulnerability first.

## What is in scope

- A `.pak` passed to `Open`, `OpenFile` and `Pak.Read`: header offsets, file-list size, entry count, per-entry offset, sizes and compressed streams are untrusted. A crafted package must not panic, read outside the package, or allocate beyond `MaxEntries` and `MaxEntrySize`.
- Stored file names: a name that is absolute, empty-segmented, contains `.` or `..`, or has a segment Windows would resolve elsewhere (a `:`, a trailing dot or space, a device name such as `CON`) must never reach a caller, so `Entry.Path` is safe to join under a directory.
- `.lsx` documents passed to `ParseMeta` and `ParseModSettings`: size-capped at 8 MiB; no external entities are resolved.

## What is out of scope

- Joining `Entry.Path` to a destination and writing it safely (symlinks in the destination, total bytes written) is the caller's job; this package never writes files.
- Total decompressed bytes across many entries are not capped; a caller extracting a whole package sets its own budget.
- Mod UUIDs, names and MD5 values are reported as stored and not verified.

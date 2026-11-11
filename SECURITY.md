# Security

## Supported versions

The latest tagged release. There are no long-lived release branches, so a fix ships in the next tag
rather than being backported.

## Reporting a vulnerability

Use GitHub's private vulnerability reporting on this repository: <https://github.com/Rethunk-Tech/go-larian/security/advisories/new>, or the **Security** tab, then **Report a vulnerability**. The report stays private between you and us until a fix ships.

If private reporting is unavailable, email [oss@rethunk.tech](mailto:oss@rethunk.tech) with **[SECURITY]** in the subject.

Please do not open a public issue, pull request or discussion for a suspected vulnerability.

A useful report has the affected repository and version (or commit), steps to reproduce, what an attacker gets and under which conditions, and whether anyone else knows.

## What counts as a vulnerability

A vulnerability is a flaw that lets someone cross a boundary they should not: run code, read or change data they have no right to, escape a sandbox or path restriction, or leak a credential. A bug is anything else that misbehaves, such as a crash on your own input, wrong output or a slow path; open an ordinary issue for those. If you are unsure which one you have, report it privately and we will say.

## Coordinated disclosure

We handle the report inside the private advisory, so you see each status change and can comment on the fix. The advisory goes public when the fix ships, or 90 days after your report, whichever comes first. If a fix is close, either of us can ask for more time in the thread. You are credited in the advisory unless you would rather not be.

## Response timelines

- We acknowledge your report within 3 business days.
- We give you an assessment (accepted, needs more information or declined, with reasons) within 10 business days.
- We aim to ship a fix within 90 days of the report.

## What is in scope

- A `.pak` passed to `Open`, `OpenFile` and `Pak.Read`: header offsets, file-list size, entry count, per-entry offset, sizes and compressed streams are untrusted. A crafted package must not panic, read outside the package, or allocate beyond `MaxEntries` and `MaxEntrySize`.
- Stored file names: a name that is absolute, empty-segmented, contains `.` or `..`, or has a segment Windows would resolve elsewhere (a `:`, a trailing dot or space, a device name such as `CON`) must never reach a caller, so `Entry.Path` is safe to join under a directory.
- `.lsx` documents passed to `ParseMeta` and `ParseModSettings`: size-capped at 8 MiB; no external entities are resolved.

## What is out of scope

- Joining `Entry.Path` to a destination and writing it safely (symlinks in the destination, total bytes written) is the caller's job; this package never writes files.
- Total decompressed bytes across many entries are not capped; a caller extracting a whole package sets its own budget.
- Mod UUIDs, names and MD5 values are reported as stored and not verified.

## Safe harbour

We will not pursue legal action against good-faith research that avoids privacy violations, data destruction and disruption of services, stops at a proof of concept, and keeps the details private until the advisory is published. Rethunk.Tech, LLC maintains this repository.

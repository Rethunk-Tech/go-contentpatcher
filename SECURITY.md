# Security

## Supported versions

The latest tagged release. There are no long-lived release branches, so a fix ships in the next tag
rather than being backported.

## Reporting a vulnerability

Use GitHub's **private vulnerability reporting** on this repository (Security → Report a
vulnerability). It stays private until a fix ships.

Please do not open a public issue for a suspected vulnerability first.

## What is in scope

A content pack is untrusted input. It is a folder of third-party files — `manifest.json`, `content.json` and
`config.json`, PNG images, `.tbin` maps, and `.tmx`/`.tmj` maps with `FromFile` references — and every one of them is
read by `Analyze` or by the readers it calls (`ReadPack`, `ReadPackForCleanup`, `ReadPackConfig`, `ModMapFiles`,
`TmxImageSources`). A crafted pack must not break, in any of them:

- **No path escape.** A `FromFile`, `LogName`, include, or map tilesheet reference that resolves outside the pack
  folder is refused or ignored; nothing outside the pack is opened, written, or read for reporting.
- **No unbounded read.** Every file read from a pack goes through a capped reader bounded by `maxPackFileBytes`, and
  no declared size in a file is trusted to allocate before the bytes are there. A `FromFile` pointing at a huge or
  endless file costs a bounded read, not the process.
- **No panic on malformed input.** Truncated JSON, a PNG whose declared dimensions disagree with its data, a `.tbin`
  whose offsets run past the end, a `.tmx` with a missing or cyclic reference, and a deeply nested condition are all
  findings or misses, never a panic, an infinite loop, or a crash of the caller.
- **No unbounded work from a small file.** Pixel-overlap scans, map scans and the alpha masks are bounded by the
  caps and by the shapes the file actually declares.
- **No silent trust of cached bytes.** A cache file is looked up by key but its content is re-checked against the
  file's size and modification time, so a stale or corrupt cache entry costs a re-read and cannot inject a result.

## What is out of scope

- The caller's own model of the profile. `Request.Enabled` and `Request.All` are trusted: the caller vouches that
  `Mod.Folder` is where that mod really is, and a caller that points one at an arbitrary directory has handed the
  package that directory's contents.
- The disk cache folder. `CacheRoot` is the application's own data folder; a caller that points it at a location
  another untrusted party can write is trusting that party.
- A pack that is merely dishonest to the player — declares a change it does not make, or claims an author it is not.
  Findings report what the files say.
- Denial of service by installing a very large profile of individually valid packs. Reading many packs is the job;
  the memory bound is `packGrace` and the caches, and a caller controls how many packs are in a request.

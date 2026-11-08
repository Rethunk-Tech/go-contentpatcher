# AGENTS.md

`contentpatcher` reads Content Patcher content packs — the JSON changes each pack declares, the images and maps they
touch, and the conditions and settings that decide when a patch applies — and reports what that means for one
profile: which packs collide, which pack wins each collision, and which packs nothing else needs. It is imported by
applications that present those findings to a player. It imports no application package: its dependencies are the
standard library and the small `Rethunk-Tech` libraries named in `go.mod`, never a consumer's model of a profile.

## Gate

`gate` is the command. Each override in `.gate.toml` is there because a runner
cannot resolve that check from PATH.

## Shape

- **`Analyze` is the only entry point that reads a profile.** Everything else is either a helper `Analyze` calls or a
  reader for one pack, so a caller cannot assemble a second, subtly different pass. Reason: two code paths over the
  same packs drift, and the cache keys are computed inside `Analyze`.
- **Package-level state is process-wide and guarded by `SetCacheRoot`.** The caches are package variables, not a
  struct a caller holds. `Reset` empties every one of them and is the only sanctioned way to get a clean slate; tests
  call it between fixtures. Reason: a caller that kept its own copy of the caches would keep the packs it read alive,
  and the memory bound is the point of the on-disk caches.
- **Packs are keyed relative to `CacheRoot`, never by absolute path.** A moved data folder keeps its cache entries;
  an absolute key would make every entry stale after a move. Reason: the app's data folder moves between machines and
  OS upgrade paths.
- **`SkipImageOverlap` is part of every cache key that can be affected by it.** Reason: the flag changes which image
  edits collide, so a result computed under one value must never be served under the other.
- **`FreeOSMemory` is a policy switch, not behaviour.** It defaults on because a released profile leaves ~100 MB of
  unreachable pages the scavenger is slow to return; a host that must not pay the pause sets it false. `ReleasePacks`
  checks it once, so a caller that flips it while a check runs gets one of the two behaviours, not a mix.
- **Every read from a pack folder goes through `safefs`.** Paths are resolved against the pack root before opening,
  file reads are capped at `maxPackFileBytes`, and a path that leaves the pack is a miss, not an error. Reason: a pack
  is third-party content; the checks are the only thing standing between it and the rest of the disk.
- **`maxPackFileBytes` and `packGrace` are policy, not tuning knobs.** They bound one file and one idle window; a
  caller changes them by editing the constant and saying why in the commit, not by a request field.
- **Unexported state stays unexported.** `packDiskState`, `parts`, `mapScans`, `footprintMemo` and the caches are
  package-private because a caller that reached into them could observe a half-updated profile. Exported surface is
  what a caller needs to describe a profile, not what the implementation happens to hold.
- **Findings carry no user-facing text.** `Findings`, `Conflict` and `Hint` describe what was found; wording belongs
  to the caller. A new field that holds a sentence breaks that split.

## Files

| File | Holds |
| --- | --- |
| `alphacache.go` | On-disk alpha masks of overlay images, keyed by path, size and mtime |
| `analyze.go` | `Analyze`, `Forget`, and the `HasMod` mention pass |
| `assetmap.go` | The asset index: `AssetTarget`, `AssetTouch`, paginated `AssetMapOf` and `WhoChangesOf` |
| `config.go` | `config.json` reading, schema toggles, and the config-condition evaluator |
| `conflict_evidence.go` | `Evidence` per clashing patch, crop bounds, and `CropPackImage` |
| `contentpatcher.go` | Pack parsing: changes, conditions, `LoadFile`, the pack disk cache |
| `doc.go` | Package documentation |
| `footprint.go` | The area each edit writes, the pixel-overlap scan, and tilesheet shapes |
| `mapscans_disk.go` | The disk cache of `.tmx`/`.tbin` scans and `FlushMapScans` |
| `memo.go` | The analysis memo, keyed by the stamps of every file the check read |
| `parts.go` | The per-target and per-pack part cache, so an unchanged target is not re-decided |
| `release.go` | `HoldPacks`, `ReleasePacks`, the grace window and `FreeOSMemory` |
| `session.go` | `SetCacheRoot` and `Reset` |
| `shadowed.go` | Which packs are overwritten everywhere they write |
| `tilesheets.go` | Map file listing, tilesheet-only and supporting pack detection |
| `types.go` | `ModID`, `Mod`, `Request`, `Findings` and the finding types |
| `winner.go` | Deciding the winner of a clash from the load order |

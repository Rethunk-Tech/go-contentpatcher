# Changelog

Notable changes to `go-contentpatcher`. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-11-21

### Added

- `Pack.Problems` and `Problem` list the changes the pack refused.

### Fixed

- Map and overlay files are read with `safefs.ReadFileCapped`, so a FIFO at the path is refused instead of blocking.
- Pack JSON and map-scan reads are capped.
- `FromFile` and `Include` paths resolve case-insensitively, and referenced maps are stamped under their real case in the cache.
- `Extend` stays out of the analysis memo key.
- Include reads stay inside the pack through directory symlinks.
- Conditioned `Include`s are kept, and `HasMod` arrays are rejected.
- `Exclusive` ranks above every numeric priority.
- Refused changes are reported on the pack.
- Stacked `./` prefixes are stripped from tilesheet asset names.

## [0.1.0] - 2026-11-08

### Added

- `Analyze`: one pass over a profile's content packs, returning conflicts with per-patch evidence, load-clash hints,
  shadowed packs, bundled packs, supporting packs, mentioned mods and tilesheet users.
- Conflict winners decided by a chosen win, a dependency's load order, or the pack Content Patcher ranks highest,
  per clash or per entry, with cosmetic overlaps marked and not counted.
- Pack reading: `content.json` changes and conditions, `config.json` settings against the config schema, images,
  `.tmx`/`.tbin`/`.tmj` maps, and `.png` tilesheets, with every path confined to its pack and every read capped.
- Pixel-accurate image overlap by default, and `Request.SkipImageOverlap` to trade it for speed.
- `BuildAssetIndex`, `AssetMapOf` and `WhoChangesOf`: a sorted, paginated index of every touched asset and who writes
  it.
- Two cache levels — an analysis memo keyed by the stamps of every file read, and a part cache keyed by the packs that
  can change one target or one pack's settings — all keyed relative to `CacheRoot`.
- `SetCacheRoot`, `Reset`, `Forget`, `HoldPacks` and `ReleasePacks`: process-wide caches, an explicit clean slate, and
  packs released a grace period after the last check with `FreeOSMemory` as a policy switch.

[Unreleased]: https://github.com/Rethunk-Tech/go-contentpatcher/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/Rethunk-Tech/go-contentpatcher/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Rethunk-Tech/go-contentpatcher/releases/tag/v0.1.0

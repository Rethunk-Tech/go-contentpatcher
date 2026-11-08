# Changelog

Notable changes to `go-contentpatcher`. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/Rethunk-Tech/go-contentpatcher/commits/main

# HUMANS.md

Run and use `go-contentpatcher`. Internals: [AGENTS.md](AGENTS.md).

## Quick start

```bash
go get github.com/Rethunk-Tech/go-contentpatcher
```

```go
package main

import (
 "fmt"
 "os"
 "path/filepath"

 "github.com/Rethunk-Tech/go-contentpatcher"
)

func main() {
 profile := filepath.Join(os.Getenv("HOME"), "profile") // one folder per installed mod

 mods := []contentpatcher.Mod{{
  Key:            "compat",
  Name:           "Compatibility Pack",
  UniqueID:       contentpatcher.SMAPI("Pack.Compat"),
  Folder:         filepath.Join(profile, "compat"),
  Enabled:        true,
  ContentPackFor: contentpatcher.SMAPI("Pathoschild.ContentPatcher"),
 }}

 found := contentpatcher.Analyze(contentpatcher.Request{
  Enabled:   mods,
  All:       mods,
  CacheRoot: filepath.Join(os.TempDir(), "contentpatcher-cache"),
 })

 for _, c := range found.Conflicts {
  fmt.Printf("%s %s: winner %v (%s)\n", c.Kind, c.Target, c.PackIDs, c.WinnerKind)
 }
 for _, s := range found.Shadowed {
  fmt.Printf("%s is overwritten everywhere by %d pack(s)\n", s.Name, len(s.By))
 }
}
```

`Findings` is the whole result: `Conflicts` (with `Evidence`, `Fixes` and the winner), `Hints`,
`Shadowed`, `Bundled`, `Supporting`, `Mentioned`, `Tilesheets` and `TilesheetsUnreadable`. Word each
one yourself; the package never produces user-facing text.

## Contracts

| Entry point | What a caller can rely on |
| --- | --- |
| `Analyze(req Request) Findings` | One pass over every pack. Returns zero-value `Findings` only if there is nothing to report; never panics on a malformed pack, a missing file, or a path that leaves the pack folder. A repeated call with unchanged files returns the memoised result. |
| `Request.Enabled` / `Request.All` | `Enabled` is what is switched on and what conflicts are drawn from; `All` adds the switched-off packs, so tilesheet users and supporting packs are found among them too. Neither list is modified. |
| `Request.Order` | The profile's load order as folded ids, or nil. With nil, load order decisions fall back to what the packs' dependencies say. It is called at most once per check. |
| `Request.SkipImageOverlap` | Skips the pixel-overlap scan of image edits: faster, and a pair of edits to one image is then reported whether or not their pixels meet. The flag is part of every cache key, so both modes' results coexist. |
| `Request.CacheRoot` / `SetCacheRoot(dir)` | Where the on-disk caches live, normally the app's data folder. Empty turns them off. Set it before the first read; packs are keyed relative to it, so moving the data folder keeps the entries. |
| `Request.Extend` | A caller's own pass over the same parsed packs, run inside the check so its reads share the check's stamps and part cache. Its hints merge ahead of the package's own. |
| `Reset()` / `Forget()` | `Reset` drops every in-memory cache and returns the process to its just-started state, for a caller that needs a clean slate. `Forget` drops only what survives between checks and keeps what a running check holds. |
| `FreeOSMemory` | Package variable, true by default. When true, released packs are followed by `debug.FreeOSMemory()`. A host that must not pay that pause sets it false. |
| `HoldPacks()` | Marks a span that reads packs, so a concurrent check keeps them in memory; the returned func ends the span. `Analyze` does this itself. |
| `BuildAssetIndex` + `AssetMapOf` / `WhoChangesOf` | Turns mods into a sorted, paginated view of every touched asset (`AssetTarget`), and answers who writes one target. Both are pure over the index they are given. |
| `ReadPack` / `ReadPackForCleanup` | One pack's parsed changes, from memory or disk, whether or not it is enabled. Unreadable is an empty pack, not an error. |
| `ReadPackConfig(folder)` | A pack's `config.json` as lower-case field values; a missing or unreadable file is an empty map, so every field keeps its schema default. |
| `ModMapFiles`, `IsTilesheetPack`, `IsSupportingPack`, `TilesheetPacks`, `SupportingPackKeys` | What a pack touches without a full check: its map files, whether it only loads maps and tilesheets, whether it only supplies assets. Incomplete folder reads are reported, never guessed at. |
| `CropPackImage` | Writes one crop of a pack image to the cache and returns its path, refusing any `fromFile` that resolves outside the pack. |
| `TmxImageSources`, `PrintableRuns` | Raw text scraped out of a map file, for a caller's own reporting. Bounded and safe on binary input. |

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

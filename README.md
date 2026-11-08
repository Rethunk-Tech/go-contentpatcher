<h1 align="center">go-contentpatcher</h1>

<div align="center">

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.27.3+-00ADD8?logo=go&logoColor=white)](go.mod)

</div>

---

Read Content Patcher content packs: patches, the assets they edit, and which pack wins each asset. Package `contentpatcher`, module `github.com/Rethunk-Tech/go-contentpatcher`.

A caller hands in the profile it already loaded — the enabled mods, the full list, an optional load order and a cache folder — and gets back findings about the packs on disk: which packs edit the same asset or data key, who wins each clash, which packs are entirely overwritten, which only supply assets to others, and which patches the current settings switch off. The package reads files and reports; it words nothing and knows nothing about the application that called it.

## Quick start

```bash
go get github.com/Rethunk-Tech/go-contentpatcher
```

A worked example and the contracts: [HUMANS.md](HUMANS.md).

## Features

- One check of a profile: `Analyze` returns conflicts with per-patch evidence, suggested settings, shadowed packs, bundled packs, supporting packs and tilesheet users.
- Wins decided by the profile's own rules: a chosen win, a dependency's load order, or the pack Content Patcher ranks highest, per clash or per entry.
- Reads only what a pack says it does — `content.json`, `config.json`, images, `.tmx`/`.tbin` maps and `.png` tilesheets — with every file capped at `maxPackFileBytes` and every path kept inside the pack.
- Caches at two levels: an analysis memo keyed by the stamps of every file it read, and per-target/per-pack parts keyed by the packs that can change them, so an edit re-checks one target instead of the profile.
- Pixel-accurate image overlap, or `SkipImageOverlap` to trade it for speed.
- Process-wide and safe for concurrent checks: packs leave memory a grace period after the last check, and `Reset` drops everything for a caller that needs a clean slate.

## Documentation

| Doc | Audience |
| --- | --- |
| [HUMANS.md](HUMANS.md) | Running and using the package |
| [AGENTS.md](AGENTS.md) | Internals, invariants, and why |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Process — commits, tests, docs |
| [SECURITY.md](SECURITY.md) | Vulnerability disclosure |
| [CHANGELOG.md](CHANGELOG.md) | Release notes |

## License

MIT — see [LICENSE](LICENSE). © 2026 Rethunk.Tech, LLC.

# Status

Current shipped state. Product principles live in `CLAUDE.md`; historical
release notes live in `CHANGELOG.md`.

Last updated: 2026-05-05.

## Current Release

Latest release: `v0.1.19` (2026-05-04).

- The TUI has a `Hot` discovery surface backed by `cliff.sh/hot.json`.
  Hot appears only after enough apps have non-zero scores; until then the
  `New` row remains visible.
- Sort order is descending-only: `stars ↓`, `recency ↓`, and `hot ↓` once
  Hot is revealed.
- README and reel fetches route through `cliff.sh/r/*` tracking redirects,
  with direct upstream fallback when the redirector fails and no cache exists.
- Registry seeding scripts now live in `cliff-registry`; this repo owns the
  client, installer, Worker, and embedded catalog snapshot.

## Live

- **`cliff.sh`** serves `scripts/install.sh` to curl and a small HTML page to
  browsers through the Cloudflare Worker in `web/worker`.
- **`registry.cliff.sh/index.json`** is published by `cliff-registry` CI and
  is the canonical catalog source.
- **Catalog** has 149 live apps across 15 categories. The embedded snapshot
  at `internal/catalog/data/index.json` is refreshed to the 2026-05-04
  registry build and remains the offline fallback.
- **GitHub releases** publish darwin/linux binaries for amd64 and arm64.
- **Install paths** are live through `curl cliff.sh | sh`,
  `go install github.com/jmcntsh/cliff/cmd/cliff@latest`, and
  `brew install jmcntsh/tap/cliff`.
- **Tracking redirects** at `cliff.sh/r/readme/<owner>/<repo>` and
  `cliff.sh/r/reel/<slug>` log Cloudflare Analytics Engine events and 302 to
  upstream content.
- **Hot aggregation** writes `hot.json` to the private `cliff-stats` R2 bucket
  after the minimum data gate is met.

## Pending

- **Weekly digest** remains the main unfinished curation surface.
- **Registry dispatch token** still needs to be wired so `cliff-registry` can
  trigger the embedded snapshot refresh workflow on merge.
- **Per-app view surfacing** is still gated on enough collected data and a
  clearer maintainer/user need.

## Known Issues

- The `brews:` GoReleaser block emits a deprecation warning. It stays for now
  because formulas ship precompiled Go binaries without the notarization or
  quarantine concerns of casks.
- Installed state is derived from `$PATH` and known manager bin dirs at
  runtime. This is intentional; cliff does not persist an installed-app list.
- `~/.cliff/cache/binmap.json` is a cache of learned repo-to-binary overrides,
  not durable state. Deleting it is safe.

## Updating This File

Update this file when shipped state, pending work, or known issues change.
Put version-by-version history in `CHANGELOG.md`.

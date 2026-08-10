# Status

Current shipped state. Product principles live in `CLAUDE.md`; historical
release notes live in `CHANGELOG.md`.

Last updated: 2026-08-10.

## Current Release

Latest release: `v0.3.0` (2026-08-10).

- **Hot rankings:** the `Hot` sidebar surface ranks apps by static 7-day or
  30-day net GitHub star growth from daily registry snapshots. It does not
  use Cliff views, accounts, or user telemetry.
- **Discovery refresh:** apps added in the same weekly scrape batch are
  ranked by stars in the capped New view, so it surfaces the highest-interest
  recent tools instead of selecting alphabetically.
- **Respec:** cliff is now scoped to a
  GitHub CLI/TUI scraper + browser + installer. Removed from the client:
  the submit flow (`cliff submit`, `+` keybind, huh form), reel
  playback, the old view-tracking Hot surface and `hot.json` fetch,
  the hand-picked Featured row, and README tracking redirects
  (READMEs now fetch directly from the GitHub API). The Worker is
  reduced to serving the install script and landing page — Analytics Engine
  logging, R2 stats, and the daily aggregation crons are gone. Standard sorts are
  `stars ↓` and `recency ↓`; sidebar rows are All / New / Hot /
  Installed / categories.
- README screenshots auto-load inline at the top of the detail view when the
  terminal supports Kitty, iTerm, or Sixel graphics; halfblocks is the
  fallback renderer. `CLIFF_IMAGE_PROTOCOL` overrides detection.
- Cargo installs can bootstrap Rust/Cargo first and then continue the original
  app install from the same TUI flow.
- `curl cliff.sh | sh` replaces an existing `cliff` binary already on `PATH`.

## Live

- **`cliff.sh`** serves `scripts/install.sh` to curl and a small HTML page to
  browsers through the Cloudflare Worker in `web/worker`. The trimmed
  Worker was redeployed 2026-07-07; `/r/*` and `/hot.json` now 404.
  Already-released clients handle this: they fall back to direct
  GitHub README fetches and treat a `hot.json` 404 as "no hot data."
- **`registry.cliff.sh/index.json`** is published by `cliff-registry` CI and
  is the canonical catalog source. The registry's weekly auto-seed
  workflow scrapes GitHub for new TUIs and CLIs and commits manifests to main.
- **Catalog** is fetched live on each launch from the weekly-refreshed
  registry. The release-time snapshot at
  `internal/catalog/data/index.json` is only the offline fallback.
- **GitHub releases** publish darwin/linux binaries for amd64 and arm64.
- **Install paths** are live through `curl cliff.sh | sh`,
  `go install github.com/jmcntsh/cliff/cmd/cliff@latest`, and
  `brew install jmcntsh/tap/cliff`.

## Pending

- **Registry dispatch token** still needs to be wired so `cliff-registry` can
  trigger the embedded snapshot refresh workflow on merge.

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

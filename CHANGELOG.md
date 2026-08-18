# Changelog

Version history for shipped cliff releases. Keep this concise; use git history
and PRs for implementation detail.

## Unreleased

- Removed the `New` sidebar view. `Hot` remains the single discovery
  surface so browsing is based on measured GitHub star growth.

## v0.3.0 - 2026-08-10

- Added a `Hot` view ranked by net GitHub star growth, with `t` switching
  between 7-day and 30-day windows. Rankings come from static daily registry
  snapshots and do not use Cliff views or user telemetry.
- Hot cards show the measured period change and label partial collection
  windows while the registry builds its initial history.

## v0.2.1 - 2026-08-03

- Ranked apps from the same weekly scrape batch by stars in the capped
  `New` view, so recent high-interest tools are shown instead of the
  first ten repositories alphabetically.
- Updated product copy to describe the live CLI and TUI catalog without
  a hard-coded app count.

## v0.2.0 - 2026-07-07

- **Respec: cliff is a GitHub TUI scraper + browser + installer.**
  The registry's weekly scraper is the sole catalog intake; everything
  built for the curated/submission model was removed. Rationale: the
  submission lifecycle, reels, and view-tracking hotness all assumed
  human-curated intake and per-app manual artifacts, which don't scale
  with an automated catalog and doubled the maintenance surface.
- Removed the submit flow: `cliff submit`, the `+` keybind, the huh
  form, and `internal/submit`.
- Removed reel playback (`internal/reelfetch`, the reel strip, the
  embedded cliff demo reel). Screenshots parsed from READMEs remain
  the visual preview.
- Removed the Hot surface, `hot.json` fetching (`internal/hotfetch`),
  and the hand-picked Featured row. Sorts are `stars ↓` / `recency ↓`;
  sidebar rows are All / New / Installed / categories.
- READMEs fetch directly from the GitHub API; the `cliff.sh/r/*`
  tracking redirects are gone. The Worker now only serves the install
  script and landing page (no Analytics Engine, R2, or crons).

## v0.1.23 - 2026-05-20

- README screenshots render inline only on terminals with a graphics
  protocol (Kitty, iTerm, Sixel). macOS Terminal and other basic emulators
  skip the screenshot strip instead of showing blocky half-block previews.
- Improved screenshot rendering on capable terminals: pixel-sized Kitty
  output, protocol warm-up before the TUI starts, and
  `CLIFF_IMAGE_PROTOCOL` override.

## v0.1.22 - 2026-05-20

- Screenshot gallery now auto-loads inline at the top of the README view,
  stacked above reels and markdown like the reel preview strip.
- Removed the full-screen `g` gallery overlay; browse multiple screenshots
  with `[` and `]` from the README view.
- Improved screenshot URL extraction (HTML `<img>` tags, badge filtering,
  WebP support).

## v0.1.21 - 2026-05-20

- Added a README screenshot gallery (`g`) that autopopulates from manifest
  `screenshots` or non-badge images found in the fetched README.
- Gallery images render through terminal graphics protocols (Kitty, iTerm,
  Sixel) via `go-termimg`, with browser fallback on `o`.
- `install.sh` now replaces an existing `cliff` binary already on `PATH`.

## v0.1.20 - 2026-05-15

- Added a `Featured` sidebar panel for launch-friendly, eye-catching apps.
- Added Cargo bootstrap in the TUI: when an install needs Cargo, Cliff can
  install Rust/Cargo first and then continue the original app install.
- Removed inline README hero-image fetching/rendering. Reels are the visual
  preview surface; README rendering is markdown-only again.

## v0.1.19 - 2026-05-04

- Added the `Hot` sidebar/sort surface, backed by a daily recency-weighted
  aggregation over README/reel view events.
- Added `internal/hotfetch` with ETag caching and 404-tolerant fallback.
- Changed sort cycling to descending-only: stars, recency, and hot when
  available.
- Moved registry seeding scripts to `cliff-registry`.

## v0.1.18 - 2026-05-01

- Routed README and reel fetches through `cliff.sh/r/*` tracking redirects.
- Added Cloudflare Worker aggregation into private R2 daily stats files.
- Added direct-upstream fallback for redirector 404, 5xx, and network failures.
- Added registry-side reel ownership attestation workflow.
- Added the `has_reel` catalog field for future UI use.

## v0.1.17 - 2026-04-26

- Refreshed the TUI visual system with gradient titles, borders, footer hints,
  and a shared spinner.
- Replaced the submit overlay with an in-TUI `huh` form.
- Made stacked reels scroll with the README content.
- Bumped `reel` to fix loop flicker.
- Documented `CLIFF_THEME` and `CLIFF_BG`.

## v0.1.16 - 2026-04-26

- Moved reels into a right-side pane on wide terminals.
- Added inline README hero-image rendering.
- Added registry reel lint guardrails.
- Refreshed the embedded catalog snapshot.

## v0.1.15 - 2026-04-25

- Added registry-hosted demo reels for every catalog app.
- Added `internal/reelfetch` with ETag cache and offline fallback.
- Added reel playback plumbing in the README view.

## v0.1.14 - 2026-04-23

- Fixed the `New` surface to use registry `added_at` when present.
- Capped the launch-week `New` row so it did not show the entire catalog.
- Refreshed the embedded catalog snapshot.

## v0.1.13 - 2026-04-23

- Added the `New` sidebar row.
- Collapsed install, uninstall, and upgrade into one package-operation state
  machine.
- Added `cliff bin-audit` for learned binary-name overrides.
- Added the embedded snapshot refresh workflow.

## v0.1.12 - 2026-04-22

- Added the first in-TUI reel preview for cliff itself.
- Disabled binmap detection for brew installs to avoid transitive dependency
  binaries.
- Added cliff to its own embedded catalog snapshot.

## v0.1.11 - 2026-04-22

- Added the `+` submit flow in browse/readme modes.
- Added `cliff submit [name|repo]` with non-interactive `--print` behavior.
- Added empty-search submit prompting.

## v0.1.10 - 2026-04-22

- Added the installed-app manage picker.
- Added update and uninstall flows in the TUI.
- Added the `Installed` sidebar row.

## v0.1.9 - 2026-04-22

- Added post-install launch in a new terminal tab for supported terminals.
- Added clipboard fallback when tab launch is unsupported or undesired.
- Connected the same launch affordance after PATH-fix flow.

## v0.1.8 - 2026-04-22

- Expanded the catalog to 43 apps, with more games, visualizers, music tools,
  infra tools, and typing apps.
- Refreshed the embedded catalog snapshot.

## v0.1.7 - 2026-04-21

- Added in-TUI and CLI support for auto-fixing PATH after off-PATH installs.
- Added `cliff install --fix-path` and `--no-fix-path`.
- Added interactive PATH prompts for TTY CLI installs.

## v0.1.6 - 2026-04-21

- Added post-install PATH warnings for binaries installed into manager default
  directories outside `$PATH`.
- Broadened installed detection to include those manager default directories.

# Changelog

Version history for shipped cliff releases. Keep this concise; use git history
and PRs for implementation detail.

## Unreleased

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

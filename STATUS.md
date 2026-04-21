# Status

What's actually shipped right now. Updated on every ship. Source of
truth for "is X live?" — principles docs ([`CLAUDE.md`](CLAUDE.md))
describe intent, not state.

Last updated: 2026-04-21.

## Latest change

`v0.1.6` (2026-04-21): post-install PATH warning. When an install
lands a binary in a known manager dir that isn't on `$PATH`
(classic `go install` → `~/go/bin`, `cargo install` →
`~/.cargo/bin`), the TUI and CLI now show the exact
`export PATH=...` line to add to the user's shell rc. Installed
detection also broadened to those same dirs so the ✓ marker
doesn't disappear after a successful off-PATH install.

## Live

- **`cliff.sh`** — Cloudflare Worker serving `scripts/install.sh` to
  `curl`, HTML to browsers. Deployed 2026-04-19.
- **`registry.cliff.sh/index.json`** — GitHub Pages, HTTPS enforced.
  Built by CI in [`jmcntsh/cliff-registry`](https://github.com/jmcntsh/cliff-registry)
  on every merge to main. DNS + cert live since 2026-04-21.
- **Catalog: 28 apps**, indie-first with a flashy games section
  (rebels-in-the-sky, tetrigo, chess-tui, plastic NES emulator,
  etc.). 13 categories. Seeded 2026-04-21 in
  [cliff-registry#1](https://github.com/jmcntsh/cliff-registry/pull/1).
  Embedded snapshot in `internal/catalog/data/index.json` matches
  the live index.
- **GitHub releases** — latest `v0.1.6` (2026-04-21). Darwin and
  linux, amd64 and arm64, via goreleaser.
- **`curl cliff.sh | sh`** — end-to-end working; downloads the
  tagged release, verifies sha256, installs to `/usr/local/bin` or
  `~/.local/bin`.
- **`go install github.com/jmcntsh/cliff/cmd/cliff@latest`** —
  works.
- **`brew install jmcntsh/tap/cliff`** — Homebrew tap live at
  [`jmcntsh/homebrew-tap`](https://github.com/jmcntsh/homebrew-tap).
  Formula auto-updated by goreleaser on each tagged release via
  `HOMEBREW_TAP_GITHUB_TOKEN`. First publish: `v0.1.5`, 2026-04-21.

## Pending

- **Phase 2** — curation surfaces (hand-picked seed, "new this
  week", submit flow, weekly digest). Not started.

## Known issues / gotchas

- The `brews:` goreleaser block emits a deprecation warning at
  `goreleaser check` time. We're intentionally keeping it: Casks
  require Apple notarization or an `xattr` quarantine bypass, and
  Formulas ship pre-compiled Go binaries cleanly without either.
  Re-evaluate when goreleaser v3 removes `brews` for real.
- Installed-state is derived from `$PATH` at runtime, not persisted
  to `~/.cliff/installed.json`. This is intentional (survives
  external uninstalls, recognizes pre-cliff installs) but `CLAUDE.md`
  used to imply otherwise; that's been cleaned up.

## How to update this file

Every PR that changes user-visible state (new domain goes live,
release cut, pending item moved to live, known issue fixed) should
update the relevant section and bump `Last updated`. If the change
is code-only and invisible to users, skip it.

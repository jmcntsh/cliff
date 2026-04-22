# Status

What's actually shipped right now. Updated on every ship. Source of
truth for "is X live?" — principles docs ([`CLAUDE.md`](CLAUDE.md))
describe intent, not state.

Last updated: 2026-04-22.

## Latest change

`v0.1.8` (2026-04-22): catalog up to 43 apps. Added 14 flashy/indie
TUIs biased toward games, visualizers, and music — the things that
make someone screenshot their terminal. Games: balatro-tui,
setrixtui. Visualizers: cava (audio), weathr (animated weather),
gitlogue (cinematic git replay), syscgo-tui (matrix/fire/fireworks).
Music: spotify-player, kew, termusic, rmpc. Infra: bottom (btm),
impala (wifi), bluetui. Typing: toofan. Quality bar held: active
repo, explicit license, installable via brew/cargo/go (no
source-only builds, no API-key walls, no server setup). Shipped in
[cliff-registry#3](https://github.com/jmcntsh/cliff-registry/pull/3);
embedded snapshot refreshed to match.

`v0.1.7` (2026-04-21): auto-fix PATH from inside cliff. Builds on
v0.1.6's warning — instead of making the user quit cliff, open
their `~/.zshrc`, paste a line, and reopen the terminal, pressing
⏎ on the post-install warning screen now opens a confirm dialog
showing the exact file and exact line; a second ⏎ appends it
(idempotent, with a `# added by cliff` marker). CLI gets
`cliff install --fix-path` / `--no-fix-path` plus an interactive
`[y/N]` prompt when stdin is a TTY, with non-interactive pipelines
falling back to the hint so scripts stay deterministic. Supports
zsh + bash today; fish is detected and shown the hand-edit line
rather than getting bash syntax written to `config.fish`.

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
- **Catalog: 43 apps**, indie-first and games/visualizer-heavy.
  Highlights: rebels-in-the-sky, tetrigo, plastic (NES emulator),
  balatro-tui, setrixtui, cava, weathr, gitlogue, syscgo-tui,
  spotify-player, kew, bottom, impala. 14 categories. Seeded in
  [cliff-registry#1](https://github.com/jmcntsh/cliff-registry/pull/1);
  expanded in [#3](https://github.com/jmcntsh/cliff-registry/pull/3).
  Embedded snapshot in `internal/catalog/data/index.json` matches
  the live index.
- **GitHub releases** — latest `v0.1.8` (2026-04-22). Darwin and
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

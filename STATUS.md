# Status

What's actually shipped right now. Updated on every ship. Source of
truth for "is X live?" — principles docs ([`CLAUDE.md`](CLAUDE.md))
describe intent, not state.

Last updated: 2026-04-21.

## Live

- **`cliff.sh`** — Cloudflare Worker serving `scripts/install.sh` to
  `curl`, HTML to browsers. Deployed 2026-04-19.
- **`registry.cliff.sh/index.json`** — GitHub Pages, HTTPS enforced.
  Built by CI in [`jmcntsh/cliff-registry`](https://github.com/jmcntsh/cliff-registry)
  on every merge to main. DNS + cert live since 2026-04-21.
- **GitHub releases** — latest `v0.1.4` (2026-04-19). Darwin and
  linux, amd64 and arm64, via goreleaser.
- **`curl cliff.sh | sh`** — end-to-end working; downloads the
  tagged release, verifies sha256, installs to `/usr/local/bin` or
  `~/.local/bin`.
- **`go install github.com/jmcntsh/cliff/cmd/cliff@latest`** —
  works.

## Pending

- **Homebrew tap** — `jmcntsh/homebrew-tap` repo exists,
  `.goreleaser.yaml` has the `brews:` block wired, release workflow
  passes `HOMEBREW_TAP_GITHUB_TOKEN`. Waiting on: PAT creation +
  secret upload, then first tagged release exercises the path.
  After that: `brew install jmcntsh/tap/cliff`.
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

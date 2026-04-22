# Status

What's actually shipped right now. Updated on every ship. Source of
truth for "is X live?" — principles docs ([`CLAUDE.md`](CLAUDE.md))
describe intent, not state.

Last updated: 2026-04-22.

## Latest change

`v0.1.12` (2026-04-22): looping reel preview above the readme view,
plus two supporting fixes. First in-TUI use of
[`jmcntsh/reel`](https://github.com/jmcntsh/reel) — the terminal
session recorder whose whole point is safe, redraw-based playback
embeddable inside another TUI. Opening the readme for the cliff
entry in the catalog now plays a hand-recorded 80×24 tour of the
app above the rendered markdown, framed by a subtle rounded border
and looped indefinitely. Reel's own format/screen/player packages
are public API as of its latest push; cliff depends on the latter
and owns only a small bubbletea adapter (`internal/ui/reel_strip.go`)
that drives the player's `Advance(dt)` / `Screen()` pull API and a
lipgloss cell renderer that coalesces same-styled runs per row. The
.reel file is embedded in the binary rather than fetched — first
contact with the feature has no network dependency, and the hosting
/ manifest-field questions ("where do other apps' reels live, who
ships them") stay deferred until the in-TUI UX has proved out. Every
non-cliff readme renders identically to before (the strip reports
zero height when no embedded reel matches the app slug, and the
layout math treats zero strips as absent). Supporting fixes in the
same ship: (a) disabled binmap detection for brew-type installs,
which previously could latch onto a transitive dependency's binary
and cache the wrong "run this" name (classic case: `brew install
cava` learned `fftwf-wisdom` because fftw's binaries landed in the
same shared bin dir); the brew path now trusts the manifest, every
other manager's detection is unchanged. (b) Added cliff to its own
catalog so the readme-view reel has somewhere to attach — visible
today through the embedded snapshot, and pending on `registry.cliff.sh`
until the matching `apps/cliff.toml` lands in
[`jmcntsh/cliff-registry`](https://github.com/jmcntsh/cliff-registry).

`v0.1.11` (2026-04-22): submit flow. cliff now has a first-class way
for users to nominate an app for the catalog without leaving the
terminal first. New `+` keybind (also `=`) in browse and readme modes
opens a small confirm overlay showing the exact URL that's about to
open — a prefilled GitHub issue on
[`jmcntsh/cliff-registry`](https://github.com/jmcntsh/cliff-registry)
with a `submission` label — and a second ⏎ hands off to the user's
browser. The overlay's post-open phase surfaces the full URL as a
fallback if `browser.Open` errored, so a headless/weird-terminal
setup never leaves the user stuck. Empty search results now show
`+ submit this app to cliff` as a secondary hint — the moment a
query returns zero is exactly when a user is most likely to want the
button. New `cliff submit [<name>|<owner/repo>]` subcommand mirrors
the TUI flow from the CLI, with `--print` for non-interactive use
and auto-detect (if stdout isn't a TTY, just prints the URL rather
than spawning a GUI window out of a CI job). Auth, storage, and
triage are all GitHub's — cliff hosts no form, runs no backend, and
adds no account system. The registry-side issue template
(`notes/registry-new-app-template.yml`) drops into the registry repo
as `.github/ISSUE_TEMPLATE/new-app.yml` for structured intake (name
/ repo / description / install type / install string / notes);
without it, GitHub falls back to a blank issue with the title
prefilled and the flow still works end-to-end.

`v0.1.10` (2026-04-22): manage picker, in-TUI update, and an
Installed sidebar row. Two changes that tighten the loop for apps
you already have. First: pressing ⏎ on an already-installed app
now opens a small horizontal picker —
`[ Update ]  Uninstall  Readme` — with Update default-selected
(leftmost, benign) and arrow keys moving the cursor. This replaces
the old "⏎ = readme regardless" behavior, which made the most
common next action on an installed tool (update it) cost four
keystrokes. Disabled actions dim themselves and are skipped by the
cursor so ⏎ never becomes a silent no-op. Direct keybind `U`
(shift-u) runs upgrade without the picker for muscle-memory users.
Script-type manifests without author-provided `[upgrade]` /
`[uninstall]` blocks get an honest "no recipe for this install
type" message rather than running the wrong command, and the
footer now leads with `U update · ⏎ manage` when the cursor is on
an installed app instead of the old `u uninstall`. Second: a new
"Installed" row pinned in the sidebar under "All" filters the grid
to just the apps cliff detects on your `$PATH` or in the known
manager bin dirs — your personal slice of the catalog, one
keypress away. Count stays live through install/uninstall. No disk
state: it's a runtime scan, same source of truth as the `✓`
markers, so it picks up pre-cliff installs and survives external
uninstalls.

`v0.1.9` (2026-04-22): open installed apps in a new tab. After a
successful install, pressing ⏎ now launches the just-installed app
in a new terminal tab while cliff keeps running in the original
one — no manual terminal-switching, no lost context. Works in
tmux, WezTerm, Kitty (with `allow_remote_control`), and iTerm2;
terminals with no safe tab-spawn API (Terminal.app, Alacritty,
Ghostty, vscode) fall back to copying the command to the
clipboard via `pbcopy`/`xclip`/`wl-copy` (native tool first,
OSC52 escape sequence as a last-resort fallback for SSH sessions
and unusual setups). The copy-to-clipboard path is now honest:
if the copy actually fails it says so and shows the command,
rather than flashing "copied" over an empty clipboard. Same
launch affordance also lands at the end of the fix-PATH flow,
so an off-PATH install → ⏎ fix PATH → ⏎ open in new tab chains
into a single gesture — and because the new tab's shell sources
the freshly edited rc, the app runs without the user thinking
about `source` or reopening anything.

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
- **Catalog: 44 apps**, indie-first and games/visualizer-heavy.
  Highlights: rebels-in-the-sky, tetrigo, plastic (NES emulator),
  balatro-tui, setrixtui, cava, weathr, gitlogue, syscgo-tui,
  spotify-player, kew, bottom, impala, and now cliff itself
  (reel-previewed). 14 categories. Seeded in
  [cliff-registry#1](https://github.com/jmcntsh/cliff-registry/pull/1);
  expanded in [#3](https://github.com/jmcntsh/cliff-registry/pull/3).
  Embedded snapshot in `internal/catalog/data/index.json` matches
  the live index except for the cliff entry, which lands on
  registry.cliff.sh as soon as the `apps/cliff.toml` PR merges.
- **GitHub releases** — latest `v0.1.12` (2026-04-22). Darwin and
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

- **Phase 2** — curation surfaces. Submit flow landed in v0.1.11
  (TUI `+` keybind and `cliff submit`); registry-side issue
  template open in [cliff-registry#4](https://github.com/jmcntsh/cliff-registry/pull/4).
  Still pending: a "new this week" surface in the TUI, and the
  weekly digest.

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
- `~/.cliff/cache/binmap.json` is a *cache*, not state: it remembers
  repo→binary-name overrides learned by scraping installer output
  (e.g. `cargo install minesweep` actually produces `minesweep`,
  not `minesweep-rs` as the repo basename would suggest). Deleting
  it is safe — installed-state detection still works, we just lose
  the right-name hint until the next install. `~/.cliff/logs/
  bin-audit.log` is an append-only record of every detected ≠
  derived event, for back-filling `binary` fields into the registry.

## How to update this file

Every PR that changes user-visible state (new domain goes live,
release cut, pending item moved to live, known issue fixed) should
update the relevant section and bump `Last updated`. If the change
is code-only and invisible to users, skip it.

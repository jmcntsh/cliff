# Status

What's actually shipped right now. Updated on every ship. Source of
truth for "is X live?" — principles docs ([`CLAUDE.md`](CLAUDE.md))
describe intent, not state.

Last updated: 2026-04-26.

## Latest change

`v0.1.17` (2026-04-26): Charm-flavored visual pass on the TUI, an
in-TUI submit form, two reel-strip fixes, and `CLIFF_THEME` /
`CLIFF_BG` documentation.

(1) Visual pass (`internal/ui/theme/styles.go`, `view.go`, `card.go`,
`overlay_*.go`, `sidebar.go`, `update.go`, `root.go`). Brand mark
ignites on launch via a torch-sweep gradient that self-terminates
after ~1.2s. Card / modal / search-bar / help borders use a 4-edge
fuchsia→indigo gradient routed through a precomputed HCL midpoint
so the corners flow rather than meeting abruptly. Selected card
name, sidebar header, focused sidebar row, and every modal header
render with `theme.GradientTitle` for consistent brand-pop on
emphasis. Footer hints parse per-keycap so action letters glow and
descriptions sit muted. README rendering switches to Glamour's
"pink" style on dark backgrounds. A single
`bubbles/spinner` instance ticks across every loading surface
(install startup, README fetch, reel fetch) so glyphs rotate in
lockstep, on a tick chain that self-arms only while something is
actually loading. Bug fixes: sidebar category change now jumps the
grid cursor to row 0 instead of leaving it on the last card of the
new list; `gridDimensions` reserves 4 chrome rows (title + blank +
footer newline + footer) instead of 2, so the title and sidebar
header no longer scroll off-screen on narrow heights.

(2) `+` opens an in-TUI submit form built on
[`charmbracelet/huh`](https://github.com/charmbracelet/huh)
(`internal/ui/overlay_submit.go` + `internal/ui/theme/huh.go`)
instead of bouncing straight to the confirm preview. Validation
runs on field exit: slug-only name, owner/name repo shape (auto-
stripping pasted `github.com/` prefixes), 120-char description cap.
Form completes → existing confirm preview → existing browser
hand-off; `submit.Request` stays the source of truth so the CLI
`cliff submit` verb is unchanged. `theme.HuhTheme()` maps cliff's
adaptive palette into huh's theme struct so the form reads as
native cliff and honors `CLIFF_THEME` the same way the rest of the
UI does.

(3) Reel scroll-along (`internal/ui/readme.go`). In stacked mode
the reel strip now lives inside the README viewport's content
stream as a hero block above the markdown, instead of as a sibling
row pinned above it. Scrolling down lets the reel leave the top of
the panel naturally — same UX as a hero image at the top of a web
page. Right-pane mode (wide terminals, reel beside the readme) is
unchanged. The per-tick refresh splices a cached glamour render
with the live `reel.View()` and preserves `YOffset` across
`SetContent` so scroll position survives the rebuild.

(4) Reel flicker fix (via reel pin bumped to
[`jmcntsh/reel@15d1f06`](https://github.com/jmcntsh/reel/commit/15d1f068f7c1)).
Looping playback used to rebuild a blank screen on every wrap from
the last frame back to frame 0 and re-apply frame 0's patch. For
the recorder's common shape — frame 0 emits no paint ops because
the source app's first stable state diffed cleanly against the
empty starting grid, frame 1 carries the entire UI — that produced
a ~9Hz blank/painted strobe. `lazygit.reel` and similar short
reels in `cliff-registry` were unwatchable. The player now skips
the blank-out on wrap when frame 0 paints zero cells (no ops, no
clear, no clear_line — cursor moves and hides still allowed since
they don't disturb cell contents). Affects every embedded reel,
not just lazygit's.

(5) `cliff help` gains an Environment section
(`cmd/cliff/commands.go`) listing `CLIFF_REGISTRY_URL`,
`GITHUB_TOKEN`, `CLIFF_THEME`, and `CLIFF_BG`. README adds a
one-line hint for users hitting washed-out colors. `DEVELOPMENT.md`
gains a "Forcing the theme" subsection explaining when
`CLIFF_THEME` (whole UI) vs `CLIFF_BG` (Glamour-rendered README
only) is the right knob.

`v0.1.16` (2026-04-26): two follow-ups on top of v0.1.15 — the
right-side reel pane and inline README hero images — plus the registry
guardrail that locks in the reel-publishing pipeline.

(1) Reels move into a right-side pane on wide terminals
(`internal/ui/reel_strip.go` + `readme.go` layout pass). Below the
breakpoint they still stack above the README; above it the reel sits
next to the prose so the playback doesn't push the README content
out of view as the user reads. (2) New `internal/ui/hero_image.go`
+ `hero_pick.go` + `hero_render.go` fetches the first non-badge image
from each app's README, half-block-renders it into ANSI, and splices
it back in at the original placement via a `CLIFFHEROANCHORZ7`
placeholder so glamour's URL wrapping doesn't fight the substitution.
Limits: PNG/JPEG/GIF first frame, 60×15 cells, 5s timeout, 5 MiB.
`raw.githubusercontent.com` URLs get pinned to `HEAD` so manifests
with stale branch segments still resolve (workaround for cliff-registry#7).
(3) `cliff-registry` gained `scripts/lint-reels.py`, wired into the
registry workflow's lint job. It greps every committed `.reel` for
`/Users/`, `/var/folders/`, common recorder usernames, requires the
opening `[disclaimer: ...]` card on Template-1 demos, and rejects
orphan artifacts (a `.reel` with no `demo.sh` source, or vice versa).
Runs in ~50ms with no extra deps; fails the publish job on regression.

Embedded snapshot at `internal/catalog/data/index.json` refreshed in
this cut so first-launch / offline users see the live 44-app catalog
including the corrected `bluetui` README URL and current
star/last_commit values.

`v0.1.15` (2026-04-25): registry-hosted demo reels go live for every
app, plus the client side that fetches and plays them. New
`internal/reelfetch` package mirrors the `internal/readme` cache
shape (`~/.cache/cliff/reels/<slug>.{reel,etag}`, `If-None-Match` for
cheap revalidation, fall-through to cache on network errors). The
reel strip starts in a "not ready" state and folds in fetched bytes
via a `reelFetchedMsg` that root update routes unconditionally, so
reels arrive even with a modal open. Adaptive high-contrast colors
are dropped while a reel is recording so captures don't lock to one
terminal's palette.

The 44 reels themselves live in `cliff-registry/reels/<slug>.reel` and
publish to `https://registry.cliff.sh/reels/<slug>.reel` from the same
Pages job that emits `index.json`. 26 are real binary captures, 18 are
scripted fakes for apps where a real run would either need
unobtainable input (audio for `scope-tui`, BT/Wi-Fi devices for
`bluetui`/`impala`) or leak host metadata (`btop`, `superfile`'s
owner/group columns). All 18 scripted reels open with a centered
`[disclaimer: simulated preview; see README for exact behavior]` card
that `scripts/record-reel.sh` injects at record time so the policy is
enforced by the recorder rather than by hand.

`v0.1.14` (2026-04-23): fix-ups on top of v0.1.13. Two problems the
first cut of "New this week" had: (1) it was falling back to
`last_commit` because the registry wasn't stamping `added_at` yet,
so the row read as "projects that pushed code this week" — mostly
famous Charmbracelet repos and two of my own — rather than "new to
cliff." (2) Even once `added_at` was present, the AddedAt branch
had no cap, so on launch week — where every app was added in the
past 7 days — the row showed the whole catalog (44/44) and conveyed
nothing.

Fix (1) shipped in [cliff-registry#6](https://github.com/jmcntsh/cliff-registry/pull/6):
`cmd/build` now runs `git log --diff-filter=A --follow --format=%aI`
against each manifest to find the commit that first added it, and
writes that as RFC3339 `added_at` on every app. CI publish job
switched to `fetch-depth: 0` so `git log` can see history older
than the clone's default depth. Live `registry.cliff.sh/index.json`
now has `added_at` populated on all 44 apps, and the embedded
snapshot was refreshed in the same cut (first real run of the
auto-PR workflow — pushed the branch fine, repo setting needed a
flip to let GHA open PRs, now enabled for next time). Fix (2)
collapsed `newSet`'s two branches into one ranked-window function
that caps both paths at 10: pick AddedAt when any app has it, else
LastCommit, take top `newCap` inside `newWindow`. Same steady-state
behavior, correct launch-week behavior, net fewer lines.

`v0.1.13` (2026-04-23): four internal/UX changes in one cut. (1) New
"New" row in the sidebar — between "All" and
"Installed" — showing recently-added apps. Until the registry starts
stamping an `added_at` field on each manifest (the correct signal),
the client falls back to `last_commit` within a 7-day window, capped
to the 10 freshest apps so the row reads as a curated surface rather
than "every active project in the catalog." Once the registry emits
`added_at`, the cap drops away automatically. (2) The install /
uninstall / upgrade mode trio collapsed into a single three-phase
state machine (`modePkgConfirm` / `Running` / `Result`) parameterized
by `pkgOp`; removes ~381 lines and one file. No user-visible
behavior change — the three ops still render different verbs and the
install-only launcher / PathWarning follow-ups still fire exactly
where they used to, just out of one view function instead of three
parallel ones. (3) New `cliff bin-audit` maintainer subcommand turns
`~/.cliff/logs/bin-audit.log` — the append-only record of
detected ≠ derived binary-name events — into paste-ready
`binary = "…"` overrides for PRs against the registry. Two output
formats: `summary` (human-scannable table) and `toml-patches`
(ready to paste per manifest). Closes the loop on the "how do we
backfill `binary` without hand-editing every manifest" question by
just reading what cliff has already been observing. (4) New
`.github/workflows/refresh-snapshot.yml` auto-PRs an update to the
embedded `internal/catalog/data/index.json` whenever the live
registry diverges. Three triggers: weekly cron (safety net), manual
button, and `repository_dispatch` from `cliff-registry`'s CI on
merge (see `notes/registry-dispatch.md` for the 5-line snippet the
registry side needs). The embedded fallback stops drifting by
default. `reel` is unchanged and stays.

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
- **GitHub releases** — latest `v0.1.17` (2026-04-26). Darwin and
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
  template merged in [cliff-registry#4](https://github.com/jmcntsh/cliff-registry/pull/4).
  "New this week" surface landed in v0.1.13, flipped to real
  add-order in v0.1.14; the weekly digest is still pending.
- **Registry dispatch token** — the auto-PR snapshot workflow has
  a `repository_dispatch` trigger, but `cliff-registry`'s CI needs
  `CLIFF_CLIENT_DISPATCH_TOKEN` configured and the 5-line notify
  step in `notes/registry-dispatch.md` added before the remote
  kick fires. Until then the weekly cron + manual button still
  keep the snapshot fresh.

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

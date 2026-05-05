# cliff — Development

The developer handbook for the cliff codebase: how to build, test,
and contribute. For the product vision, see [README.md](README.md);
for operating principles, see [CLAUDE.md](CLAUDE.md).

The TUI fetches `index.json` from a registry URL (overridable via
`CLIFF_REGISTRY_URL`) with an ETag-cached fallback, then falls
back further to a build-time snapshot of the same index embedded
in the binary if everything upstream fails. The `i` install action
shells out via the manifest's `[install]` block; installed apps
are detected by scanning `$PATH` and marked with `✓` in the list.

The registry itself lives in [`jmcntsh/cliff-registry`](https://github.com/jmcntsh/cliff-registry):
TOML manifests under `apps/`, lint + build commands, and a CI
workflow that publishes `index.json` to GitHub Pages at
`registry.cliff.sh` on every merge to main. Distribution
scaffolding (`.goreleaser.yaml`, `scripts/install.sh`) is in
place; releases auto-cut on `v*` tag push.

## Run it

```sh
go run ./cmd/cliff                       # iterate
go build -o cliff ./cmd/cliff && ./cliff # production-like local run
```

No flags required. Press `?` for keybinds, `q` / `ctrl-c` to quit.

### Pointing at a custom registry

By default the client fetches `https://registry.cliff.sh/index.json`.
For local registry dev, build an `index.json` against a checkout of
[`jmcntsh/cliff-registry`](https://github.com/jmcntsh/cliff-registry)
and point the client at it:

```sh
cd ../cliff-registry && go run ./cmd/build ./apps /tmp/index.json
CLIFF_REGISTRY_URL="file:///tmp/index.json" go run ./cmd/cliff
```

`CLIFF_DEBUG=1` prints the catalog source (`registry`, `cache`,
or `embedded`) and any non-fatal fetch error to stderr at startup.

### Forcing the theme

cliff picks light/dark variants based on the terminal's reported
background. Two overrides exist for terminals that lie or refuse
to answer:

- `CLIFF_THEME=dark|light` forces the entire UI palette (sidebar,
  cards, footer, modals) to one side, regardless of what the
  terminal reports.
- `CLIFF_BG=dark|light` forces only the README's Glamour renderer.
  Useful when OSC 11 doesn't round-trip (some SSH/tmux setups).

Set whichever matches your reality:

```sh
CLIFF_THEME=dark cliff       # one-off
echo 'export CLIFF_THEME=dark' >> ~/.zshrc   # persist
```

## Project layout

```
cmd/cliff/main.go                     — entry point (thin: load catalog, start tea program)
internal/catalog/                     — catalog types, embedded snapshot, fetch chain
  catalog.go, load.go                 — App/Category/Catalog types, //go:embed loader
  catalog_test.go                     — smoke test: embedded snapshot parses cleanly
  data/index.json                     — build-time snapshot of registry.cliff.sh/index.json
  fetch.go, fetch_test.go             — registry index fetcher (ETag-aware) with cache→embedded fallback
internal/ui/                          — all TUI code; Bubble Tea models
  root.go                             — Root struct, New, Init, state helpers (resize, refilter)
  update.go                           — Update + mode-specific update handlers + action helpers
  view.go                             — View + computeTitle + footer + emptyListView
  filter.go                           — pure filterAndSort pipeline (category/lang/stars/search/sort)
  filter_test.go                      — unit tests for filter/sort/search
  keys.go                             — central key.Binding map (single source of truth)
  delegate.go                         — bubbles/list row rendering (stars/name/lang/desc)
  layout.go                           — width breakpoints (wide/medium/narrow) and pane sizes
  sidebar.go, detail.go, readme.go    — secondary models/views
  overlay_filter.go, overlay_help.go  — modal overlays
  theme/styles.go                     — colors + reusable lipgloss styles
internal/readme/                      — runtime README fetcher + disk cache (ETag-aware)
  fetch.go                            — GET /repos/{owner}/{repo}/readme with If-None-Match
  cache.go                            — ~/.cache/cliff/readme/<owner>/<repo>.{md,etag}
internal/browser/open.go              — per-OS URL opener (darwin/linux/windows); invoked by `o` to open the selected app's homepage
internal/clipboard/osc52.go           — OSC 52 escape-sequence writer (no deps, works over SSH); invoked by `y` to copy the preferred install command (falls back to GitHub URL)
internal/install/                     — install runner + PATH-based detection
  install.go                          — Stream (sh -c, streamed output) + Detect ($PATH scan) + Diagnose
  install_test.go                     — runner + detection + diagnose unit tests
scripts/install.sh                    — installer for `curl cliff.sh | sh`
web/worker/                           — Cloudflare Worker behind cliff.sh
  src/index.js                        — serves install.sh to curl, HTML to browsers
  wrangler.toml                       — deploy config; routes commented until DNS lives on CF
  README.md                           — one-time setup + deploy instructions
.goreleaser.yaml                      — release config (cross-compile + checksums)
```

## Development

```sh
go build ./...         # build everything
go test ./...          # run all tests
go vet ./...           # static checks
```

### Working with the registry

Manifests live in [`jmcntsh/cliff-registry`](https://github.com/jmcntsh/cliff-registry)
under `apps/<name>.toml`. To add or edit one, open a PR there:

```sh
$EDITOR apps/myapp.toml
go run ./cmd/lint ./apps                       # validate
go run ./cmd/build ./apps /tmp/index.json      # preview the compiled index
```

CI lints every PR and, on merge to main, rebuilds `index.json`
and publishes it to `https://registry.cliff.sh/index.json` via
GitHub Pages. Schema: [`docs/manifest.md`](https://github.com/jmcntsh/cliff-registry/blob/main/docs/manifest.md)
in that repo.

### Refreshing the embedded offline fallback

The binary ships with a snapshot of the live index so it has content
to show on first launch and when offline. Refresh it with:

```sh
curl -fsSL https://registry.cliff.sh/index.json -o internal/catalog/data/index.json
```

Commit the diff. No scraper, no GitHub token — the registry repo's
CI already did the work.

### Where state lives at runtime

- `~/.cache/cliff/readme/<owner>/<repo>.md` + `.etag` — README cache.
- `~/.cliff/cache/index.json` + `.etag` — fetched registry index.

Installed-state is derived from `$PATH` at runtime, not persisted.
No config file, no telemetry.

## Common maintainer tasks

**Add a keybinding:** edit `internal/ui/keys.go`, then handle it in the relevant `update*` function in `internal/ui/update.go`. The help overlay and footer hints update automatically for everything that's already listed in `helpSections` (see `internal/ui/overlay_help.go`).

**Add a filter criterion:** add a field to `filterCriteria` in `internal/ui/filter.go`, extend the filter predicate, and update `filter_test.go`.

**Tweak colors or typography:** everything routable lives in `internal/ui/theme/styles.go`.

**Change layout breakpoints:** `internal/ui/layout.go` — `modeFor()` decides wide/medium/narrow from terminal width.

## Stack

Go + [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Bubbles](https://github.com/charmbracelet/bubbles) + [Lip Gloss](https://github.com/charmbracelet/lipgloss) + [Glamour](https://github.com/charmbracelet/glamour) (README rendering) + [sahilm/fuzzy](https://github.com/sahilm/fuzzy) (search).

Chosen for speed to MVP and single-binary distribution. See [CLAUDE.md](CLAUDE.md) for the full rationale.

## What's not in the tool today

cliff does not have accounts, hosted binaries, a server-side catalog
database, client telemetry, or sandboxed installs. The remaining
curation work is mostly editorial/product surface area, such as a
weekly digest and deciding how much of the collected view data to show.

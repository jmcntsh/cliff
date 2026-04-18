# cliff

Operating principles for working in this repo. Pitch lives in
[`README.md`](README.md); build/test instructions in
[`DEVELOPMENT.md`](DEVELOPMENT.md); manifest schema in
[`notes/manifest.md`](notes/manifest.md).

Strategy, roadmap, and active checklist live in a separate private
repo (`jmcntsh/cliff-notes`). If you're working on cliff with
access to that repo, open it alongside this one.

## What this repo is

cliff is a terminal-native directory for CLIs and TUIs. The Go
binary in `cmd/cliff` opens a TUI: list of apps, search, sort,
rendered README, one-key install via the project's own package
manager (brew / cargo / npm / pipx / `go install` / script).

The registry is TOML manifests under `registry/apps/`, compiled to
`index.json` by CI, served via GitHub Pages at
`https://registry.cliff.sh/index.json`. The client fetches that
URL (overridable via `CLIFF_REGISTRY_URL`) with an ETag-cached copy
in `~/.cliff/cache/` and an embedded scrape as last-resort fallback.

Distribution: `curl cliff.sh | sh`, brew tap, `go install`. Single
static binary.

## Operating principles

Three rules shape every decision. If a change violates one, either
the change is wrong or a principle is wrong — stop and escalate.

### 1. Lazy in planning, meticulous in execution

Pick the smallest scope and simplest mechanism that could work;
then do that one thing carefully and well. Concretely:

- **We host nothing we don't have to.** No binaries, no catalog
  database, no web app, no backend. The registry is TOML in a
  GitHub repo, compiled to `index.json` by CI, served via GitHub
  Pages. There is no server to run.
- **We wrap existing infrastructure, we don't replace it.**
  Installs shell out to brew/cargo/npm/pipx/go/script. Discovery
  leans on GitHub stars and curation, not a recommendation engine.
  Search is `sahilm/fuzzy` over a static list, not Elasticsearch.
- **Defer until proven needed.** Gallery view, ratings, image
  rendering, asciinema playback, `cliff try` sandbox — out of
  scope until a user pushes against the absence.

### 2. The directory's only job is to be used

Success looks like weekly-active terminal users who return to
cliff to find new tools. That is the single metric that matters.
Everything else — pretty UI, rich previews, submit flow, clever
search — is in service of it.

- **Zero friction to first install.** No account, no signup, no
  prompt, no config file. `curl cliff.sh | sh` → `cliff` → arrow
  keys → `i`. Anything that adds a step between "I just heard
  about cliff" and "I installed an app with cliff" is load-bearing
  in the wrong direction.
- **Curation is the product.** The front door is not the full
  600-app scraped list. It's a hand-picked seed refreshed weekly,
  plus a "new this week" surface. Famous FOSS is backdrop;
  indie/new work is the identity.
- **No dark patterns, ever.** No signup walls, no "sign in to see
  more," no modal nags.

### 3. Name the trust model; don't pretend to be the App Store

cliff is a directory and an installer dispatcher. It is not a
review team.

- **We don't sandbox or inspect code.** Installs run with the
  user's shell privileges, same as `brew install` or `cargo
  install` from a repo you don't personally audit. The install
  confirmation shows the exact command that will run.
- **`script`-type installs show an extra warning.** Piping curl
  to shell is normal in our ecosystem, but the warning copy is
  real, not a rubber stamp.
- **Delisting is the enforcement mechanism.** If a listed app is
  credibly reported as malicious, we delist fast — next
  `index.json` build, minutes not days. That's the remedy. We
  don't run a review queue.

### Corollary: don't make the user leave

The browser is a last resort. There are no forced browser hops.

## Working notes

- The bundled awesome-tuis scrape (~600 apps) is the last-resort
  fallback. The front page is a hand-picked seed; the scrape
  backfills a "Browse all" surface.
- UI target: feels like a modern app (comparable to Claude Code or
  gh-dash), not a basic list.
- `cliff.sh` is the priority domain — `curl cliff.sh | sh` is the
  single most important growth surface.

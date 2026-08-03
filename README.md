# cliff

**A terminal-based browser for TUIs and CLIs.** New terminal apps are scraped
from GitHub weekly; browse them, read the README without leaving the
terminal, install in one keystroke.

```
$ curl cliff.sh | sh
$ cliff
```

A live catalog refreshed by the weekly scraper, rendered READMEs with
inline screenshots, fuzzy search, one-key install via the project's own
package manager (brew / cargo / npm / pipx / `go install`). No
accounts, no telemetry, no hosted binaries.

If the colors look washed out, your terminal is reporting the
wrong background. Force it: `CLIFF_THEME=dark cliff` (or `light`).

## Why this exists

AI coding tools are producing a flood of weird, useful, personal
terminal apps faster than GitHub stars or awesome-lists can surface
them. cliff finds them automatically: a scheduled scraper searches
GitHub for new TUIs and CLIs each week, filters out libraries and
templates, and adds anything installable to the catalog. The client is
a fast in-terminal browser over that catalog.

The audience is people who live in the terminal and want an easy way
to check for new tools and install them. That's it.

## The product in one screen

- **Browse** the catalog in a real TUI. README rendered with Glamour
  inline, screenshots when your terminal supports graphics, metadata
  sidebar, fuzzy search, a "New" row for this week's finds.
- **Install** in one keystroke. cliff shells out to the right
  package manager (brew / cargo / npm / pipx / `go install` /
  upstream install script). We host zero binaries — we wrap
  existing infrastructure.
- **Stay in the terminal.** `o` opens the project page in your
  browser; `y` copies the install command via OSC 52 (works over
  SSH, no clipboard helper needed); `?` shows everything else.

That's the whole product. It is small on purpose.

## Non-goals (and why)

- **No hosted binaries.** Ever. Package managers already solved
  distribution; we wrap them.
- **No accounts or telemetry.** The directory works with zero
  friction and phones home to no one.
- **No submission pipeline.** The scraper is the intake. Anything it
  misses can be added with a one-file PR to the registry repo.
- **No sandbox as a security boundary.** Installs run with the
  user's shell privileges, same as `brew install`.

## Architecture, briefly

- **Client:** Go single static binary (Bubble Tea stack).
  Distributed via `curl cliff.sh | sh`, `brew`, `go install`.
- **Registry:** TOML manifests in the registry repo, written mostly
  by a weekly GitHub-scraper workflow, compiled to `index.json` by CI,
  served via GitHub Pages at `https://registry.cliff.sh/index.json`.
  No catalog database.
- **Distribution of apps:** wrap existing package managers. We
  host zero binaries and never will.
- **Backend:** a small Cloudflare Worker serves the `cliff.sh`
  install script and landing page. Nothing else runs server-side.

See [`CLAUDE.md`](CLAUDE.md) for operating principles, and
[`DEVELOPMENT.md`](DEVELOPMENT.md) for how to build and run.

## Manifest at a glance

```toml
name = "lazygit"
description = "Simple terminal UI for git commands"
author = "jesseduffield"
homepage = "https://github.com/jesseduffield/lazygit"
readme = "https://raw.githubusercontent.com/.../README.md"
tags = ["git", "tui"]
license = "MIT"

[install]
type = "brew"                    # brew | cargo | npm | pipx | go | script
package = "lazygit"
```

Use `[[installs]]` instead when an app supports multiple package
managers. Full schema in [`notes/manifest.md`](notes/manifest.md).

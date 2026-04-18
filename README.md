# cliff

**A terminal-native directory for CLIs and TUIs.** Browse what
people have built, read the README without leaving the terminal,
install in one keystroke.

```
$ curl cliff.sh | sh
$ cliff
```

Today: ~600 apps, rendered READMEs, fuzzy search, one-key install
via the project's own package manager (brew / cargo / npm / pipx /
`go install`). No accounts, no payments, no hosted binaries.

## Why this exists

AI coding tools are producing a flood of weird, useful, personal
terminal apps faster than GitHub stars or awesome-lists can surface
them. Most of it lives in a gist or a half-finished repo nobody
finds. cliff is the front door: a well-curated, fast, in-terminal
directory that treats new indie TUIs as first-class, not buried
under the same five famous FOSS projects.

The audience is people who live in the terminal and want a better
way to discover new tools for it. That's it. Everything else is
downstream of nailing that.

## The product in one screen

- **Browse** a curated catalog in a real TUI. README rendered with
  Glamour inline, metadata sidebar, fuzzy search.
- **Install** in one keystroke. cliff shells out to the right
  package manager (brew / cargo / npm / pipx / `go install` /
  upstream install script). We host zero binaries — we wrap
  existing infrastructure.
- **Discover** — the list is sorted by recency and curation, not
  just stars, so a good project from last week isn't drowned by
  btop. Weekly highlights, tags, and categories surface new work.
- **Stay in the terminal.** `o` opens the project page in your
  browser; `y` copies the install command via OSC 52 (works over
  SSH, no clipboard helper needed); `?` shows everything else.

That's the whole product. It is small on purpose.

## Non-goals (and why)

- **No hosted binaries.** Ever. Package managers already solved
  distribution; we wrap them.
- **No accounts for browsing or installing.** Auth is a wall; the
  directory works with zero friction.
- **No sandbox as a security boundary.** Installs run with the
  user's shell privileges, same as `brew install`. If we add a
  try-mode later, it's for trying, not safety.

## Architecture, briefly

- **Client:** Go single static binary (Bubble Tea stack).
  Distributed via `curl cliff.sh | sh`, `brew`, `go install`.
- **Registry:** TOML manifests in the registry repo, compiled to
  `index.json` by CI, served via GitHub Pages (planned canonical
  URL: `https://registry.cliff.sh/index.json`). No database.
- **Distribution of apps:** wrap existing package managers. We
  host zero binaries and never will.
- **Backend:** none.

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

Full schema in [`notes/manifest.md`](notes/manifest.md).

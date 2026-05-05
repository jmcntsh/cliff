# cliff — Manifest Schema v0

One TOML file per app, lives at `apps/<name>.toml` in
`jmcntsh/cliff-registry`. CI validates and compiles all manifests into
`index.json`.

Manifests describe an app and how to install it. They do not
describe pricing, licensing, or sellers — cliff is a directory,
not a marketplace.

## Full example

```toml
name = "lazygit"
description = "Simple terminal UI for git commands"
author = "jesseduffield"
homepage = "https://github.com/jesseduffield/lazygit"
readme = "https://raw.githubusercontent.com/jesseduffield/lazygit/master/README.md"
demo = "https://asciinema.org/a/410848"
screenshots = [
  "https://example.com/lazygit-1.png",
]
tags = ["git", "tui"]
license = "MIT"

[install]
type = "brew"          # brew | cargo | npm | pipx | go | script
package = "lazygit"    # or `command = "..."` for type=script
```

## Fields

### Required
- `name` — unique slug, lowercase, `[a-z0-9-]`.
- `description` — one line, ≤120 chars.
- `author` — GitHub handle (or other canonical handle). Informational
  only; used to show "by X" in the UI and to DM authors when curating.
  No account system; no payment linkage.
- `homepage` — canonical URL for the project.
- `[install]` — see Install Types below.

### Recommended (drives in-TUI richness)
- `readme` — raw URL to a markdown file. Rendered in detail view.
- `demo` — URL for an external demo artifact. Registry-hosted `.reel`
  previews are added separately under `reels/<name>.reel` and exposed
  in `index.json` as `has_reel`.
- `screenshots` — array of image URLs. Reserved for future UI use.
- `tags` — array of lowercase tags.
- `license` — SPDX identifier.
- `binary` — name of the installed executable when it doesn't match
  the repo basename (e.g. `cli/cli` → `gh`, `ClementTsang/bottom` →
  `btm`). Optional; clients fall back to the repo basename. Used by
  uninstall (for `type = "go"`) and installed-state detection.

  Backfill path: when the client installs an app whose produced
  binary disagrees with the derived basename, the disagreement is
  recorded in `~/.cliff/logs/bin-audit.log`. Run
  `cliff bin-audit --format=toml-patches` to turn that log into
  paste-ready `binary = "…"` overrides to PR against the registry.
  This is the lazy way to close the loop — every install is quietly
  building the worklist.

### Reserved
- `tryable` — bool. If true, a future `cliff try <name>` will run
  the app in an ephemeral sandbox. Deferred.
- `try_image` — optional Docker image for `cliff try`. Deferred.

## Install types

Manifests declare install methods with either `[install]` (single
method — the common case) or `[[installs]]` (array-of-tables, for
apps that ship through more than one package manager). Exactly one
of the two must be present; the client picks the first `[[installs]]`
entry whose tool is on the user's `$PATH`, or honors an explicit
`--via <type>` override.

```toml
[install]
type = "brew"
package = "lazygit"
```

```toml
[install]
type = "cargo"
package = "bottom"
```

```toml
[install]
type = "npm"
package = "tldr"
global = true
```

```toml
[install]
type = "pipx"
package = "httpie"
```

```toml
[install]
type = "go"
package = "github.com/charmbracelet/glow@latest"
```

```toml
[install]
type = "script"
command = "curl -fsSL https://example.com/install.sh | sh"
```

`script` is escape-hatch only. The TUI shows an extra confirmation
warning before running `script`-type installs — the warning copy
is real, not a rubber stamp — because we don't review or sandbox
what runs.

### Multi-method (`[[installs]]`)

When the same app is available via multiple package managers, use
`[[installs]]` (double-bracket, array-of-tables) instead of
`[install]`:

```toml
[[installs]]
type = "brew"
package = "chess-tui"

[[installs]]
type = "cargo"
package = "chess-tui"
```

Order matters — it's the preference order. On install, the client
picks the first entry whose tool is on `$PATH`; if the user passes
`--via <type>`, that wins. Duplicate types within `[[installs]]` are
rejected by lint.

Today uninstall and upgrade still derive from the primary (first)
method only — so for the `chess-tui` manifest above, uninstall would
run `brew uninstall chess-tui` regardless of how the user installed.
Tracking "which method did I actually use?" per install is on the
roadmap (`internal/binmap` is the groundwork).

## Uninstall and upgrade recipes

For known package managers (brew, cargo, npm, pipx, go) cliff derives
the uninstall and upgrade verbs automatically — no manifest fields
needed. Authors can override the derivation when they want something
non-standard (e.g. `brew uninstall --force`) via optional top-level
`[uninstall]` and `[upgrade]` blocks:

```toml
[uninstall]
command = "brew uninstall --force lazygit"

[upgrade]
command = "brew upgrade lazygit"
```

Each block takes a single `command` field — nothing else. The client
runs the command through the same confirm → stream-output → diagnose
pipeline as install.

### Script-type installs

**`[uninstall]` is required when `install.type = "script"`.** The lint
rejects PRs without it. There's no general reverse for a curl-pipe-sh
install; only the author knows which files were created, which paths
were modified, or which launch agents were registered. An example:

```toml
[install]
type = "script"
command = "curl -fsSL https://starship.rs/install.sh | sh"

[uninstall]
command = "rm -f /usr/local/bin/starship"
```

`[upgrade]` is **optional** even for script-type installs. If absent,
`cliff upgrade` refuses with an honest "no upgrade recipe" error
rather than silently re-running the install script — some installers
are idempotent, some break on re-run, and the safe default is to
ask the author to be explicit.

## Validation

CI validates every PR via the standalone `cmd/lint` program in
the registry tree. A future `cliff lint` subcommand will replace
it.

Checks:
- Schema validity.
- `name` uniqueness; matches `[a-z0-9][a-z0-9-]*`.
- `tags` are lowercase.
- URL shape (readme, homepage, screenshots, demo).
- Exactly one of `[install]` or `[[installs]]` is declared.
- Within `[[installs]]`, each entry's type is unique.
- `[uninstall]` present when any method has `type = "script"`.
- `[uninstall]` / `[upgrade]` blocks, if present, have non-empty `command`.
- GitHub star count + last-commit timestamp snapshotted at build
  time into `index.json` (so the client sorts/displays without
  live GitHub calls and we don't burn rate limit at runtime).

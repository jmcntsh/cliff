# cliff — Manifest Schema v0

One TOML file per app, lives at `apps/<name>.toml` in
`cliffsh/registry`. CI validates and compiles all manifests into
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
- `demo` — asciinema cast URL or ID. Will play in-TUI in a future
  release; currently informational.
- `screenshots` — array of image URLs. Currently listed as URLs;
  in-TUI rendering planned.
- `tags` — array of lowercase tags. CI lowercases and dedupes.
- `license` — SPDX identifier.
- `binary` — name of the installed executable when it doesn't match
  the repo basename (e.g. `cli/cli` → `gh`, `ClementTsang/bottom` →
  `btm`). Optional; clients fall back to the repo basename. Used by
  uninstall (for `type = "go"`) and installed-state detection.

### Reserved
- `tryable` — bool. If true, a future `cliff try <name>` will run
  the app in an ephemeral sandbox. Deferred.
- `try_image` — optional Docker image for `cliff try`. Deferred.

## Install types

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
- `name` uniqueness; matches `[a-z0-9-]`.
- `tags` lowercased and deduped.
- URL reachability (readme, homepage, screenshots, demo).
- `[uninstall]` present when `install.type = "script"`.
- `[uninstall]` / `[upgrade]` blocks, if present, have non-empty `command`.
- GitHub star count + last-commit timestamp snapshotted at build
  time into `index.json` (so the client sorts/displays without
  live GitHub calls and we don't burn rate limit at runtime).

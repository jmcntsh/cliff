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

## Validation

CI validates every PR via the standalone `cmd/lint` program in
the registry tree. A future `cliff lint` subcommand will replace
it.

Checks:
- Schema validity.
- `name` uniqueness; matches `[a-z0-9-]`.
- `tags` lowercased and deduped.
- URL reachability (readme, homepage, screenshots, demo).
- GitHub star count + last-commit timestamp snapshotted at build
  time into `index.json` (so the client sorts/displays without
  live GitHub calls and we don't burn rate limit at runtime).

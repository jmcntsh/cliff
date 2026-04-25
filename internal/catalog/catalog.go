package catalog

import (
	"encoding/json"
	"strings"
	"time"
)

type App struct {
	Name        string `json:"name"`
	Repo        string `json:"repo"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Language    string `json:"language"`
	Stars       int    `json:"stars"`
	Homepage    string `json:"homepage"`

	Author      string   `json:"author,omitempty"`
	Readme      string   `json:"readme,omitempty"`
	Demo        string   `json:"demo,omitempty"`
	Screenshots []string `json:"screenshots,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Binary      string   `json:"binary,omitempty"` // override for the installed executable name; defaults to repo basename
	// InstallSpecs is the ordered list of install methods. Single-method
	// manifests produce a one-element slice; multi-method ([[installs]])
	// manifests produce the list in author order, which is also the
	// client's default preference order when picking. Callers that only
	// need the primary can use PrimaryInstallSpec; multi-method-aware
	// paths use PreferredInstallSpec to honor a `--via` override and
	// tool-availability detection.
	InstallSpecs []InstallSpec `json:"install_specs,omitempty"`

	// LastCommit is the most recent commit on the project's default
	// branch, snapshotted at index build time. Already emitted by the
	// registry's CI; used here to power the "New" sidebar surface and
	// as a fallback signal for freshness when AddedAt isn't set yet.
	LastCommit time.Time `json:"last_commit,omitempty"`
	// AddedAt is when the manifest first landed in the registry. The
	// registry doesn't populate this yet (planned), so clients treat
	// zero as "unknown, fall back to LastCommit for the New filter".
	// When present, it takes precedence: a freshly-added app with an
	// old last commit (e.g. a well-maintained classic a curator just
	// noticed) should still appear under New.
	AddedAt time.Time `json:"added_at,omitempty"`
	// Optional author-provided recipes. Required for type=script (no
	// general reverse exists); optional otherwise, where presence
	// overrides the derivation in InstallSpec.UninstallShell/UpgradeShell.
	UninstallSpec *CommandSpec `json:"uninstall_spec,omitempty"`
	UpgradeSpec   *CommandSpec `json:"upgrade_spec,omitempty"`
}

// CommandSpec is the wire shape for the [uninstall] and [upgrade]
// manifest blocks: just a shell command to run. Modeled separately
// from InstallSpec because these blocks are structurally simpler —
// no type switch, no package/global fields.
type CommandSpec struct {
	Command string `json:"command"`
}

type InstallSpec struct {
	Type    string `json:"type"`              // brew | cargo | npm | pipx | go | script
	Package string `json:"package,omitempty"` // for non-script types
	Command string `json:"command,omitempty"` // for type=script
	Global  bool   `json:"global,omitempty"`  // npm: pass -g
}

func (s *InstallSpec) Shell() string {
	if s == nil {
		return ""
	}
	switch s.Type {
	case "brew":
		return "brew install " + s.Package
	case "cargo":
		return "cargo install " + s.Package
	case "npm":
		if s.Global {
			return "npm install -g " + s.Package
		}
		return "npm install " + s.Package
	case "pipx":
		return "pipx install " + s.Package
	case "go":
		return "go install " + s.Package
	case "script":
		return s.Command
	}
	return ""
}

// UninstallShell returns the shell command to uninstall the app, or ""
// if uninstall isn't supported for this install type. The binary
// argument is the installed executable name; required for type=go
// (since go install leaves a binary at $GOBIN/<name> with no built-in
// uninstall) and ignored otherwise. type=script is always unsupported
// here — manifests must ship an explicit [uninstall] block for that
// case (see notes/cli-verbs.md), which is not yet wired in.
func (s *InstallSpec) UninstallShell(binary string) string {
	if s == nil {
		return ""
	}
	switch s.Type {
	case "brew":
		return "brew uninstall " + s.Package
	case "cargo":
		return "cargo uninstall " + s.Package
	case "npm":
		if s.Global {
			return "npm uninstall -g " + s.Package
		}
		return "npm uninstall " + s.Package
	case "pipx":
		return "pipx uninstall " + s.Package
	case "go":
		if binary == "" {
			return ""
		}
		// Ask `go env` at runtime — asdf and other toolchain managers set
		// GOBIN/GOPATH per-process, not in the user's shell, so shell
		// expansion of those vars ends up wrong (e.g. asdf users would
		// rm from ~/go/bin when the real binary lives under ~/.asdf).
		// The guard on empty $b prevents rm'ing /<binary> if `go` itself
		// isn't installed. The actual "did it work" gate is the post-
		// uninstall Detect check in cmd/cliff/pkgverbs.go.
		return `b="$(go env GOBIN)"; [ -z "$b" ] && b="$(go env GOPATH)/bin"; [ -n "$b" ] && rm -f "$b/` + binary + `"`
	}
	return ""
}

// UpgradeShell returns the shell command to upgrade the app to its
// latest version, or "" if upgrade isn't supported. type=script isn't
// supported — re-running an arbitrary install script isn't safe in the
// general case and the author-owned case needs a manifest block.
func (s *InstallSpec) UpgradeShell() string {
	if s == nil {
		return ""
	}
	switch s.Type {
	case "brew":
		return "brew upgrade " + s.Package
	case "cargo":
		return "cargo install --force " + s.Package
	case "npm":
		if s.Global {
			return "npm install -g " + s.Package + "@latest"
		}
		return "npm install " + s.Package + "@latest"
	case "pipx":
		return "pipx upgrade " + s.Package
	case "go":
		return "go install " + goLatestPath(s.Package)
	}
	return ""
}

// goLatestPath normalizes a Go module path to pin @latest. Accepts
// both "example.com/x/y" (no version) and "example.com/x/y@v1.2.3"
// and returns "example.com/x/y@latest".
func goLatestPath(pkg string) string {
	if i := strings.Index(pkg, "@"); i >= 0 {
		return pkg[:i] + "@latest"
	}
	return pkg + "@latest"
}

// ResolvedBinaryName returns the best answer we have for "what will
// the user type to run this?". Precedence, strongest to weakest:
//
//  1. An override learned from a previous install's scraped output
//     (internal/binmap). This wins over everything because it's
//     ground truth — we saw the installer produce that file.
//  2. The manifest's explicit Binary field.
//  3. The repo basename.
//
// The override argument is the caller's in-memory copy of the binmap
// (usually loaded once at startup and passed around). Missing keys
// are fine — they just fall through to the manifest/basename path,
// which is the pre-binmap behavior.
func (a *App) ResolvedBinaryName(overrides map[string]string) string {
	if a == nil {
		return ""
	}
	if overrides != nil {
		if b, ok := overrides[a.Repo]; ok && b != "" {
			return b
		}
	}
	return a.BinaryName()
}

// FreshnessTime returns the timestamp the "New" filter should compare
// against. Precedence: AddedAt (when the registry stamps it) → LastCommit
// → zero time. Centralized here so the sidebar, grid badge, and any
// future "new this week" surface all agree on the same rule.
func (a *App) FreshnessTime() time.Time {
	if a == nil {
		return time.Time{}
	}
	if !a.AddedAt.IsZero() {
		return a.AddedAt
	}
	return a.LastCommit
}

// BinaryName returns the expected executable name for the app. If the
// manifest sets an explicit Binary, that wins; otherwise we fall back
// to the repo basename (charmbracelet/glow → "glow"), which matches
// the convention for ~90% of CLI TUIs.
func (a *App) BinaryName() string {
	if a == nil {
		return ""
	}
	if a.Binary != "" {
		return a.Binary
	}
	if i := strings.LastIndex(a.Repo, "/"); i >= 0 {
		return a.Repo[i+1:]
	}
	return a.Repo
}

// UninstallCommand returns the shell command to uninstall the app, or
// "" if uninstall isn't supported. An explicit UninstallSpec from the
// manifest wins; otherwise we derive from InstallSpec.UninstallShell.
// This is how script-type installs get their uninstall recipe (the
// manifest lint enforces that UninstallSpec is present for them), and
// how authors override the derived command for known managers.
func (a *App) UninstallCommand() string {
	return a.UninstallCommandWithOverrides(nil)
}

// UninstallCommandWithOverrides is UninstallCommand but uses a
// binmap-style overrides map to pick the binary name passed to
// UninstallShell. Matters for type=go where the derived uninstall
// command `rm -f $GOBIN/<name>` needs the *actual* binary name —
// not the repo basename — or it silently removes nothing. The CLI
// uninstall path consults this so a repo-basename mismatch (e.g.
// minesweep-rs → minesweep) doesn't cause a phantom-success.
func (a *App) UninstallCommandWithOverrides(overrides map[string]string) string {
	if a == nil {
		return ""
	}
	if a.UninstallSpec != nil && a.UninstallSpec.Command != "" {
		return a.UninstallSpec.Command
	}
	return a.PrimaryInstallSpec().UninstallShell(a.ResolvedBinaryName(overrides))
}

// UpgradeCommand returns the shell command to upgrade the app, or ""
// if upgrade isn't supported. Override-first like UninstallCommand. For
// script-type installs, UpgradeSpec is optional: absent means we refuse
// to upgrade (honest error at the CLI), because silently re-running an
// install script isn't always safe.
func (a *App) UpgradeCommand() string {
	if a == nil {
		return ""
	}
	if a.UpgradeSpec != nil && a.UpgradeSpec.Command != "" {
		return a.UpgradeSpec.Command
	}
	return a.PrimaryInstallSpec().UpgradeShell()
}

// PrimaryInstallSpec returns the first install method, or nil if the
// app has none. This is the right choice when a caller needs "the one
// install method" — display, type-check, default derivations — and
// doesn't care about multi-method picking. Callers that run an install
// and need to honor user overrides or tool-availability should use
// PreferredInstallSpec instead.
func (a *App) PrimaryInstallSpec() *InstallSpec {
	if a == nil || len(a.InstallSpecs) == 0 {
		return nil
	}
	return &a.InstallSpecs[0]
}

// PreferredInstallSpec picks which install method to run for this app.
//
//   - If `typeHint` is non-empty (e.g. from `--via brew`), return the
//     matching method, or nil if the manifest doesn't offer it.
//   - Otherwise, return the first method whose tool reports available
//     via `isAvailable`. For type=script, availability is assumed true
//     (nothing to probe for; the shell runs the command as-is).
//   - Otherwise, fall back to the primary (first) method, so we
//     surface the tool's own "command not found" error via
//     install.Diagnose rather than a cliff-side refusal.
//
// `isAvailable` may be nil, in which case only the type-hint and
// primary-fallback paths apply. Returns nil only when the app has no
// install methods at all.
func (a *App) PreferredInstallSpec(typeHint string, isAvailable func(installType string) bool) *InstallSpec {
	if a == nil || len(a.InstallSpecs) == 0 {
		return nil
	}
	if typeHint != "" {
		for i := range a.InstallSpecs {
			if a.InstallSpecs[i].Type == typeHint {
				return &a.InstallSpecs[i]
			}
		}
		return nil
	}
	if isAvailable != nil {
		for i := range a.InstallSpecs {
			t := a.InstallSpecs[i].Type
			if t == "script" || isAvailable(t) {
				return &a.InstallSpecs[i]
			}
		}
	}
	return &a.InstallSpecs[0]
}

// UnmarshalJSON supports both the current wire shape (`install_specs`)
// and the pre-multi-method shape (`install_spec`, singular). This lets
// a client binary built after the schema change still parse an embedded
// snapshot or cached `index.json` that was produced by an earlier
// registry build. The reverse — new wire, old binary — breaks and
// that's fine; we own both ends and the wire moves forward.
func (a *App) UnmarshalJSON(data []byte) error {
	// Alias sidesteps the infinite-recursion trap of calling json.Unmarshal
	// on *App (which would invoke this method again). The alias has the
	// same field layout but no UnmarshalJSON method.
	type appAlias App
	var raw struct {
		appAlias
		LegacyInstallSpec *InstallSpec `json:"install_spec,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*a = App(raw.appAlias)
	if len(a.InstallSpecs) == 0 && raw.LegacyInstallSpec != nil {
		a.InstallSpecs = []InstallSpec{*raw.LegacyInstallSpec}
	}
	return nil
}

type Category struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type Catalog struct {
	SchemaVersion int        `json:"schema_version"`
	GeneratedAt   time.Time  `json:"generated_at"`
	SourceCommit  string     `json:"source_commit"`
	Apps          []App      `json:"apps"`
	Categories    []Category `json:"categories"`
}

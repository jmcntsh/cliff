package catalog

import (
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

	Author      string       `json:"author,omitempty"`
	Readme      string       `json:"readme,omitempty"`
	Demo        string       `json:"demo,omitempty"`
	Screenshots []string     `json:"screenshots,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	InstallSpec *InstallSpec `json:"install_spec,omitempty"`
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
		return `rm -f "${GOBIN:-${GOPATH:-$HOME/go}/bin}/` + binary + `"`
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

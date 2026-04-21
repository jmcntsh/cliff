package catalog

import (
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

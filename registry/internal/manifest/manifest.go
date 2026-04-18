// Package manifest defines the on-disk TOML schema for cliff registry
// entries and the validation rules. Both `cmd/lint` and `cmd/build`
// consume this package; the client never does (it reads the compiled
// index.json instead).
package manifest

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/jmcntsh/cliff/internal/catalog"
)

type Manifest struct {
	Name        string   `toml:"name"`
	Description string   `toml:"description"`
	Author      string   `toml:"author"`
	Homepage    string   `toml:"homepage"`
	Readme      string   `toml:"readme,omitempty"`
	Demo        string   `toml:"demo,omitempty"`
	Screenshots []string `toml:"screenshots,omitempty"`
	Tags        []string `toml:"tags,omitempty"`
	License     string   `toml:"license,omitempty"`
	Category    string   `toml:"category,omitempty"`
	Language    string   `toml:"language,omitempty"`

	Install Install `toml:"install"`
}

type Install struct {
	Type    string `toml:"type"`
	Package string `toml:"package,omitempty"`
	Command string `toml:"command,omitempty"`
	Global  bool   `toml:"global,omitempty"`
}

var (
	nameRE     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	validTypes = map[string]bool{
		"brew": true, "cargo": true, "npm": true,
		"pipx": true, "go": true, "script": true,
	}
)

const MaxDescriptionLen = 120

func (m *Manifest) Validate() error {
	var errs []string

	if m.Name == "" {
		errs = append(errs, "name is required")
	} else if !nameRE.MatchString(m.Name) {
		errs = append(errs, fmt.Sprintf("name %q must match [a-z0-9][a-z0-9-]*", m.Name))
	}

	if m.Description == "" {
		errs = append(errs, "description is required")
	} else if len(m.Description) > MaxDescriptionLen {
		errs = append(errs, fmt.Sprintf("description is %d chars (max %d)", len(m.Description), MaxDescriptionLen))
	}

	if m.Author == "" {
		errs = append(errs, "author is required")
	}

	if err := requireURL("homepage", m.Homepage, true); err != nil {
		errs = append(errs, err.Error())
	}
	if err := requireURL("readme", m.Readme, false); err != nil {
		errs = append(errs, err.Error())
	}
	if err := requireURL("demo", m.Demo, false); err != nil {
		errs = append(errs, err.Error())
	}
	for i, s := range m.Screenshots {
		if err := requireURL(fmt.Sprintf("screenshots[%d]", i), s, true); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if m.Install.Type == "" {
		errs = append(errs, "install.type is required")
	} else if !validTypes[m.Install.Type] {
		errs = append(errs, fmt.Sprintf("install.type %q is not one of brew|cargo|npm|pipx|go|script", m.Install.Type))
	} else if m.Install.Type == "script" {
		if m.Install.Command == "" {
			errs = append(errs, "install.command is required when install.type=\"script\"")
		}
		if m.Install.Package != "" {
			errs = append(errs, "install.package must be empty when install.type=\"script\"")
		}
	} else {
		if m.Install.Package == "" {
			errs = append(errs, fmt.Sprintf("install.package is required when install.type=%q", m.Install.Type))
		}
		if m.Install.Command != "" {
			errs = append(errs, "install.command is only valid when install.type=\"script\"")
		}
		if m.Install.Global && m.Install.Type != "npm" {
			errs = append(errs, "install.global only applies to install.type=\"npm\"")
		}
	}

	for i, t := range m.Tags {
		if t != strings.ToLower(t) {
			errs = append(errs, fmt.Sprintf("tags[%d] %q must be lowercase", i, t))
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(errs, "; "))
}

func requireURL(field, raw string, required bool) error {
	if raw == "" {
		if required {
			return fmt.Errorf("%s is required", field)
		}
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s: %v", field, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%s: scheme must be http or https (got %q)", field, u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("%s: missing host", field)
	}
	return nil
}

// ToApp converts a validated manifest into the catalog.App shape that the
// client consumes. Stars and SourceCommit are out of band — the build
// step fills them in from a snapshot taken at index-build time.
func (m *Manifest) ToApp() catalog.App {
	repo := m.Author + "/" + m.Name
	if u, err := url.Parse(m.Homepage); err == nil && u.Host == "github.com" {
		trimmed := strings.Trim(u.Path, "/")
		if trimmed != "" {
			repo = trimmed
		}
	}

	return catalog.App{
		Name:        m.Name,
		Repo:        repo,
		Description: m.Description,
		Category:    fallback(m.Category, "Other"),
		Language:    m.Language,
		License:     m.License,
		Homepage:    m.Homepage,
		Author:      m.Author,
		Readme:      m.Readme,
		Demo:        m.Demo,
		Screenshots: m.Screenshots,
		Tags:        m.Tags,
		InstallSpec: &catalog.InstallSpec{
			Type:    m.Install.Type,
			Package: m.Install.Package,
			Command: m.Install.Command,
			Global:  m.Install.Global,
		},
	}
}

func fallback(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

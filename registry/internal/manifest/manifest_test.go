package manifest

import "testing"

func validBase() Manifest {
	return Manifest{
		Name:        "lazygit",
		Description: "Simple terminal UI for git commands",
		Author:      "jesseduffield",
		Homepage:    "https://github.com/jesseduffield/lazygit",
		Install:     Install{Type: "brew", Package: "lazygit"},
	}
}

func TestValidate_OK(t *testing.T) {
	m := validBase()
	if err := m.Validate(); err != nil {
		t.Fatalf("expected ok, got: %v", err)
	}
}

func TestValidate_RequiredFields(t *testing.T) {
	cases := map[string]func(*Manifest){
		"name":               func(m *Manifest) { m.Name = "" },
		"description":        func(m *Manifest) { m.Description = "" },
		"author":             func(m *Manifest) { m.Author = "" },
		"homepage":           func(m *Manifest) { m.Homepage = "" },
		"install.type":       func(m *Manifest) { m.Install.Type = "" },
		"install.package":    func(m *Manifest) { m.Install.Package = "" },
		"bad name":           func(m *Manifest) { m.Name = "Lazy_Git" },
		"bad install type":   func(m *Manifest) { m.Install.Type = "snap" },
		"bad homepage":       func(m *Manifest) { m.Homepage = "not-a-url" },
		"description too long": func(m *Manifest) {
			m.Description = ""
			for i := 0; i < MaxDescriptionLen+1; i++ {
				m.Description += "x"
			}
		},
		"uppercase tag": func(m *Manifest) { m.Tags = []string{"Git"} },
		"global on non-npm": func(m *Manifest) {
			m.Install = Install{Type: "brew", Package: "x", Global: true}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := validBase()
			mutate(&m)
			if err := m.Validate(); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidate_Script(t *testing.T) {
	m := validBase()
	m.Install = Install{Type: "script", Command: "curl -fsSL https://x.io/i.sh | sh"}
	if err := m.Validate(); err != nil {
		t.Fatalf("script ok case: %v", err)
	}
	m.Install.Package = "x"
	if err := m.Validate(); err == nil {
		t.Fatal("expected error when both command and package set on script")
	}
}

func TestToApp(t *testing.T) {
	m := validBase()
	app := m.ToApp()
	if app.Repo != "jesseduffield/lazygit" {
		t.Errorf("repo = %q, want jesseduffield/lazygit", app.Repo)
	}
	if app.InstallSpec == nil || app.InstallSpec.Shell() != "brew install lazygit" {
		t.Errorf("install spec wrong: %+v", app.InstallSpec)
	}
}

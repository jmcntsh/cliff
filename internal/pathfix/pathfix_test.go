package pathfix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyShell(t *testing.T) {
	cases := []struct {
		in   string
		want ShellKind
	}{
		{"/bin/zsh", ShellZsh},
		{"/usr/local/bin/zsh-static", ShellZsh},
		{"/bin/bash", ShellBash},
		{"/usr/local/bin/fish", ShellFish},
		{"", ShellUnknown},
		{"/bin/ksh", ShellUnknown},
	}
	for _, tc := range cases {
		if got := classifyShell(tc.in); got != tc.want {
			t.Errorf("classifyShell(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestDetect_Zsh_NewFile is the first-run case: ~/.zshrc doesn't
// exist yet. Plan should point at ~/.zshrc under the isolated HOME
// with the right line and Present=false.
func TestDetect_Zsh_NewFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")

	p, err := Detect("/Users/jmc/go/bin")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if p.Shell != ShellZsh {
		t.Errorf("Shell = %v, want ShellZsh", p.Shell)
	}
	if want := filepath.Join(home, ".zshrc"); p.RcPath != want {
		t.Errorf("RcPath = %q, want %q", p.RcPath, want)
	}
	if !strings.Contains(p.Line, "/Users/jmc/go/bin") {
		t.Errorf("Line missing dir: %q", p.Line)
	}
	if p.Present {
		t.Error("Present should be false for missing file")
	}
}

// TestDetect_Bash_PrefersBashrc pins the ~/.bashrc-over-.bash_profile
// rule when both are absent (new-user case).
func TestDetect_Bash_PrefersBashrcWhenBothMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")

	p, err := Detect("/opt/tools/bin")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if want := filepath.Join(home, ".bashrc"); p.RcPath != want {
		t.Errorf("RcPath = %q, want %q", p.RcPath, want)
	}
}

// TestDetect_Bash_FallsBackToBashProfile covers the macOS default
// where only ~/.bash_profile exists.
func TestDetect_Bash_FallsBackToBashProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	bashProfile := filepath.Join(home, ".bash_profile")
	if err := os.WriteFile(bashProfile, []byte("# existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := Detect("/opt/tools/bin")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if p.RcPath != bashProfile {
		t.Errorf("RcPath = %q, want %q", p.RcPath, bashProfile)
	}
}

// TestDetect_Fish_ReturnsUnsupported guarantees we don't auto-edit a
// fish config with bash syntax.
func TestDetect_Fish_ReturnsUnsupported(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/usr/local/bin/fish")

	p, err := Detect("/opt/tools/bin")
	if err != ErrShellUnsupported {
		t.Fatalf("err = %v, want ErrShellUnsupported", err)
	}
	if p == nil {
		t.Fatal("plan should still be non-nil on ErrShellUnsupported so the UI can show the correct hand-edit line")
	}
	if !strings.Contains(p.Line, "fish_add_path") {
		t.Errorf("fish plan should use fish_add_path, got %q", p.Line)
	}
}

// TestApply_AppendsWhenAbsent is the happy path: rc exists, our line
// is not in it, Apply writes it.
func TestApply_AppendsWhenAbsent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")
	rc := filepath.Join(home, ".zshrc")
	pre := "# some existing zshrc content\nalias ll='ls -la'\n"
	if err := os.WriteFile(rc, []byte(pre), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := Detect("/Users/jmc/go/bin")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if p.Present {
		t.Fatal("line shouldn't be present yet")
	}
	if err := Apply(p); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	data, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, pre) {
		t.Error("Apply should not disturb existing content")
	}
	if !strings.Contains(got, p.Line) {
		t.Errorf("Apply didn't append line %q; rc now:\n%s", p.Line, got)
	}
	if !strings.Contains(got, p.Marker) {
		t.Errorf("Apply didn't include marker %q", p.Marker)
	}
	if !p.Present {
		t.Error("Present should flip to true after Apply")
	}
}

// TestApply_IdempotentWhenPresent: if the exact line is already in
// the rc, Apply must not append a duplicate.
func TestApply_IdempotentWhenPresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")
	rc := filepath.Join(home, ".zshrc")
	existing := `export PATH="/Users/jmc/go/bin:$PATH"` + "\n"
	if err := os.WriteFile(rc, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := Detect("/Users/jmc/go/bin")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !p.Present {
		t.Fatal("Present should be true when line already in rc")
	}
	if err := Apply(p); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	data, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	// Line should appear exactly once — once before Apply, not twice.
	if count := strings.Count(string(data), p.Line); count != 1 {
		t.Errorf("line appears %d times, want 1; rc:\n%s", count, string(data))
	}
}

// TestApply_CreatesRcIfMissing — new user has no ~/.zshrc; Apply
// should create it rather than erroring out. The whole point is
// zero-friction; making them touch the file first defeats that.
func TestApply_CreatesRcIfMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")

	p, err := Detect("/Users/jmc/go/bin")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if err := Apply(p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	data, err := os.ReadFile(p.RcPath)
	if err != nil {
		t.Fatalf("rc not created: %v", err)
	}
	if !strings.Contains(string(data), p.Line) {
		t.Errorf("fresh rc missing line %q; got:\n%s", p.Line, string(data))
	}
}

// TestApply_FishReturnsUnsupported guards against a future refactor
// ever letting Apply write to fish.
func TestApply_FishReturnsUnsupported(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/usr/local/bin/fish")

	p, _ := Detect("/opt/tools/bin")
	if err := Apply(p); err != ErrShellUnsupported {
		t.Errorf("Apply on fish plan = %v, want ErrShellUnsupported", err)
	}
}

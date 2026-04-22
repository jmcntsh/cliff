// Package pathfix appends a single "export PATH=..." line to the
// user's shell rc so a binary that just got installed into a known
// manager dir (~/go/bin, ~/.cargo/bin, ~/.local/bin) becomes
// discoverable in every new terminal.
//
// The contract is deliberately narrow:
//
//   - We only ever *append* one line plus a comment marker. We never
//     edit or rewrite existing lines, and we never delete anything.
//   - We no-op if the exact export line is already present anywhere
//     in the file. Idempotent: running twice is the same as once.
//   - We only support zsh and bash in v1. Fish uses `fish_add_path`
//     and a different syntax; we detect it and return ShellUnsupported
//     so the UI can fall back to "here's the line, add it yourself"
//     rather than corrupting the user's config.
//
// This package does not reload the current shell. It can't — child
// processes can't mutate a parent shell's environment. Callers must
// tell the user to open a new terminal (or `source` the rc) after.
package pathfix

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ShellKind identifies the login shell we're targeting, which
// dictates both the file we edit and the syntax we write.
type ShellKind int

const (
	ShellUnknown ShellKind = iota
	ShellZsh
	ShellBash
	ShellFish // detected but not auto-edited in v1
)

// ErrShellUnsupported is returned when we can detect the shell but
// don't want to auto-edit its config (currently: fish). Callers
// should fall back to showing the user the command to run by hand.
var ErrShellUnsupported = errors.New("pathfix: shell not supported for auto-edit")

// Plan is what will be written (or was written) to make dir runnable
// in new shells. Returning this from Detect lets the UI preview the
// exact file path and line before the user confirms.
type Plan struct {
	Shell    ShellKind
	RcPath   string // absolute path of the rc file we'd append to
	Dir      string // the bin directory being added to PATH
	Line     string // the literal line we'd write
	Marker   string // the comment line we write above Line
	Present  bool   // true when Line is already in RcPath (no-op)
}

// Detect plans the edit for dir based on the current user's shell.
// It reads $SHELL to pick the rc file and builds the exact line that
// would be appended. No filesystem writes happen here; Plan.RcPath
// may point at a file that doesn't yet exist (we'll create it).
func Detect(dir string) (*Plan, error) {
	if dir == "" {
		return nil, errors.New("pathfix: empty dir")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("pathfix: user home: %w", err)
	}

	kind := classifyShell(os.Getenv("SHELL"))
	plan := &Plan{
		Shell:  kind,
		Dir:    dir,
		Marker: "# added by cliff — makes PATH-installed binaries discoverable",
	}

	switch kind {
	case ShellZsh:
		plan.RcPath = filepath.Join(home, ".zshrc")
		plan.Line = fmt.Sprintf(`export PATH="%s:$PATH"`, dir)
	case ShellBash:
		// Prefer ~/.bashrc; if only ~/.bash_profile exists (macOS
		// default for login shells) we target that instead. New
		// files get ~/.bashrc as the sensible default.
		bashrc := filepath.Join(home, ".bashrc")
		bashProfile := filepath.Join(home, ".bash_profile")
		if !fileExists(bashrc) && fileExists(bashProfile) {
			plan.RcPath = bashProfile
		} else {
			plan.RcPath = bashrc
		}
		plan.Line = fmt.Sprintf(`export PATH="%s:$PATH"`, dir)
	case ShellFish:
		plan.RcPath = filepath.Join(home, ".config", "fish", "config.fish")
		plan.Line = fmt.Sprintf(`fish_add_path %s`, dir)
		return plan, ErrShellUnsupported
	default:
		return plan, ErrShellUnsupported
	}

	present, err := linePresent(plan.RcPath, plan.Line)
	if err != nil {
		return plan, err
	}
	plan.Present = present
	return plan, nil
}

// Apply executes the plan: if the line isn't already present, append
// it (with a marker comment and leading blank line for readability).
// Safe to call when Plan.Present is true — it just no-ops.
func Apply(p *Plan) error {
	if p == nil {
		return errors.New("pathfix: nil plan")
	}
	if p.Shell == ShellFish || p.Shell == ShellUnknown {
		return ErrShellUnsupported
	}
	if p.Present {
		return nil
	}
	if err := ensureParentDir(p.RcPath); err != nil {
		return fmt.Errorf("pathfix: prepare rc dir: %w", err)
	}
	// O_APPEND so we never truncate, and we write a leading newline
	// defensively in case the existing file doesn't end in one (the
	// single worst way to break someone's rc is to concatenate two
	// statements onto the same line).
	f, err := os.OpenFile(p.RcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("pathfix: open %s: %w", p.RcPath, err)
	}
	defer f.Close()

	block := "\n" + p.Marker + "\n" + p.Line + "\n"
	if _, err := f.WriteString(block); err != nil {
		return fmt.Errorf("pathfix: write %s: %w", p.RcPath, err)
	}
	p.Present = true
	return nil
}

// classifyShell maps $SHELL to a ShellKind by suffix. We match on the
// basename so values like "/usr/local/bin/zsh-static" still resolve.
func classifyShell(shellEnv string) ShellKind {
	base := filepath.Base(shellEnv)
	switch {
	case base == "":
		return ShellUnknown
	case strings.Contains(base, "zsh"):
		return ShellZsh
	case strings.Contains(base, "bash"):
		return ShellBash
	case strings.Contains(base, "fish"):
		return ShellFish
	default:
		return ShellUnknown
	}
}

// linePresent returns true if target appears as a trimmed line
// anywhere in path. Missing file is treated as "not present, no
// error" — that's the "first run, rc doesn't exist yet" case.
func linePresent(path, target string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	target = strings.TrimSpace(target)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == target {
			return true, nil
		}
	}
	return false, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func ensureParentDir(p string) error {
	return os.MkdirAll(filepath.Dir(p), 0o755)
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// installState mirrors ~/.cliff/install.json — the breadcrumb that
// scripts/install.sh writes at install time so self-uninstall knows
// exactly where the binary went, instead of having to re-derive a
// candidate-path list that drifts from install.sh's actual order.
//
// The file is only present for script-type installs (curl cliff.sh |
// sh). brew tap, `go install`, and other managers don't write it —
// self-uninstall falls back to os.Executable() in those cases, with
// extra guards so we don't blindly rm a manager-owned file.
type installState struct {
	InstallDir    string `json:"install_dir"`
	InstallMethod string `json:"install_method"`
	Version       string `json:"version,omitempty"`
}

const helpSelfUninstall = `cliff self-uninstall — remove cliff from your system.

Reads ~/.cliff/install.json (written by 'curl cliff.sh | sh' at
install time) to find where the binary lives. If the file is missing
(brew tap, 'go install', etc.) falls back to the running binary's
path. Then removes the binary and the ~/.cliff data directory.

Refuses to act on binaries that look manager-owned (symlinks, paths
inside a Homebrew Cellar). Use the matching package manager for
those:

  brew install: brew uninstall cliff
  go install:   rm "$(which cliff)"

Usage:
  cliff self-uninstall            remove the binary and ~/.cliff
  cliff self-uninstall --dry-run  print the plan, change nothing
`

func cmdSelfUninstall(args []string) int {
	dryRun := false
	for _, a := range args {
		switch a {
		case "--dry-run", "-n":
			dryRun = true
		case "help", "--help", "-h":
			fmt.Print(helpSelfUninstall)
			return 0
		default:
			fmt.Fprintf(os.Stderr, "cliff: unknown flag %q\n", a)
			fmt.Fprintln(os.Stderr, "run 'cliff self-uninstall --help' for usage")
			return 2
		}
	}

	binPath, source, err := resolveSelfBinary()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cliff:", err)
		return 1
	}

	if reason := managerOwned(binPath); reason != "" {
		fmt.Fprintf(os.Stderr, "cliff: refusing to remove %s\n", binPath)
		fmt.Fprintf(os.Stderr, "  %s\n", reason)
		return 1
	}

	dataDir, _ := cliffDataDir()

	fmt.Printf("self-uninstall plan (binary path source: %s):\n", source)
	fmt.Printf("  rm    %s\n", binPath)
	if dataDir != "" {
		if _, statErr := os.Stat(dataDir); statErr == nil {
			fmt.Printf("  rm -r %s\n", dataDir)
		}
	}

	if dryRun {
		fmt.Println("\n(dry run — no changes made)")
		return 0
	}

	fmt.Println()
	if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "cliff: removing %s: %v\n", binPath, err)
		return 1
	}
	fmt.Printf("removed %s\n", binPath)

	if dataDir != "" {
		if _, statErr := os.Stat(dataDir); statErr == nil {
			if err := os.RemoveAll(dataDir); err != nil {
				fmt.Fprintf(os.Stderr, "cliff: removing %s: %v\n", dataDir, err)
				return 1
			}
			fmt.Printf("removed %s\n", dataDir)
		}
	}

	fmt.Println("\n✓ cliff uninstalled.")
	return 0
}

// cliffDataDir returns ~/.cliff. Empty string + nil error when HOME
// can't be resolved — caller treats that as "skip data cleanup,"
// which is the right thing to do rather than RemoveAll-ing /.cliff.
func cliffDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if home == "" {
		return "", nil
	}
	return filepath.Join(home, ".cliff"), nil
}

func installStatePath() (string, error) {
	dir, err := cliffDataDir()
	if err != nil || dir == "" {
		return "", err
	}
	return filepath.Join(dir, "install.json"), nil
}

// loadInstallState reads ~/.cliff/install.json. Missing file returns
// (nil, nil) — that's the expected case for tap/go-install users and
// the caller falls through to os.Executable().
func loadInstallState() (*installState, error) {
	p, err := installStatePath()
	if err != nil || p == "" {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s installState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("install.json corrupt: %w", err)
	}
	return &s, nil
}

// resolveSelfBinary returns the cliff binary path to remove and a
// short tag for the user-facing plan header. install.json wins when
// present because it's exact (install.sh wrote it at install time);
// os.Executable is the fallback for installs that didn't go through
// our script.
func resolveSelfBinary() (string, string, error) {
	if s, err := loadInstallState(); err == nil && s != nil && s.InstallDir != "" {
		p := filepath.Join(s.InstallDir, "cliff")
		if _, statErr := os.Stat(p); statErr == nil {
			return p, "~/.cliff/install.json", nil
		}
		// install.json points at a path that no longer exists
		// (someone rm'd it manually, or it was moved). Fall through
		// to os.Executable rather than failing — better to remove
		// the binary the user is actually running than to give up.
	}
	exe, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("could not locate cliff binary: %w", err)
	}
	return exe, "os.Executable()", nil
}

// managerOwned returns a non-empty reason string when binPath looks
// like it belongs to a package manager that should do its own
// uninstall. Empty string means "safe to remove."
//
// Two heuristics, both intentionally conservative:
//
//  1. Symlink. brew puts /opt/homebrew/bin/cliff as a symlink into
//     /opt/homebrew/Cellar/...; rm-ing the symlink leaves Cellar
//     orphaned and `brew doctor` will complain.
//  2. Cellar-path substring. If os.Executable resolved through the
//     symlink (some shells do, some don't), we'd otherwise blindly
//     rm the Cellar copy and leave the symlink dangling.
func managerOwned(binPath string) string {
	if info, err := os.Lstat(binPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
		target, _ := os.Readlink(binPath)
		if target != "" {
			return fmt.Sprintf("%s is a symlink (→ %s); use the manager that created it (e.g. 'brew uninstall cliff').", binPath, target)
		}
		return fmt.Sprintf("%s is a symlink; use the manager that created it (e.g. 'brew uninstall cliff').", binPath)
	}
	if strings.Contains(binPath, "/Cellar/") || strings.Contains(binPath, "/homebrew/Cellar/") {
		return fmt.Sprintf("%s lives inside a Homebrew Cellar; use 'brew uninstall cliff' instead.", binPath)
	}
	return ""
}

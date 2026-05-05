// Package install runs install commands derived from manifest InstallSpec
// values and answers "is this app installed?" by inspecting $PATH plus a
// small set of well-known manager bin directories.
//
// The runner shells out via `sh -c` and captures stdout+stderr together.
// We intentionally don't try to sandbox or sanitize — the trust model
// (CLAUDE.md §3) is that installs run with the user's shell privileges,
// same as `brew install`. The confirm modal in the UI shows the exact
// command before it runs.
//
// Installed-state detection is derived from the filesystem at runtime
// rather than persisted to disk. We scan $PATH first, then a short list
// of manager-default bin dirs ($GOBIN, $GOPATH/bin, ~/go/bin,
// ~/.cargo/bin, ~/.local/bin). That second list matters because
// `go install` and `cargo install` drop binaries into directories that
// many users haven't added to $PATH, and without it cliff would show "not
// installed" immediately after a successful install.
//
// When a post-install Locate finds the binary only in one of those
// off-PATH dirs, Stream attaches a PathWarning to the Result so the UI
// can tell the user the install worked but their shell can't run it
// until they add the directory to $PATH.
package install

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/jmcntsh/cliff/internal/catalog"
)

// PathWarning describes the case where an install landed a binary in a
// known manager dir that isn't on the user's $PATH. The UI surfaces
// this as "install succeeded, but your shell can't find it yet — add
// this to your shell rc".
type PathWarning struct {
	Binary string // e.g. "tetrigo"
	Dir    string // absolute dir the binary lives in, e.g. "/Users/jmc/go/bin"
}

// Result is what Stream reports back when the install finishes.
type Result struct {
	App         *catalog.App
	Command     string
	ExitCode    int
	Output      string // combined stdout+stderr
	Err         error
	PathWarning *PathWarning // non-nil when install OK but binary isn't on $PATH
	// DetectedBinaries names executables produced by the install, if cliff
	// could infer them from output or bin-dir diffs.
	DetectedBinaries []string
}

// Stream runs the install command and invokes onLine for each line of
// combined stdout+stderr as it's produced. Blocks until the process
// exits or ctx is canceled (in which case the process is killed).
// Result.Output contains the full buffered output so callers can read
// it after completion without needing to have subscribed.
//
// onLine may be nil, in which case Stream just buffers the output.
// Callers in a TUI should invoke Stream off the main loop (inside a
// tea.Cmd) since it blocks for the duration of the install.
func Stream(ctx context.Context, app *catalog.App, onLine func(string)) Result {
	if app == nil || app.PrimaryInstallSpec() == nil {
		return Result{App: app, Err: errors.New("app has no install spec")}
	}
	return StreamCmd(ctx, app, app.PrimaryInstallSpec().Shell(), onLine)
}

// StreamCmd is Stream but for an arbitrary shell command — used by the
// uninstall and upgrade verbs, which derive their commands from
// InstallSpec.UninstallShell and .UpgradeShell. The app reference is
// retained on Result so Diagnose can still pattern-match on the install
// type when a missing-tool (exit 127) failure occurs.
func StreamCmd(ctx context.Context, app *catalog.App, cmd string, onLine func(string)) Result {
	res := Result{App: app}
	if cmd == "" {
		res.Err = errors.New("empty command")
		return res
	}
	res.Command = cmd

	// Brew is exempt: a single `brew install` drops the primary
	// package and deps into the same bin dir, so detection is too noisy.
	var preSnap map[string]struct{}
	var matchedSpec *catalog.InstallSpec
	if app != nil {
		for i := range app.InstallSpecs {
			if cmd == app.InstallSpecs[i].Shell() {
				matchedSpec = &app.InstallSpecs[i]
				break
			}
		}
	}
	isInstall := matchedSpec != nil
	detectable := isInstall && matchedSpec.Type != "brew"
	if detectable {
		preSnap = snapshotBinDirs()
	}

	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	var output bytes.Buffer
	lineR, lineW := io.Pipe()
	c.Stdout = io.MultiWriter(&output, lineW)
	c.Stderr = io.MultiWriter(&output, lineW)

	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		sc := bufio.NewScanner(lineR)
		// Some installs emit very long lines (progress bars, long URLs).
		// Bump max token size well above the default 64 KiB.
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			if onLine != nil {
				onLine(sc.Text())
			}
		}
		// If sc.Scan stopped early (bufio.ErrTooLong on a line > 1 MiB),
		// we MUST keep reading from the pipe. The command's stdout/stderr
		// go through io.MultiWriter(&output, lineW); a synchronous io.Pipe
		// with no reader would block those writes and deadlock c.Run.
		// Drain the rest so the full bytes still reach the output buffer.
		_, _ = io.Copy(io.Discard, lineR)
	}()

	err := c.Run()
	_ = lineW.Close()
	<-scanDone

	res.Output = output.String()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
		} else {
			res.ExitCode = -1
		}
		res.Err = err
		return res
	}
	res.ExitCode = 0

	if isInstall {
		var detected []string
		if detectable {
			detected = scrapeBinaries(matchedSpec.Type, res.Output)
			detected = appendUnique(detected, diffBinDirs(preSnap, snapshotBinDirs())...)
			res.DetectedBinaries = detected
		}

		// Prefer detected names when checking for off-PATH installs.
		warnBin := firstNonEmpty(detected...)
		if warnBin == "" {
			warnBin = app.BinaryName()
		}
		if warnBin != "" {
			if dir, onPath := LocateBinary(warnBin); dir != "" && !onPath {
				res.PathWarning = &PathWarning{Binary: warnBin, Dir: dir}
			}
		}
	}
	return res
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func appendUnique(into []string, more ...string) []string {
	seen := make(map[string]struct{}, len(into)+len(more))
	for _, s := range into {
		seen[s] = struct{}{}
	}
	for _, s := range more {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		into = append(into, s)
	}
	return into
}

// toolHints maps install types to missing-package-manager guidance.
var toolHints = map[string]string{
	"brew":  "Homebrew isn't installed.\nGet it at https://brew.sh",
	"cargo": "Cargo isn't installed.\nInstall Rust at https://rustup.rs",
	"go":    "Go isn't installed.\nInstall it at https://go.dev/dl/",
	"pipx":  "pipx isn't installed.\nInstall it at https://pipx.pypa.io/stable/installation/",
	"npm":   "npm isn't installed.\nInstall Node.js at https://nodejs.org",
}

// Diagnose turns recognized install failures into short hints.
func Diagnose(res Result) string {
	if res.Err == nil {
		return ""
	}
	if res.App != nil && res.App.PrimaryInstallSpec() != nil && res.ExitCode == 127 {
		if h, ok := toolHints[res.App.PrimaryInstallSpec().Type]; ok {
			return h
		}
	}
	for tool, h := range toolHints {
		// Word-boundary match keeps "go" from latching inside "cargo".
		if cmdNotFoundRes[tool].MatchString(res.Output) {
			return h
		}
	}
	return ""
}

// cmdNotFoundRes is one pre-compiled regex per tool name. Package-init
// so we compile once rather than on every Diagnose call.
var cmdNotFoundRes = func() map[string]*regexp.Regexp {
	out := make(map[string]*regexp.Regexp, len(toolHints))
	for tool := range toolHints {
		out[tool] = regexp.MustCompile(`\b` + regexp.QuoteMeta(tool) + `:\s*(command\s+)?not found`)
	}
	return out
}()

// Detect walks every directory on $PATH and returns a set of executable
// basenames found. Non-executable files and directories are skipped.
// This is the narrow "what can your shell run right now?" answer —
// callers that want to also recognize binaries sitting in manager
// default dirs (go install, cargo install) should use InstalledApps,
// which wraps Detect with a fallback scan.
func Detect() map[string]bool {
	out := map[string]bool{}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		addExecutables(out, dir)
	}
	return out
}

// addExecutables reads dir and OR-merges executable basenames into out.
// Non-executables, subdirs, and unreadable dirs are silently skipped so
// a bogus PATH entry can't break detection for the rest.
func addExecutables(out map[string]bool, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		// Any-execute bit set. On Windows this check would need
		// extension-based heuristics (.exe, .bat) — we accept the
		// Unix-only limitation consistent with the sh -c runner.
		if info.Mode()&0o111 == 0 {
			continue
		}
		out[e.Name()] = true
	}
}

// managerBinDirs returns common package-manager bin dirs beyond $PATH.
func managerBinDirs() []string {
	var dirs []string
	add := func(d string) {
		if d == "" {
			return
		}
		for _, existing := range dirs {
			if existing == d {
				return
			}
		}
		dirs = append(dirs, d)
	}

	home, _ := os.UserHomeDir()
	if gobin := os.Getenv("GOBIN"); gobin != "" {
		add(gobin)
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		// GOPATH can be colon-separated; only the first entry gets
		// `go install` output, matching cmd/go's behavior.
		for _, p := range filepath.SplitList(gopath) {
			if p != "" {
				add(filepath.Join(p, "bin"))
				break
			}
		}
	} else if home != "" {
		add(filepath.Join(home, "go", "bin"))
	}
	if home != "" {
		add(filepath.Join(home, ".cargo", "bin"))
		add(filepath.Join(home, ".local", "bin"))
	}
	_ = runtime.GOOS // reserved for future per-OS entries (e.g. %USERPROFILE%\go\bin on Windows)
	return dirs
}

// LocateBinary answers "where is this binary, and is that dir on $PATH?"
// for a single basename. It's how Stream decides whether to attach a
// PathWarning after a successful install. Returns "" if the binary
// isn't found in $PATH or any known manager dir.
func LocateBinary(name string) (dir string, onPath bool) {
	if name == "" {
		return "", false
	}
	// $PATH wins — if the binary is already runnable, no warning needed.
	if p, err := exec.LookPath(name); err == nil {
		return filepath.Dir(p), true
	}
	// Otherwise, look in the manager defaults. These are not on $PATH
	// (LookPath would have found it); any hit here means "install put
	// it here, but the shell can't run it".
	for _, d := range managerBinDirs() {
		candidate := filepath.Join(d, name)
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if info.Mode()&0o111 == 0 {
			continue
		}
		return d, false
	}
	return "", false
}

// InstalledApps returns apps whose binary is on PATH or in a manager bin dir.
func InstalledApps(apps []catalog.App) map[string]bool {
	bins := Detect()
	for _, d := range managerBinDirs() {
		addExecutables(bins, d)
	}
	out := map[string]bool{}
	for i := range apps {
		if bins[apps[i].BinaryName()] {
			out[apps[i].Repo] = true
		}
	}
	return out
}

// InstalledAppsWithOverrides applies learned repo→binary overrides.
func InstalledAppsWithOverrides(apps []catalog.App, overrides map[string]string) map[string]bool {
	bins := Detect()
	for _, d := range managerBinDirs() {
		addExecutables(bins, d)
	}
	out := map[string]bool{}
	for i := range apps {
		name := apps[i].BinaryName()
		if o, ok := overrides[apps[i].Repo]; ok && o != "" {
			name = o
		}
		if bins[name] {
			out[apps[i].Repo] = true
		}
	}
	return out
}

// snapshotBinDirs returns executable basenames across PATH and manager dirs.
func snapshotBinDirs() map[string]struct{} {
	out := map[string]struct{}{}
	dirs := filepath.SplitList(os.Getenv("PATH"))
	dirs = append(dirs, managerBinDirs()...)
	seen := map[string]struct{}{}
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil || info.Mode()&0o111 == 0 {
				continue
			}
			out[e.Name()] = struct{}{}
		}
	}
	return out
}

// diffBinDirs returns basenames present in after but not in before.
func diffBinDirs(before, after map[string]struct{}) []string {
	if before == nil {
		return nil
	}
	var out []string
	for name := range after {
		if _, ok := before[name]; !ok {
			out = append(out, name)
		}
	}
	return out
}

// scrapeBinaries extracts narrow, manager-specific executable names.
func scrapeBinaries(installType, output string) []string {
	var out []string
	switch installType {
	case "cargo":
		for _, m := range reCargoExecutable.FindAllStringSubmatch(output, -1) {
			out = appendUnique(out, m[1])
		}
		for _, m := range reCargoExecutables.FindAllStringSubmatch(output, -1) {
			for _, part := range strings.Split(m[1], ",") {
				name := strings.Trim(strings.TrimSpace(part), "'")
				out = appendUnique(out, name)
			}
		}
		for _, m := range reCargoReplacing.FindAllStringSubmatch(output, -1) {
			out = appendUnique(out, filepath.Base(m[1]))
		}
	case "pipx":
		// "These apps are now globally available:" followed by
		// "  - name" lines, one per binary. Consume until the first
		// non-matching line, so trailing pipx chatter doesn't get
		// slurped as a binary name.
		sc := bufio.NewScanner(strings.NewReader(output))
		inBlock := false
		for sc.Scan() {
			line := sc.Text()
			if strings.Contains(line, "These apps are now globally available") {
				inBlock = true
				continue
			}
			if inBlock {
				if m := rePipxBullet.FindStringSubmatch(line); m != nil {
					out = appendUnique(out, m[1])
					continue
				}
				if strings.TrimSpace(line) == "" {
					continue
				}
				inBlock = false
			}
		}
	}
	return out
}

var (
	reCargoExecutable  = regexp.MustCompile(`\(executable '([^']+)'\)`)
	reCargoExecutables = regexp.MustCompile(`\(executables ([^)]+)\)`)
	reCargoReplacing   = regexp.MustCompile(`Replacing\s+(\S+)`)
	rePipxBullet       = regexp.MustCompile(`^\s*-\s+(\S+)\s*$`)
)

package install

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmcntsh/cliff/internal/catalog"
)

func sampleApp() *catalog.App {
	return &catalog.App{
		Name: "demo",
		Repo: "u/demo",
		InstallSpecs: []catalog.InstallSpec{
			{Type: "script", Command: "echo hello"},
		},
	}
}

func TestStream_OK(t *testing.T) {
	res := Stream(context.Background(), sampleApp(), nil)
	if res.Err != nil {
		t.Fatalf("err: %v (output=%q)", res.Err, res.Output)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit = %d", res.ExitCode)
	}
	if res.Output != "hello\n" {
		t.Errorf("output = %q", res.Output)
	}
}

func TestStream_Failure(t *testing.T) {
	app := sampleApp()
	app.InstallSpecs[0].Command = "exit 7"
	res := Stream(context.Background(), app, nil)
	if res.Err == nil {
		t.Fatal("expected error")
	}
	if res.ExitCode != 7 {
		t.Errorf("exit = %d, want 7", res.ExitCode)
	}
}

func TestStream_NoSpec(t *testing.T) {
	res := Stream(context.Background(), &catalog.App{Name: "x"}, nil)
	if res.Err == nil {
		t.Fatal("expected error for missing spec")
	}
}

// TestStream_HandlesLineLongerThanScannerBuffer guards the drain added to
// Stream: when a command emits a single line larger than bufio.Scanner's
// 1 MiB token limit, the scanner bails with ErrTooLong. Without the drain,
// subsequent writes through io.MultiWriter → io.Pipe would block forever
// and c.Run would deadlock. This test fails with a timeout if that
// regression returns.
func TestStream_HandlesLineLongerThanScannerBuffer(t *testing.T) {
	app := sampleApp()
	// 2 MB of 'x' on one line (well over the 1 MiB scanner limit),
	// then a newline, then "done". /dev/zero + tr is portable across
	// macOS and Linux.
	app.InstallSpecs[0].Command = "head -c 2000000 /dev/zero | tr '\\0' 'x'; printf '\\n'; echo done"

	resCh := make(chan Result, 1)
	go func() {
		resCh <- Stream(context.Background(), app, nil)
	}()
	select {
	case res := <-resCh:
		if res.Err != nil {
			t.Fatalf("err: %v", res.Err)
		}
		if !strings.HasSuffix(strings.TrimRight(res.Output, "\n"), "done") {
			t.Errorf("expected output to end with 'done', got suffix %q (len=%d)",
				res.Output[max(0, len(res.Output)-20):], len(res.Output))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Stream hung — pipe drain regression")
	}
}

func TestStream_CallsOnLine(t *testing.T) {
	app := sampleApp()
	app.InstallSpecs[0].Command = "printf 'one\\ntwo\\nthree\\n'"
	var got []string
	res := Stream(context.Background(), app, func(line string) {
		got = append(got, line)
	})
	if res.Err != nil {
		t.Fatalf("err: %v", res.Err)
	}
	if len(got) != 3 || got[0] != "one" || got[1] != "two" || got[2] != "three" {
		t.Errorf("lines = %v", got)
	}
}

// TestDetect_FindsExecutables drops a sentinel executable into a temp
// dir, prepends it to PATH, and verifies Detect picks it up while
// skipping a non-executable sibling.
func TestDetect_FindsExecutables(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "cliff-test-exe-abc123")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	notExe := filepath.Join(dir, "cliff-test-notexe-abc123")
	if err := os.WriteFile(notExe, []byte("just a file"), 0o644); err != nil {
		t.Fatal(err)
	}

	orig := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+orig)

	got := Detect()
	if !got["cliff-test-exe-abc123"] {
		t.Error("expected executable to be detected")
	}
	if got["cliff-test-notexe-abc123"] {
		t.Error("non-executable should not be detected")
	}
}

func TestDiagnose_CommandNotFoundByExitCode(t *testing.T) {
	res := Result{
		App: &catalog.App{
			InstallSpecs: []catalog.InstallSpec{{Type: "brew", Package: "foo"}},
		},
		ExitCode: 127,
		Err:      context.DeadlineExceeded, // any non-nil err
		Output:   "sh: brew: command not found\n",
	}
	got := Diagnose(res)
	if got == "" {
		t.Fatal("expected a hint for brew not installed")
	}
	if !strings.Contains(got, "brew.sh") {
		t.Errorf("hint should mention brew.sh, got %q", got)
	}
}

func TestDiagnose_CommandNotFoundByOutputScrape(t *testing.T) {
	res := Result{
		// No InstallSpec — force the fallback path to kick in
		ExitCode: 1,
		Err:      context.DeadlineExceeded,
		Output:   "/bin/sh: cargo: command not found\n",
	}
	got := Diagnose(res)
	if got == "" {
		t.Fatal("expected hint from output scrape")
	}
	if !strings.Contains(got, "rustup.rs") {
		t.Errorf("hint should mention rustup.rs, got %q", got)
	}
}

func TestDiagnose_NoHintOnSuccess(t *testing.T) {
	if got := Diagnose(Result{ExitCode: 0}); got != "" {
		t.Errorf("successful install shouldn't produce a hint, got %q", got)
	}
}

func TestDiagnose_NoHintOnUnknownFailure(t *testing.T) {
	res := Result{
		ExitCode: 2,
		Err:      context.DeadlineExceeded,
		Output:   "some unrelated error\n",
	}
	if got := Diagnose(res); got != "" {
		t.Errorf("unknown failure shouldn't produce a hint, got %q", got)
	}
}

func TestInstalledApps_MatchesByRepoBasename(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "glow")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	// Isolate manager-dir scan: point the defaults at an empty temp
	// dir so a real ~/.cargo/bin or ~/go/bin on the test host can't
	// leak a false positive for "lazygit" and pass the negative check.
	isolateManagerDirs(t)

	apps := []catalog.App{
		{Name: "glow", Repo: "charmbracelet/glow"},
		{Name: "lazygit", Repo: "jesseduffield/lazygit"},
	}
	got := InstalledApps(apps)
	if !got["charmbracelet/glow"] {
		t.Error("expected glow to be detected via PATH")
	}
	if got["jesseduffield/lazygit"] {
		t.Error("lazygit binary does not exist in PATH; should not be marked")
	}
}

// TestInstalledApps_DetectsBinaryInGOBIN_OffPATH pins the post-v0.1.6
// behavior: a binary dropped in $GOBIN counts as installed even when
// $GOBIN isn't on $PATH. This is the common post-`go install` state on
// a fresh machine and was previously reported as "not installed".
func TestInstalledApps_DetectsBinaryInGOBIN_OffPATH(t *testing.T) {
	pathDir := t.TempDir()
	gobin := t.TempDir()
	exe := filepath.Join(gobin, "tetrigo")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pathDir) // deliberately no gobin in PATH
	isolateManagerDirs(t)
	t.Setenv("GOBIN", gobin)

	got := InstalledApps([]catalog.App{
		{Name: "tetrigo", Repo: "Broderick-Westrope/tetrigo"},
	})
	if !got["Broderick-Westrope/tetrigo"] {
		t.Error("tetrigo in $GOBIN should count as installed even off PATH")
	}
}

func TestLocateBinary_OnPATH(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "cliff-locate-a")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	isolateManagerDirs(t)

	got, onPath := LocateBinary("cliff-locate-a")
	if !onPath {
		t.Errorf("expected onPath=true for binary in $PATH, got dir=%q onPath=%v", got, onPath)
	}
}

func TestLocateBinary_InGOBIN_OffPATH(t *testing.T) {
	pathDir := t.TempDir()
	gobin := t.TempDir()
	exe := filepath.Join(gobin, "cliff-locate-b")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pathDir)
	isolateManagerDirs(t)
	t.Setenv("GOBIN", gobin)

	dir, onPath := LocateBinary("cliff-locate-b")
	if onPath {
		t.Errorf("expected onPath=false for binary only in $GOBIN, got dir=%q onPath=%v", dir, onPath)
	}
	if dir != gobin {
		t.Errorf("expected dir=%q, got %q", gobin, dir)
	}
}

func TestLocateBinary_NotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	isolateManagerDirs(t)

	dir, onPath := LocateBinary("cliff-locate-nonesuch")
	if dir != "" || onPath {
		t.Errorf("expected empty miss, got dir=%q onPath=%v", dir, onPath)
	}
}

// TestStream_AttachesPathWarning exercises the end-to-end post-install
// check: a successful install whose produced binary lives only in a
// manager dir (not $PATH) must emit a PathWarning on the Result.
func TestStream_AttachesPathWarning(t *testing.T) {
	pathDir := t.TempDir()
	gobin := t.TempDir()
	// /bin and /usr/bin so sh + printf + chmod resolve inside sh -c.
	// If those weren't on PATH, exec would fail before we ever got to
	// the post-install check we're trying to exercise.
	t.Setenv("PATH", pathDir+":/bin:/usr/bin")
	isolateManagerDirs(t)
	t.Setenv("GOBIN", gobin)

	// Script install that drops an executable into $GOBIN, simulating
	// what `go install foo@latest` would do. Using a script type keeps
	// the test hermetic — we don't need Go installed on the test host.
	// printf + chmod is portable across /bin/sh (macOS + Linux).
	bin := filepath.Join(gobin, "phantom")
	app := &catalog.App{
		Name: "phantom",
		Repo: "u/phantom",
		InstallSpecs: []catalog.InstallSpec{{
			Type:    "script",
			Command: `printf '#!/bin/sh\nexit 0\n' > ` + bin + ` && chmod +x ` + bin,
		}},
	}

	res := Stream(context.Background(), app, nil)
	if res.Err != nil {
		t.Fatalf("install failed: %v (output=%q)", res.Err, res.Output)
	}
	if res.PathWarning == nil {
		t.Fatal("expected PathWarning for off-PATH install, got nil")
	}
	if res.PathWarning.Binary != "phantom" {
		t.Errorf("warning.Binary = %q, want %q", res.PathWarning.Binary, "phantom")
	}
	if res.PathWarning.Dir != gobin {
		t.Errorf("warning.Dir = %q, want %q", res.PathWarning.Dir, gobin)
	}
}

// TestStream_DetectsBinaryViaDirDiff covers the cross-installer
// fallback path: an install that drops a brand-new executable into
// a manager bin dir (here: $GOBIN) should be detected even when its
// output is mute — the dir-diff catches what output-scraping cannot.
// This is the exact case that broke for minesweep (cargo package
// minesweep vs. repo basename minesweep-rs) before we added
// detection.
func TestStream_DetectsBinaryViaDirDiff(t *testing.T) {
	pathDir := t.TempDir()
	gobin := t.TempDir()
	t.Setenv("PATH", pathDir+":/bin:/usr/bin")
	isolateManagerDirs(t)
	t.Setenv("GOBIN", gobin)

	app := &catalog.App{
		Name: "phantom",
		Repo: "u/phantom-extra-suffix",
		InstallSpecs: []catalog.InstallSpec{{
			Type:    "script",
			Command: `printf '#!/bin/sh\nexit 0\n' > ` + filepath.Join(gobin, "phantom") + ` && chmod +x ` + filepath.Join(gobin, "phantom"),
		}},
	}
	res := Stream(context.Background(), app, nil)
	if res.Err != nil {
		t.Fatalf("install failed: %v", res.Err)
	}
	if len(res.DetectedBinaries) == 0 {
		t.Fatal("expected DetectedBinaries to be populated via dir-diff, got empty")
	}
	found := false
	for _, b := range res.DetectedBinaries {
		if b == "phantom" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'phantom' in DetectedBinaries, got %v", res.DetectedBinaries)
	}
}

// TestStream_EmptyDetectedOnMute covers the negative case: an
// install that produces no new binary (e.g. the file already existed
// in the manager dir and the installer overwrote it silently)
// leaves DetectedBinaries empty rather than inventing a false name.
// Empty lets ResolvedBinaryName fall through to the manifest-derived
// guess, which is the right behavior.
func TestStream_EmptyDetectedOnMute(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir+":/bin:/usr/bin")
	isolateManagerDirs(t)

	app := &catalog.App{
		Name: "silent",
		Repo: "u/silent",
		InstallSpecs: []catalog.InstallSpec{{
			Type:    "script",
			Command: "echo doing nothing",
		}},
	}
	res := Stream(context.Background(), app, nil)
	if res.Err != nil {
		t.Fatalf("install failed: %v", res.Err)
	}
	if len(res.DetectedBinaries) != 0 {
		t.Errorf("expected empty DetectedBinaries, got %v", res.DetectedBinaries)
	}
}

// TestScrapeBinaries_Cargo pins the regex against real cargo output.
// The formats tested here are the ones stable across cargo versions
// circa 1.80+; a silent break in cargo's output would fail this test
// and prompt us to update the pattern rather than silently drop to
// the dir-diff fallback (which still works but is coarser).
func TestScrapeBinaries_Cargo(t *testing.T) {
	output := `    Updating crates.io index
  Downloaded minesweep v0.5.7
   Compiling minesweep v0.5.7
    Finished release [optimized] target(s) in 4.2s
  Installing /Users/u/.cargo/bin/minesweep
   Installed package ` + "`minesweep v0.5.7`" + ` (executable 'minesweep')
`
	got := scrapeBinaries("cargo", output)
	if len(got) == 0 || got[0] != "minesweep" {
		t.Errorf("cargo scrape = %v, want [minesweep]", got)
	}
}

// TestScrapeBinaries_CargoMulti covers multi-binary crates, which
// cargo reports with the plural (executables 'a', 'b') form.
func TestScrapeBinaries_CargoMulti(t *testing.T) {
	output := `   Installed package ` + "`tool v1.0.0`" + ` (executables 'tool-a', 'tool-b')`
	got := scrapeBinaries("cargo", output)
	if len(got) != 2 || got[0] != "tool-a" || got[1] != "tool-b" {
		t.Errorf("cargo multi scrape = %v, want [tool-a tool-b]", got)
	}
}

// TestScrapeBinaries_Pipx covers the block-consume logic — only
// the hyphen-bullets immediately under "These apps are now globally
// available" should be picked up, not every dashed line in the output.
func TestScrapeBinaries_Pipx(t *testing.T) {
	output := `installed package httpie 3.2.1, installed using Python 3.11.5
These apps are now globally available:
  - http
  - https
  - httpie
done! ✨ 🌟 ✨
`
	got := scrapeBinaries("pipx", output)
	if len(got) != 3 || got[0] != "http" || got[1] != "https" || got[2] != "httpie" {
		t.Errorf("pipx scrape = %v, want [http https httpie]", got)
	}
}

// TestStream_NoPathWarningWhenOnPATH is the negative case: when the
// install drops the binary into a dir already on $PATH, no warning
// should fire.
func TestStream_NoPathWarningWhenOnPATH(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir+":/bin:/usr/bin")
	isolateManagerDirs(t)

	app := &catalog.App{
		Name: "ghost",
		Repo: "u/ghost",
		InstallSpecs: []catalog.InstallSpec{{
			Type: "script",
			Command: `printf '#!/bin/sh\nexit 0\n' > ` +
				filepath.Join(dir, "ghost") + ` && chmod +x ` + filepath.Join(dir, "ghost"),
		}},
	}
	res := Stream(context.Background(), app, nil)
	if res.Err != nil {
		t.Fatalf("install failed: %v (output=%q)", res.Err, res.Output)
	}
	if res.PathWarning != nil {
		t.Errorf("unexpected PathWarning for on-PATH install: %+v", *res.PathWarning)
	}
}

// isolateManagerDirs points GOBIN/GOPATH/HOME at empty temp dirs for
// the duration of the test so LocateBinary and managerBinDirs can't
// pick up real binaries from the developer's ~/go/bin or ~/.cargo/bin.
// Without this, a test host that has `tetrigo` installed via `go
// install` would make the "not found" tests flake.
func isolateManagerDirs(t *testing.T) {
	t.Helper()
	emptyHome := t.TempDir()
	emptyGopath := t.TempDir()
	t.Setenv("HOME", emptyHome)
	t.Setenv("GOPATH", emptyGopath)
	t.Setenv("GOBIN", "") // cleared unless the caller re-sets it
}

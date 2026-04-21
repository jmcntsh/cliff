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
		InstallSpec: &catalog.InstallSpec{
			Type: "script", Command: "echo hello",
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
	app.InstallSpec.Command = "exit 7"
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
	app.InstallSpec.Command = "head -c 2000000 /dev/zero | tr '\\0' 'x'; printf '\\n'; echo done"

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
	app.InstallSpec.Command = "printf 'one\\ntwo\\nthree\\n'"
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
			InstallSpec: &catalog.InstallSpec{Type: "brew", Package: "foo"},
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

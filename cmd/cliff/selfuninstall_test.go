package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// withTempHome points $HOME at a fresh temp dir so cliffDataDir() and
// installStatePath() resolve under the test's private filesystem.
func withTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

// writeInstallState is the test-side counterpart of install.sh's
// write_install_state. Keeping the JSON shape inline (rather than
// using encoding/json on installState) deliberately mirrors the
// shell-emitted file, so a future drift between install.sh's printf
// and the Go struct shape is what this test would catch.
func writeInstallState(t *testing.T, home, dir, method, version string) {
	t.Helper()
	stateDir := filepath.Join(home, ".cliff")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	body := `{"install_dir":"` + dir + `","install_method":"` + method + `","version":"` + version + `"}` + "\n"
	if err := os.WriteFile(filepath.Join(stateDir, "install.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("write install.json: %v", err)
	}
}

// touchExecutable creates a fake cliff binary at path with mode 0755.
// Used to give resolveSelfBinary something to stat-and-return.
func touchExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
}

// TestLoadInstallState_MissingFileReturnsNilNil pins the contract
// that "no install.json yet" is a normal first-call case, not an
// error — self-uninstall must fall through to os.Executable, not
// abort.
func TestLoadInstallState_MissingFileReturnsNilNil(t *testing.T) {
	withTempHome(t)
	s, err := loadInstallState()
	if err != nil {
		t.Fatalf("missing file should be (nil, nil), got err=%v", err)
	}
	if s != nil {
		t.Errorf("missing file should be (nil, nil), got s=%+v", s)
	}
}

func TestLoadInstallState_RoundTrip(t *testing.T) {
	home := withTempHome(t)
	writeInstallState(t, home, "/opt/homebrew/bin", "script", "v0.1.19")

	s, err := loadInstallState()
	if err != nil {
		t.Fatalf("loadInstallState: %v", err)
	}
	if s == nil {
		t.Fatal("loadInstallState returned nil state")
	}
	if s.InstallDir != "/opt/homebrew/bin" {
		t.Errorf("InstallDir = %q, want /opt/homebrew/bin", s.InstallDir)
	}
	if s.InstallMethod != "script" {
		t.Errorf("InstallMethod = %q, want script", s.InstallMethod)
	}
	if s.Version != "v0.1.19" {
		t.Errorf("Version = %q, want v0.1.19", s.Version)
	}
}

// TestResolveSelfBinary_PrefersInstallStateWhenExtant verifies the
// happy path: install.json points at a real file, resolveSelfBinary
// hands back that exact path. This is the scalability win — exact
// because install.sh recorded it, not heuristic.
func TestResolveSelfBinary_PrefersInstallStateWhenExtant(t *testing.T) {
	home := withTempHome(t)
	binDir := filepath.Join(home, "fake-bin")
	binPath := filepath.Join(binDir, "cliff")
	touchExecutable(t, binPath)
	writeInstallState(t, home, binDir, "script", "v0.1.19")

	got, source, err := resolveSelfBinary()
	if err != nil {
		t.Fatalf("resolveSelfBinary: %v", err)
	}
	if got != binPath {
		t.Errorf("path = %q, want %q", got, binPath)
	}
	if source != "~/.cliff/install.json" {
		t.Errorf("source = %q, want ~/.cliff/install.json", source)
	}
}

// TestResolveSelfBinary_FallsBackWhenInstallStateStale guards the
// "install.json points at a path that no longer exists" case — e.g.
// the user manually moved the binary. We don't want to fail outright;
// we want to fall back to os.Executable so the user can still
// uninstall the copy they're actually running.
func TestResolveSelfBinary_FallsBackWhenInstallStateStale(t *testing.T) {
	home := withTempHome(t)
	writeInstallState(t, home, filepath.Join(home, "does-not-exist"), "script", "v0.1.19")

	_, source, err := resolveSelfBinary()
	if err != nil {
		t.Fatalf("resolveSelfBinary: %v", err)
	}
	if source != "os.Executable()" {
		t.Errorf("source = %q, want os.Executable()", source)
	}
}

// TestResolveSelfBinary_FallsBackWhenNoInstallState covers the
// brew-tap / go-install user — no install.json was ever written, so
// we use the running binary's path.
func TestResolveSelfBinary_FallsBackWhenNoInstallState(t *testing.T) {
	withTempHome(t)
	_, source, err := resolveSelfBinary()
	if err != nil {
		t.Fatalf("resolveSelfBinary: %v", err)
	}
	if source != "os.Executable()" {
		t.Errorf("source = %q, want os.Executable()", source)
	}
}

// TestManagerOwned_SymlinkRefused is the brew-symlink guard: rm-ing
// /opt/homebrew/bin/cliff when it's a symlink into Cellar would
// orphan Cellar contents. managerOwned returns a non-empty refusal.
func TestManagerOwned_SymlinkRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on Windows; cliff doesn't ship there")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "real-cliff")
	link := filepath.Join(dir, "cliff-link")
	touchExecutable(t, target)
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if reason := managerOwned(link); reason == "" {
		t.Errorf("expected refusal for symlink, got empty reason")
	}
}

// TestManagerOwned_CellarPathRefused covers the case where
// os.Executable resolved through the symlink and handed us the
// Cellar copy directly. Substring check on /Cellar/ catches this.
func TestManagerOwned_CellarPathRefused(t *testing.T) {
	cases := []string{
		"/opt/homebrew/Cellar/cliff/0.1.18/bin/cliff",
		"/usr/local/Cellar/cliff/0.1.18/bin/cliff",
	}
	for _, p := range cases {
		if reason := managerOwned(p); reason == "" {
			t.Errorf("expected refusal for Cellar path %q, got empty reason", p)
		}
	}
}

// TestManagerOwned_RegularFileAllowed ensures a plain
// non-symlink, non-Cellar file passes — that's the
// curl-cliff.sh-|-sh install case we want to actually remove.
func TestManagerOwned_RegularFileAllowed(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "cliff")
	touchExecutable(t, bin)
	if reason := managerOwned(bin); reason != "" {
		t.Errorf("expected regular file to be allowed, got refusal: %s", reason)
	}
}

// TestManagerOwned_NonExistentFileAllowed covers the "binary already
// gone" case (e.g. user manually rm'd it before running
// self-uninstall). We don't want to refuse just because the file's
// missing — the os.Remove later is harmless on a missing path.
func TestManagerOwned_NonExistentFileAllowed(t *testing.T) {
	if reason := managerOwned("/does/not/exist/cliff"); reason != "" {
		t.Errorf("expected non-existent path to be allowed (no symlink, no Cellar), got refusal: %s", reason)
	}
}

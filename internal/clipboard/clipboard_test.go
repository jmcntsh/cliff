package clipboard

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestNativeTool_Darwin confirms macOS picks pbcopy when it exists on
// $PATH. pbcopy is always on $PATH on macOS (it's in /usr/bin), so the
// test doesn't need to fake it — we just verify the branch.
func TestNativeTool_Darwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only check")
	}
	name, args, ok := nativeTool()
	if !ok {
		t.Fatal("expected nativeTool to detect pbcopy on darwin")
	}
	if name != "pbcopy" {
		t.Errorf("got %q, want pbcopy", name)
	}
	if len(args) != 0 {
		t.Errorf("got args %v, want none", args)
	}
}

// TestNativeTool_EmptyPATH verifies the fallback path kicks in when
// no native tool is findable. Without this guard, a user in a Docker
// container with no pbcopy/xclip/wl-copy could get a silent
// "nativeTool: ok=true" with a bogus exec target.
func TestNativeTool_EmptyPATH(t *testing.T) {
	// Point PATH at an empty dir so LookPath can't find anything.
	empty := t.TempDir()
	t.Setenv("PATH", empty)
	// Kill the Wayland hint too so the linux branch doesn't try wl-copy
	// via some other mechanism.
	t.Setenv("WAYLAND_DISPLAY", "")

	if _, _, ok := nativeTool(); ok {
		t.Error("expected nativeTool to return ok=false when PATH is empty")
	}
}

// TestWrite_ViaFakePbcopy proves Write actually executes the native
// tool and sends text to its stdin. We build a tiny shell stub that
// writes stdin to a file, point PATH at its directory under the name
// pbcopy/xclip/wl-copy (whichever the current GOOS branch will
// select), and verify the file contents.
func TestWrite_ViaFakeNativeTool(t *testing.T) {
	// Pick the binary name this platform's nativeTool() would choose.
	// Keep it narrow: only the common three.
	var fakeName string
	switch runtime.GOOS {
	case "darwin":
		fakeName = "pbcopy"
	case "linux", "freebsd", "openbsd", "netbsd":
		// Force the xclip branch by clearing WAYLAND_DISPLAY. xclip's
		// args are non-trivial, which is a better smoke test than
		// wl-copy (no args).
		t.Setenv("WAYLAND_DISPLAY", "")
		fakeName = "xclip"
	default:
		t.Skipf("no native-tool branch for GOOS=%s", runtime.GOOS)
	}

	fakeDir := t.TempDir()
	out := filepath.Join(fakeDir, "captured.txt")
	// The stub writes *all of stdin* to $CLIPBOARD_TEST_OUT. We wire
	// the path via env so the stub is self-contained and doesn't need
	// argument parsing (which would make it accidentally work when
	// called with e.g. xclip's flags).
	stubPath := filepath.Join(fakeDir, fakeName)
	stub := "#!/bin/sh\ncat > \"$CLIPBOARD_TEST_OUT\"\n"
	if err := os.WriteFile(stubPath, []byte(stub), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	// Prepend the stub dir to PATH, keeping /bin:/usr/bin so "sh" and
	// "cat" resolve. Without the latter two, the stub itself can't
	// run, which would mask real bugs.
	t.Setenv("PATH", fakeDir+":/bin:/usr/bin")
	t.Setenv("CLIPBOARD_TEST_OUT", out)

	if err := Write("hello clipboard"); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading captured file: %v", err)
	}
	if string(got) != "hello clipboard" {
		t.Errorf("got %q, want %q", string(got), "hello clipboard")
	}
}

// TestWrite_FallsBackToOSC52WhenNoNativeTool confirms that Write
// returns nil (OSC52's sentinel success) when no native tool is
// available and OSC52 writes cleanly to stderr. We don't inspect the
// escape bytes — a separate test could, but the goal here is just to
// prove we don't error out when the native path is absent.
func TestWrite_FallsBackToOSC52WhenNoNativeTool(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("WAYLAND_DISPLAY", "")

	if err := Write("no native tool here"); err != nil {
		t.Errorf("expected OSC52 fallback to succeed, got error: %v", err)
	}
}

// TestWriteOSC52_LegacyEntryPoint pins that the old public name still
// works and swallows errors. Existing callers rely on fire-and-forget
// semantics, so switching them to the new Write in the same commit
// would be a larger blast radius — the wrapper exists to keep that
// change deferrable.
func TestWriteOSC52_LegacyEntryPoint(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("WriteOSC52 panicked: %v", r)
		}
	}()
	WriteOSC52("legacy call")
}

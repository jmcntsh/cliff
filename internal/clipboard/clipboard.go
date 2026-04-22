// Package clipboard writes text to the user's clipboard using whichever
// mechanism is most likely to actually work in the current environment.
//
// The dispatch order is:
//
//  1. A native OS clipboard tool when one is on $PATH:
//     - macOS: pbcopy
//     - Wayland: wl-copy (detected by $WAYLAND_DISPLAY)
//     - X11: xclip (then xsel as a second try)
//
//  2. OSC52 escape sequence as a fallback. This works inside tmux and
//     most modern terminal emulators *that have it enabled*, but is a
//     silent no-op in some defaults (notably macOS Terminal.app unless
//     "Allow terminal applications to set the clipboard" is checked).
//     We try it last precisely because of that silent-failure mode.
//
// Write returns an error when neither path succeeded, so callers can
// decide whether to show a toast ("copied") or an honest fallback
// message ("copy failed; here's the command to run: ..."). The old
// WriteOSC52 name is preserved as a fire-and-forget helper for the
// existing callsites, which don't yet surface copy failures to users;
// it now delegates to Write and only emits raw OSC52 as a last resort.
package clipboard

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// Write copies text to the system clipboard. Returns nil on success.
// On failure, the returned error describes the last mechanism tried;
// callers can surface it or ignore it depending on how important the
// copy is.
func Write(text string) error {
	if cmd, args, ok := nativeTool(); ok {
		if err := runCopy(text, cmd, args...); err == nil {
			return nil
		}
		// Fall through to OSC52 on native-tool failure (unusual: the
		// binary exists but errored). Better than giving up silently.
	}
	return writeOSC52(text)
}

// WriteOSC52 is the legacy fire-and-forget entry point used by
// existing callers. Kept to avoid touching every callsite in one
// commit; it now uses the same native-first strategy as Write and
// just swallows the error. New code should prefer Write and decide
// what to do when copy fails.
func WriteOSC52(text string) {
	_ = Write(text)
}

// nativeTool picks the best OS clipboard command for the current
// environment. Returns (program, args, true) when one is available on
// $PATH. The returned args are the flags that put the tool in
// "read from stdin, write to clipboard" mode; we then pipe text in.
//
// Detection is cheap (exec.LookPath only) so we call it per Write
// rather than caching — the user's $PATH can change mid-session and
// there's no reason to pin it.
func nativeTool() (name string, args []string, ok bool) {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("pbcopy"); err == nil {
			return "pbcopy", nil, true
		}
	case "linux", "freebsd", "openbsd", "netbsd":
		// Prefer wl-copy under Wayland. Falling back to xclip/xsel is
		// fine on Wayland too (via XWayland) but wl-copy is the native
		// path and avoids an extra process layer.
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			if _, err := exec.LookPath("wl-copy"); err == nil {
				return "wl-copy", nil, true
			}
		}
		if _, err := exec.LookPath("xclip"); err == nil {
			// -selection clipboard hits the "real" clipboard that
			// Cmd+V / Ctrl+V reads; the default is PRIMARY (middle-
			// click paste), which is almost never what the user wants
			// from a "copied" toast.
			return "xclip", []string{"-selection", "clipboard"}, true
		}
		if _, err := exec.LookPath("xsel"); err == nil {
			return "xsel", []string{"--clipboard", "--input"}, true
		}
	}
	return "", nil, false
}

// runCopy executes `name args...` and writes text to its stdin. This
// is the standard idiom for every clipboard CLI on the supported OSes:
// they read from stdin until EOF and put whatever they got on the
// clipboard. We attach a pipe rather than using CombinedOutput because
// these tools don't print anything useful on success and we don't want
// to hold onto their stderr unless they fail.
func runCopy(text, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("%s: stdin pipe: %w", name, err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: start: %w", name, err)
	}
	if _, err := stdin.Write([]byte(text)); err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return fmt.Errorf("%s: write: %w", name, err)
	}
	if err := stdin.Close(); err != nil {
		_ = cmd.Wait()
		return fmt.Errorf("%s: close stdin: %w", name, err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%s: exit: %w", name, err)
	}
	return nil
}

// writeOSC52 emits the OSC52 escape sequence that asks the terminal
// emulator to put text on the clipboard. Works in a lot of modern
// terminals (and importantly, tmux passes it through when configured
// to); silently ignored in others. We emit on stderr because cliff's
// TUI is drawing to stdout and we don't want to interfere with
// Bubble Tea's framebuffer — the escape goes direct to the tty.
//
// Returns nil unconditionally: OSC52 has no reply channel, so we
// can't know whether the terminal honored it. Callers should treat
// this as "probably worked but no guarantees."
func writeOSC52(text string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	_, err := fmt.Fprintf(os.Stderr, "\x1b]52;c;%s\x07", encoded)
	return err
}

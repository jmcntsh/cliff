// Package launcher opens a new terminal tab running a given command,
// when the host terminal exposes a mechanism for doing so.
//
// The post-install UX we want is:
//
//	✓ Installed tetrigo
//	  ⏎ open in new tab
//
// Pressing ⏎ should leave cliff running (it's a TUI occupying the
// current pane) and launch the just-installed app next door. That's
// only possible because a handful of terminals expose programmatic
// tab-spawning:
//
//   - tmux (any terminal) — tmux new-window <cmd>
//   - WezTerm             — wezterm cli spawn -- <cmd>
//   - Kitty (if remote control enabled) — kitten @ launch --type=tab <cmd>
//   - iTerm2 (macOS)      — osascript against current window
//
// Everything else (Terminal.app, Alacritty, Ghostty, GNOME Terminal,
// Konsole, vscode, …) either has no programmatic tab API or requires
// invasive accessibility permissions we refuse to ask for. For those,
// Detect returns Method=Unsupported and the UI falls back to "copy
// command" rather than pretending we can do something we can't.
//
// The mechanisms are tried in this priority order:
//
//  1. tmux — if $TMUX is set, tmux is the "tab" the user actually
//     sees, regardless of which GUI terminal hosts it.
//  2. WezTerm — first-class CLI (wezterm cli spawn), no setup needed.
//  3. Kitty — CLI exists but only works if allow_remote_control is
//     enabled; we check $KITTY_LISTEN_ON before claiming support.
//  4. iTerm2 — AppleScript is deprecated but still works; macOS only.
package launcher

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Method identifies which mechanism Detect found for opening a new tab.
type Method int

const (
	// MethodUnsupported is returned when no usable tab-spawn mechanism
	// is available in the current environment. Callers should render
	// the "copy command" fallback rather than a "open in new tab"
	// affordance.
	MethodUnsupported Method = iota
	MethodTmux
	MethodWezTerm
	MethodKitty
	MethodITerm2
)

func (m Method) String() string {
	switch m {
	case MethodTmux:
		return "tmux"
	case MethodWezTerm:
		return "WezTerm"
	case MethodKitty:
		return "Kitty"
	case MethodITerm2:
		return "iTerm2"
	default:
		return "unsupported"
	}
}

// Env snapshots the environment variables and OS that Detect inspects.
// Extracted so tests can drive detection deterministically without
// clobbering the real process env.
type Env struct {
	// Tmux is $TMUX (empty outside tmux).
	Tmux string
	// WezTermSocket is $WEZTERM_UNIX_SOCKET. Only set when running
	// inside a wezterm pane.
	WezTermSocket string
	// KittyListen is $KITTY_LISTEN_ON. Set when kitty is started with
	// remote control enabled; we refuse to claim Kitty support when
	// this is empty even if $TERM_PROGRAM=kitty, because kitten @ will
	// error without a listener.
	KittyListen string
	// TermProgram is $TERM_PROGRAM. Used as the last-resort signal for
	// iTerm2 (where the AppleScript path kicks in).
	TermProgram string
	// GOOS is runtime.GOOS. iTerm2's osascript path is macOS-only.
	GOOS string
}

// CurrentEnv reads the real process environment. Production callers
// should use this; tests use NewEnv with explicit values.
func CurrentEnv() Env {
	return Env{
		Tmux:          os.Getenv("TMUX"),
		WezTermSocket: os.Getenv("WEZTERM_UNIX_SOCKET"),
		KittyListen:   os.Getenv("KITTY_LISTEN_ON"),
		TermProgram:   os.Getenv("TERM_PROGRAM"),
		GOOS:          runtime.GOOS,
	}
}

// Detect returns the preferred tab-spawn method for the given env.
// MethodUnsupported means none of the mechanisms we trust are usable
// here; the caller should render a copy-command affordance instead of
// a launch affordance.
//
// We do not probe for executables here (no exec.LookPath). Detect is
// pure env-driven and cheap enough to run on every render; the actual
// exec error, if the binary is missing despite the env hint, surfaces
// from Launch.
func Detect(env Env) Method {
	if env.Tmux != "" {
		return MethodTmux
	}
	if env.WezTermSocket != "" || env.TermProgram == "WezTerm" {
		return MethodWezTerm
	}
	// $KITTY_LISTEN_ON means the user both runs kitty AND has remote
	// control enabled. $TERM_PROGRAM=kitty alone isn't enough — kitten
	// @ launch will fail if remote control wasn't turned on. Better
	// to fall through to unsupported than to promise a tab and error.
	if env.KittyListen != "" {
		return MethodKitty
	}
	if env.GOOS == "darwin" && env.TermProgram == "iTerm.app" {
		return MethodITerm2
	}
	return MethodUnsupported
}

// Launch runs command in a new tab using the given method. Returns the
// method's string identifier on success and an error on failure.
//
// command is a single shell line; we do not parse it. Each backend
// either passes it through a shell (iTerm2 via AppleScript, tmux via
// new-window with a -c "sh -c ..." style invocation) or runs it as a
// program-plus-args list (wezterm, kitty). To get consistent behavior
// across backends we always hand the string to "/bin/sh -c" so quoting
// and shell features work identically everywhere.
//
// A 5s hard timeout is imposed on the *spawn* call itself (not on the
// spawned app). This catches the case where kitten/wezterm hang
// talking to a stale socket — we'd rather return an error than block
// the UI event loop.
func Launch(method Method, command string) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("launch: empty command")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch method {
	case MethodTmux:
		return runSpawn(ctx, "tmux", "new-window", command)

	case MethodWezTerm:
		// wezterm cli spawn -- sh -c "<command>"
		// "--" is required so wezterm doesn't try to interpret our
		// flags. The shell wrap keeps behavior consistent across
		// backends (see package doc).
		return runSpawn(ctx, "wezterm", "cli", "spawn", "--", "sh", "-c", command)

	case MethodKitty:
		// kitten @ launch --type=tab --cwd current -- sh -c "<command>"
		// --cwd current inherits cliff's working directory, which is
		// what you'd expect after "install X" → "open X".
		return runSpawn(ctx, "kitten", "@", "launch", "--type=tab", "--cwd", "current", "--", "sh", "-c", command)

	case MethodITerm2:
		return launchITerm2(ctx, command)

	case MethodUnsupported:
		return fmt.Errorf("launch: no supported terminal detected")
	}
	return fmt.Errorf("launch: unknown method %d", method)
}

// runSpawn is the common exec path. It captures stderr so failures can
// be surfaced to the user (e.g. "tmux: can't find server") rather than
// swallowed.
func runSpawn(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed != "" {
			return fmt.Errorf("%s: %s", name, trimmed)
		}
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// launchITerm2 drives iTerm2 via AppleScript. This uses iTerm2's
// scripting dictionary (deprecated but functional through 3.5+). We
// create a tab in the current window and write the command to its
// session; iTerm2 runs it under the session's login shell, so PATH
// changes from a just-applied rc edit take effect automatically.
//
// The escaping strategy: AppleScript strings use double quotes and
// escape " and \ with a backslash. We do that inline rather than
// pulling in a whole osascript library for a single call.
func launchITerm2(ctx context.Context, command string) error {
	escaped := strings.ReplaceAll(command, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	script := fmt.Sprintf(`
tell application "iTerm2"
  activate
  if (count of windows) = 0 then
    create window with default profile
  end if
  tell current window
    set newTab to (create tab with default profile)
    tell current session of newTab
      write text "%s"
    end tell
  end tell
end tell`, escaped)
	return runSpawn(ctx, "osascript", "-e", script)
}

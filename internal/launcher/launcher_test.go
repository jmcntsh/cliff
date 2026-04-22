package launcher

import "testing"

func TestDetect(t *testing.T) {
	cases := []struct {
		name string
		env  Env
		want Method
	}{
		{
			name: "empty env → unsupported",
			env:  Env{},
			want: MethodUnsupported,
		},
		{
			name: "tmux wins over everything",
			env: Env{
				Tmux:          "/private/tmp/tmux-501/default,12345,0",
				WezTermSocket: "/tmp/wezterm.sock",
				KittyListen:   "unix:/tmp/kitty",
				TermProgram:   "iTerm.app",
				GOOS:          "darwin",
			},
			want: MethodTmux,
		},
		{
			name: "wezterm socket wins over kitty and iterm",
			env: Env{
				WezTermSocket: "/tmp/wezterm.sock",
				KittyListen:   "unix:/tmp/kitty",
				TermProgram:   "iTerm.app",
				GOOS:          "darwin",
			},
			want: MethodWezTerm,
		},
		{
			name: "wezterm via TERM_PROGRAM alone",
			env: Env{
				TermProgram: "WezTerm",
				GOOS:        "linux",
			},
			want: MethodWezTerm,
		},
		{
			name: "kitty without listen socket → not detected",
			// This is the subtle rule: $TERM_PROGRAM=kitty isn't enough
			// because `kitten @` fails without remote control on. We'd
			// rather fall through to "copy command" than promise a
			// tab and fail at spawn time.
			env: Env{
				TermProgram: "kitty",
				GOOS:        "linux",
			},
			want: MethodUnsupported,
		},
		{
			name: "kitty with listen socket → kitty",
			env: Env{
				KittyListen: "unix:/tmp/kitty-12345",
				TermProgram: "kitty",
				GOOS:        "linux",
			},
			want: MethodKitty,
		},
		{
			name: "iterm2 on macOS",
			env: Env{
				TermProgram: "iTerm.app",
				GOOS:        "darwin",
			},
			want: MethodITerm2,
		},
		{
			name: "iterm2 on linux is nonsense → unsupported",
			// Guards against the theoretical case of something setting
			// TERM_PROGRAM=iTerm.app on a non-mac; osascript wouldn't
			// exist there and we'd trip at spawn time.
			env: Env{
				TermProgram: "iTerm.app",
				GOOS:        "linux",
			},
			want: MethodUnsupported,
		},
		{
			name: "Terminal.app → unsupported",
			// Terminal.app can open tabs via AppleScript+System Events,
			// but it requires accessibility permission and feels like
			// a malware prompt to first-timers. Explicitly opt out.
			env: Env{
				TermProgram: "Apple_Terminal",
				GOOS:        "darwin",
			},
			want: MethodUnsupported,
		},
		{
			name: "Alacritty → unsupported",
			env: Env{
				TermProgram: "Alacritty",
				GOOS:        "linux",
			},
			want: MethodUnsupported,
		},
		{
			name: "Ghostty → unsupported",
			// Ghostty doesn't expose a programmatic tab API yet. Re-
			// evaluate if/when it does.
			env: Env{
				TermProgram: "ghostty",
				GOOS:        "darwin",
			},
			want: MethodUnsupported,
		},
		{
			name: "vscode integrated terminal → unsupported",
			env: Env{
				TermProgram: "vscode",
				GOOS:        "darwin",
			},
			want: MethodUnsupported,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Detect(tc.env); got != tc.want {
				t.Errorf("Detect(%+v) = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}

func TestMethod_String(t *testing.T) {
	// The String() is only used in logs and error messages, but pin
	// it so a stray enum reorder doesn't silently change user-facing
	// text.
	cases := []struct {
		m    Method
		want string
	}{
		{MethodUnsupported, "unsupported"},
		{MethodTmux, "tmux"},
		{MethodWezTerm, "WezTerm"},
		{MethodKitty, "Kitty"},
		{MethodITerm2, "iTerm2"},
	}
	for _, tc := range cases {
		if got := tc.m.String(); got != tc.want {
			t.Errorf("Method(%d).String() = %q, want %q", tc.m, got, tc.want)
		}
	}
}

func TestLaunch_EmptyCommand(t *testing.T) {
	// The method-specific backends would get to spawn their binaries
	// and hit various errors on an empty string; centralize that
	// guard in Launch for a predictable error message.
	if err := Launch(MethodTmux, ""); err == nil {
		t.Error("expected error for empty command, got nil")
	}
	if err := Launch(MethodTmux, "   "); err == nil {
		t.Error("expected error for whitespace-only command, got nil")
	}
}

func TestLaunch_Unsupported(t *testing.T) {
	if err := Launch(MethodUnsupported, "echo hi"); err == nil {
		t.Error("expected error for MethodUnsupported, got nil")
	}
}

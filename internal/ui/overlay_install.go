package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/internal/install"
	"github.com/jmcntsh/cliff/internal/launcher"
	"github.com/jmcntsh/cliff/internal/pathfix"
	"github.com/jmcntsh/cliff/internal/ui/theme"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// installResultMsg fires when an in-flight install completes. The Result
// embeds the originating App so the receiver can mark state without
// needing to thread it through.
type installResultMsg struct {
	Result install.Result
}

// installStartedMsg fires synchronously from runInstallCmd's tea.Cmd
// with the cancel func for the in-flight install. The receiver stashes
// it so `esc` in modeInstallRunning can kill the child process.
type installStartedMsg struct {
	Cancel context.CancelFunc
}

// installLineMsg is posted via tea.Program.Send per stdout/stderr line
// from the running install. Many can fire in quick succession.
type installLineMsg struct {
	Line string
}

// program is set once from main.go via SetProgram so goroutines outside
// of a tea.Cmd (namely the install streamer) can push messages into the
// running Bubble Tea event loop. Nil until the TUI has started.
var program *tea.Program

// SetProgram registers the running tea.Program. main.go must call this
// after tea.NewProgram and before user input can trigger an install.
func SetProgram(p *tea.Program) { program = p }

// runInstallCmd kicks off the install in a background goroutine and
// returns an installStartedMsg immediately with the cancel func. As the
// install runs, stdout/stderr lines are forwarded to the UI via
// installLineMsg, and completion is reported via installResultMsg.
func runInstallCmd(app *catalog.App) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			// Always release context resources. On esc, cancel() is also
			// invoked from the UI to kill the child; calling it twice is
			// safe. Without this defer, a normal completion leaks the
			// context's internal goroutine per install.
			defer cancel()
			res := install.Stream(ctx, app, func(line string) {
				if program != nil {
					program.Send(installLineMsg{Line: line})
				}
			})
			if program != nil {
				program.Send(installResultMsg{Result: res})
			}
		}()
		return installStartedMsg{Cancel: cancel}
	}
}

// runUninstallCmd mirrors runInstallCmd but drives UninstallCommand
// through StreamCmd. Shares the same installLineMsg/installResultMsg
// envelope so the running/result views can consume one stream. The
// receiver decides which verb to render based on r.installOp.
func runUninstallCmd(app *catalog.App, cmd string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			defer cancel()
			res := install.StreamCmd(ctx, app, cmd, func(line string) {
				if program != nil {
					program.Send(installLineMsg{Line: line})
				}
			})
			if program != nil {
				program.Send(installResultMsg{Result: res})
			}
		}()
		return installStartedMsg{Cancel: cancel}
	}
}

// runUpgradeCmd is the upgrade-specific analog of the runners above.
// Structurally identical to runUninstallCmd — same StreamCmd, same
// envelope — but takes the pre-derived UpgradeCommand string. Lives
// beside its siblings so the three package-op runners read as one
// shape with one parameter changed.
func runUpgradeCmd(app *catalog.App, cmd string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			defer cancel()
			res := install.StreamCmd(ctx, app, cmd, func(line string) {
				if program != nil {
					program.Send(installLineMsg{Line: line})
				}
			})
			if program != nil {
				program.Send(installResultMsg{Result: res})
			}
		}()
		return installStartedMsg{Cancel: cancel}
	}
}

// installConfirmView is the modal shown after `i`. It displays the exact
// shell command that will run and a stronger warning for script-type
// installs (per CLAUDE.md §3 and notes/manifest.md).
func installConfirmView(app *catalog.App, width int) string {
	if app == nil || app.InstallSpec == nil {
		return modalBox(width,
			theme.WarnText.Render("No install available")+"\n\n"+
				theme.MutedText.Render("This app has no install spec in the manifest.")+"\n\n"+
				theme.MutedText.Render("esc close"))
	}

	header := theme.AccentBold.Render("Install ") +
		theme.FocusText.Render(app.Name) +
		theme.AccentBold.Render("?")
	// Literal command as the subline — more useful than naming the
	// package manager once you know that's where 'i' goes, and it's
	// what CLAUDE.md §3 asks the confirm modal to surface anyway.
	subline := theme.MutedText.Render(app.InstallSpec.Shell())

	body := []string{
		header,
		subline,
	}

	if app.InstallSpec.Type == "script" {
		body = append(body,
			"",
			theme.WarnText.Render("⚠  This is a `script`-type install."),
			theme.MutedText.Render("cliff does not review or sandbox what this command does."),
			theme.MutedText.Render("It runs with your full shell privileges. Inspect the URL"),
			theme.MutedText.Render("above before continuing."),
		)
	}

	body = append(body,
		"",
		theme.MutedText.Render("⏎ run     esc cancel"),
	)

	return modalBox(width, strings.Join(body, "\n"))
}

func installRunningView(app *catalog.App, vp viewport.Model, hasOutput bool, width int) string {
	header := theme.AccentBold.Render("Installing ") + theme.FocusText.Render(app.Name)

	outputBlock := renderLogViewport(vp, hasOutput, "(starting…)")

	body := []string{
		header,
		"",
		theme.MutedText.Render(app.InstallSpec.Shell()),
		"",
		outputBlock,
		"",
		theme.MutedText.Render("↑↓/jk scroll  esc cancel"),
	}
	return modalBox(width, strings.Join(body, "\n"))
}

func installResultView(res *install.Result, vp viewport.Model, launchMethod launcher.Method, launchErr error, width int) string {
	if res == nil {
		return modalBox(width, "no result")
	}
	app := res.App

	var status string
	if res.Err == nil {
		status = theme.OKText.Render("✓ Installed " + app.Name)
	} else {
		status = theme.ErrorText.Render(fmt.Sprintf("✗ Install failed (exit %d)", res.ExitCode))
	}

	hasOutput := strings.TrimSpace(res.Output) != ""
	outputBlock := renderLogViewport(vp, hasOutput, "(no output)")

	body := []string{status}
	// Surface an actionable hint for recognized failures (e.g. "brew
	// isn't installed"). Placed right after the status so it's the
	// first thing the user sees below ✗.
	if hint := install.Diagnose(*res); hint != "" {
		body = append(body,
			"",
			lipgloss.NewStyle().
				Foreground(theme.ColorWarn).
				Render(hint))
	}
	// Successful install but the binary landed off $PATH. The ✓
	// marker reflects the filesystem (honest), but the user's shell
	// still can't run the app until the dir is on $PATH. We offer to
	// do the dotfile edit in-place (see modeFixPath) — "press ⏎" is
	// the prompt because it's the only remaining step between "app
	// installed" and "I can type its name in a new terminal."
	if res.Err == nil && res.PathWarning != nil {
		pw := res.PathWarning
		headline := fmt.Sprintf("Installed to %s, but that directory isn't on your $PATH.", pw.Dir)
		prompt := "Press ⏎ to add it automatically, or esc to dismiss."
		body = append(body,
			"",
			lipgloss.NewStyle().
				Foreground(theme.ColorWarn).
				Render(headline),
			theme.MutedText.Render(prompt),
		)
	}

	// Clean success with no PathWarning: offer to launch the app in
	// a new tab (when the host terminal supports it). This is the
	// zero-friction finish line — install → ⏎ → running in the next
	// tab while cliff stays open in this one. When we can't spawn a
	// tab, fall back to "copy command" so the user still has a single
	// keystroke path to trying the app out.
	showLaunch := res.Err == nil && res.PathWarning == nil && app != nil && app.BinaryName() != ""
	if showLaunch {
		bin := app.BinaryName()
		if launchErr != nil {
			// The previous Launch attempt on this modal failed.
			// Surface the error in-line so the user sees why their
			// tab didn't open, and leave the affordance as "copy"
			// so a second Enter does something useful.
			body = append(body,
				"",
				theme.ErrorText.Render("× Couldn't open a new tab: "+launchErr.Error()),
				theme.MutedText.Render("Run this in any terminal: "+bin),
			)
		} else if launchMethod == launcher.MethodUnsupported {
			body = append(body,
				"",
				theme.MutedText.Render("Run ")+theme.FocusText.Render(bin)+theme.MutedText.Render(" in a new terminal tab to try it out."),
			)
		} else {
			body = append(body,
				"",
				theme.MutedText.Render("Try it: ")+theme.FocusText.Render(bin),
			)
		}
	}

	body = append(body,
		"",
		theme.MutedText.Render(res.Command),
		"",
		outputBlock,
	)
	// Footer hint: three mutually exclusive cases, labeled so ⏎ never
	// does a surprise thing.
	switch {
	case res.Err == nil && res.PathWarning != nil:
		body = append(body, "", theme.MutedText.Render("⏎ fix PATH  ↑↓/jk scroll  esc close"))
	case showLaunch && launchErr == nil && launchMethod != launcher.MethodUnsupported:
		body = append(body, "", theme.MutedText.Render("⏎ open in new tab  c copy  ↑↓/jk scroll  esc close"))
	case showLaunch:
		body = append(body, "", theme.MutedText.Render("⏎ copy command  ↑↓/jk scroll  esc close"))
	default:
		body = append(body, "", theme.MutedText.Render("↑↓/jk scroll  pgup/pgdn page  esc close"))
	}
	return modalBox(width, strings.Join(body, "\n"))
}

// fixPathView draws the "add <dir> to $PATH?" prompt and, once the
// user has confirmed and we've written the rc, the success (or
// fallback) result. Two screens sharing one view function because
// they wear the same modal chrome and only differ in body + keybinds.
//
// The single err argument carries the Detect error in the pre-apply
// phase and the Apply error post-apply (the Root overwrites fixErr
// when it calls Apply). alreadyPresent is the snapshot of Plan.Present
// taken at Detect time, so the post-apply screen can distinguish
// "just added" from "was already in the rc" after Apply has flipped
// Plan.Present unconditionally.
func fixPathView(plan *pathfix.Plan, err error, applied, alreadyPresent bool, app *catalog.App, launchMethod launcher.Method, launchErr error, width int) string {
	if plan == nil {
		return modalBox(width, theme.ErrorText.Render("internal error: no path-fix plan"))
	}

	if !applied {
		// Confirm phase: preview the exact file and line before writing.
		header := theme.AccentBold.Render("Add to $PATH?")

		var body []string
		body = append(body, header, "")

		if err == pathfix.ErrShellUnsupported {
			// Fish or an unknown shell. We can't auto-edit safely —
			// show the line the user would have to add by hand and
			// let them dismiss.
			body = append(body,
				theme.WarnText.Render("cliff can't auto-edit your shell config."),
				theme.MutedText.Render("Shell detected: "+shellLabel(plan.Shell)),
				"",
				theme.MutedText.Render("Add this line yourself, then reopen the terminal:"),
				theme.FocusText.Render("  "+plan.Line),
				"",
				theme.MutedText.Render(fmt.Sprintf("File: %s", plan.RcPath)),
				"",
				theme.MutedText.Render("esc close"),
			)
			return modalBox(width, strings.Join(body, "\n"))
		}

		body = append(body,
			theme.MutedText.Render("cliff will append this line to:"),
			theme.FocusText.Render("  "+plan.RcPath),
			"",
			theme.MutedText.Render("Line:"),
			theme.FocusText.Render("  "+plan.Line),
			"",
		)
		if plan.Present {
			body = append(body,
				theme.OKText.Render("Already present — this will be a no-op."),
				"",
			)
		} else {
			body = append(body,
				theme.MutedText.Render("Open a new terminal (or `source` the file) to pick it up."),
				"",
			)
		}
		body = append(body,
			theme.MutedText.Render("⏎ apply     esc cancel"),
		)
		return modalBox(width, strings.Join(body, "\n"))
	}

	// Result phase.
	header := theme.AccentBold.Render("$PATH")
	var body []string
	body = append(body, header, "")

	switch {
	case err == pathfix.ErrShellUnsupported:
		// Reachable when the user hit Enter on a fish/unknown shell
		// anyway (Apply returns ErrShellUnsupported rather than
		// writing bash syntax into config.fish).
		body = append(body,
			theme.WarnText.Render("Shell not supported for auto-edit."),
			"",
			theme.MutedText.Render("Add this line yourself, then reopen the terminal:"),
			theme.FocusText.Render("  "+plan.Line),
		)
	case err != nil:
		body = append(body,
			theme.ErrorText.Render("× Couldn't update "+plan.RcPath),
			"",
			theme.MutedText.Render(err.Error()),
			"",
			theme.MutedText.Render("Line to add by hand:"),
			theme.FocusText.Render("  "+plan.Line),
		)
	case alreadyPresent:
		body = append(body,
			theme.OKText.Render("✓ Already configured."),
			theme.MutedText.Render(plan.Line+" is already in "+plan.RcPath+"."),
		)
	default:
		body = append(body,
			theme.OKText.Render("✓ Added to "+plan.RcPath),
			"",
			theme.MutedText.Render("Appended:"),
			theme.FocusText.Render("  "+plan.Line),
		)
	}

	// Post-apply success path: the rc change is written, but the
	// current cliff shell won't see it (cliff was started before the
	// edit). A new tab will source the edited rc and have the binary
	// on PATH, so "open in new tab" is precisely the right hand-off.
	//
	// We only offer it when Apply actually succeeded (err == nil).
	// ErrShellUnsupported means the rc wasn't touched, so a new tab
	// still wouldn't have PATH set — no point spawning one.
	showLaunch := err == nil && app != nil && app.BinaryName() != ""
	if showLaunch {
		bin := app.BinaryName()
		if launchErr != nil {
			body = append(body,
				"",
				theme.ErrorText.Render("× Couldn't open a new tab: "+launchErr.Error()),
				theme.MutedText.Render("Run this in any terminal: "+bin),
			)
		} else if launchMethod == launcher.MethodUnsupported {
			body = append(body,
				"",
				theme.MutedText.Render("Open a new terminal and run: ")+theme.FocusText.Render(bin),
			)
		} else {
			body = append(body,
				"",
				theme.MutedText.Render("Try it: ")+theme.FocusText.Render(bin),
			)
		}
	} else {
		// No launch affordance (no app context, or the apply path
		// isn't "clean success"); keep the pre-launcher hint so the
		// user still knows the next step.
		switch {
		case err == pathfix.ErrShellUnsupported,
			err != nil:
			// Already shown above in the per-case switch.
		case alreadyPresent:
			body = append(body, "", theme.MutedText.Render("Open a new terminal to use the tool."))
		default:
			body = append(body,
				"",
				theme.MutedText.Render("Open a new terminal (or `source "+plan.RcPath+"`)"),
				theme.MutedText.Render("to pick it up."),
			)
		}
	}

	// Footer keybinds. When we can open a tab, Enter is the launch
	// action; otherwise Enter is a plain dismiss (same as before).
	switch {
	case showLaunch && launchErr == nil && launchMethod != launcher.MethodUnsupported:
		body = append(body, "", theme.MutedText.Render("⏎ open in new tab  esc close"))
	case showLaunch && launchMethod == launcher.MethodUnsupported:
		body = append(body, "", theme.MutedText.Render("⏎ copy command  esc close"))
	default:
		body = append(body, "", theme.MutedText.Render("⏎ or esc close"))
	}
	return modalBox(width, strings.Join(body, "\n"))
}

func shellLabel(k pathfix.ShellKind) string {
	switch k {
	case pathfix.ShellZsh:
		return "zsh"
	case pathfix.ShellBash:
		return "bash"
	case pathfix.ShellFish:
		return "fish"
	default:
		return "unknown"
	}
}

// renderLogViewport boxes the scrollable viewport for the install log
// modals and shows a scroll indicator in the bottom-right when there's
// content to scroll. Shared between the running and result views so the
// two visually match.
func renderLogViewport(vp viewport.Model, hasOutput bool, emptyPlaceholder string) string {
	if !hasOutput {
		return lipgloss.NewStyle().
			Foreground(theme.ColorMuted).
			Italic(true).
			Padding(0, 1).
			Width(installLogWidth).
			Render(emptyPlaceholder)
	}
	inner := lipgloss.NewStyle().
		Foreground(theme.ColorText).
		Background(theme.ColorPanel).
		Padding(0, 1).
		Render(vp.View())
	// Scroll indicator — only shown once the content exceeds the viewport.
	pct := vp.ScrollPercent()
	if vp.TotalLineCount() <= vp.Height {
		return inner
	}
	indicator := lipgloss.NewStyle().
		Foreground(theme.ColorMuted).
		Align(lipgloss.Right).
		Width(installLogWidth).
		Render(fmt.Sprintf("%3.0f%%", pct*100))
	return inner + "\n" + indicator
}

func modalBox(width int, content string) string {
	maxW := width - 8
	if maxW < 40 {
		maxW = 40
	}
	if maxW > 90 {
		maxW = 90
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(1, 3).
		Width(maxW).
		Render(content)
}

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

// installStartedMsg fires synchronously from runPkgCmd's tea.Cmd with
// the cancel func for the in-flight child process. The receiver
// stashes it so `esc` in modePkgRunning can kill whatever is running
// (install, uninstall, or upgrade — the envelope doesn't care).
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

// runPkgCmd kicks off a package operation — install, uninstall, or
// upgrade — in a background goroutine and returns an installStartedMsg
// immediately with the cancel func. As the op runs, stdout/stderr lines
// are forwarded via installLineMsg, and completion is reported via
// installResultMsg. The caller passes the pre-derived shell command
// (via pkgOpCommand) so this runner is op-agnostic: install.StreamCmd
// doesn't care whether `cmd` is `brew install foo`, `brew uninstall
// foo`, or `brew upgrade foo` — it just runs it and captures output.
//
// Consolidates the former runInstallCmd / runUninstallCmd /
// runUpgradeCmd trio, which were identical except for the install
// variant hard-coding InstallSpec.Shell(). Now the "what string to
// run" decision lives in one place (pkgOpCommand) and this runner is
// the one place that spawns a goroutine around StreamCmd.
func runPkgCmd(app *catalog.App, cmd string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			// Always release context resources. On esc, cancel() is
			// also invoked from the UI to kill the child; calling it
			// twice is safe. Without this defer, a normal completion
			// leaks the context's internal goroutine per op.
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

// pkgConfirmView is the unified confirm modal for any package op. The
// op argument drives labels ("Install" / "Uninstall" / "Update") and
// the command-derivation rule; the script-type warning is install-only
// because uninstall/upgrade don't shell out to an unsolicited remote
// script (they run the inverse / latest of the known install verb).
//
// When the op isn't supported for the current app (no install spec,
// script-type without author-provided [uninstall] / [upgrade] blocks,
// etc.) we show an honest "can't X" message with the install type
// named — the user can then close and try a different action. The
// update handler treats ⏎ on this state as a dismiss.
func pkgConfirmView(app *catalog.App, op pkgOp, width int) string {
	if app == nil {
		return modalBox(width,
			theme.WarnText.Render("No app selected")+"\n\n"+
				theme.MutedText.Render("esc close"))
	}

	cmd := pkgOpCommand(app, op)
	if cmd == "" {
		return pkgNotAvailableView(app, op, width)
	}

	header := theme.GradientTitle(op.verb() + " " + app.Name + "?")
	// Literal command as the subline — more useful than naming the
	// package manager and what CLAUDE.md §3 asks the confirm modal
	// to surface anyway.
	subline := theme.MutedText.Render(cmd)

	body := []string{header, subline}

	// Script-type warning is install-only: uninstall/upgrade use
	// derived commands that don't curl-pipe-sh an unknown URL.
	if op == pkgOpInstall && app.PrimaryInstallSpec() != nil && app.PrimaryInstallSpec().Type == "script" {
		body = append(body,
			"",
			theme.WarnText.Render("⚠  This is a `script`-type install."),
			theme.MutedText.Render("cliff does not review or sandbox what this command does."),
			theme.MutedText.Render("It runs with your full shell privileges. Inspect the URL"),
			theme.MutedText.Render("above before continuing."),
		)
	}

	body = append(body, "", theme.MutedText.Render("⏎ run     esc cancel"))
	return modalBox(width, strings.Join(body, "\n"))
}

// pkgNotAvailableView renders the "can't X this app" message for ops
// that have no recipe in this manifest. Rendered verb changes per op;
// the rest of the copy is shared.
func pkgNotAvailableView(app *catalog.App, op pkgOp, width int) string {
	typeLabel := "unknown"
	if s := app.PrimaryInstallSpec(); s != nil && s.Type != "" {
		typeLabel = s.Type
	}

	var title, recipeKind string
	switch op {
	case pkgOpUninstall:
		title = "Can't uninstall " + app.Name
		recipeKind = "[uninstall]"
	case pkgOpUpgrade:
		title = "Can't update " + app.Name
		recipeKind = "[upgrade]"
	default:
		// The install path reaches this branch only when there's no
		// install spec at all (rare — the registry lint rejects it).
		return modalBox(width,
			theme.WarnText.Render("No install available")+"\n\n"+
				theme.MutedText.Render("This app has no install spec in the manifest.")+"\n\n"+
				theme.MutedText.Render("esc close"))
	}

	body := []string{
		theme.WarnText.Render(title),
		"",
		theme.MutedText.Render(fmt.Sprintf("cliff has no recipe for install type: %s", typeLabel)),
	}
	if typeLabel == "script" {
		body = append(body,
			theme.MutedText.Render(fmt.Sprintf("Script-type apps need an author-provided %s block", recipeKind)),
			theme.MutedText.Render("in the registry manifest."),
		)
	}
	body = append(body, "", theme.MutedText.Render("esc close"))
	return modalBox(width, strings.Join(body, "\n"))
}

// pkgRunningView is the unified streaming-logs modal. The op argument
// drives the header verb ("Installing" / "Uninstalling" / "Updating");
// everything else — subline command, viewport, footer — is identical
// across ops, which is why the triplet collapsed to one function.
//
// spinnerGlyph is the rotating dot from the Root spinner; passed in
// (rather than imported) so the view stays a pure renderer and the
// modal still works in tests that construct a static frame.
func pkgRunningView(app *catalog.App, op pkgOp, vp viewport.Model, hasOutput bool, spinnerGlyph string, width int) string {
	header := theme.GradientTitle(op.runningVerb() + " " + app.Name)
	placeholder := "starting…"
	if spinnerGlyph != "" {
		placeholder = spinnerGlyph + " " + placeholder
	}
	outputBlock := renderLogViewport(vp, hasOutput, placeholder)

	body := []string{
		header,
		"",
		theme.MutedText.Render(pkgOpCommand(app, op)),
		"",
		outputBlock,
		"",
		theme.MutedText.Render("↑↓/jk scroll  esc cancel"),
	}
	return modalBox(width, strings.Join(body, "\n"))
}

// pkgResultView is the unified terminal modal. The launcher + PathWarning
// follow-ups are install-only — uninstall/upgrade render a plain
// dismiss footer regardless of result, because once an app is gone (or
// upgraded) there's no hand-off step that would be useful.
func pkgResultView(res *install.Result, op pkgOp, vp viewport.Model, launchMethod launcher.Method, launchErr error, overrides map[string]string, width int) string {
	if res == nil {
		return modalBox(width, "no result")
	}
	app := res.App
	appName := "app"
	if app != nil {
		appName = app.Name
	}

	var status string
	if res.Err == nil {
		status = theme.OKText.Render("✓ " + op.pastVerb() + " " + appName)
	} else {
		status = theme.ErrorText.Render(fmt.Sprintf("✗ %s failed (exit %d)", op.verb(), res.ExitCode))
	}

	hasOutput := strings.TrimSpace(res.Output) != ""
	outputBlock := renderLogViewport(vp, hasOutput, "(no output)")

	body := []string{status}
	if hint := install.Diagnose(*res); hint != "" {
		body = append(body, "", theme.Warn.Render(hint))
	}

	// Install-only follow-up section: PathWarning + launcher.
	installing := op == pkgOpInstall
	if installing && res.Err == nil && res.PathWarning != nil {
		pw := res.PathWarning
		headline := fmt.Sprintf("Installed to %s, but that directory isn't on your $PATH.", pw.Dir)
		prompt := "Press ⏎ to add it automatically, or esc to dismiss."
		body = append(body,
			"",
			theme.Warn.Render(headline),
			theme.MutedText.Render(prompt),
		)
	}

	showLaunch := installing && res.Err == nil && res.PathWarning == nil && app != nil && app.ResolvedBinaryName(overrides) != ""
	if showLaunch {
		bin := app.ResolvedBinaryName(overrides)
		if launchErr != nil {
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

	// Footer hint. Install has three mutually-exclusive Enter meanings;
	// uninstall/upgrade have exactly one (dismiss).
	switch {
	case installing && res.Err == nil && res.PathWarning != nil:
		body = append(body, "", theme.MutedText.Render("⏎ fix PATH  ↑↓/jk scroll  esc close"))
	case installing && showLaunch && launchErr == nil && launchMethod != launcher.MethodUnsupported:
		body = append(body, "", theme.MutedText.Render("⏎ open in new tab  c copy  ↑↓/jk scroll  esc close"))
	case installing && showLaunch:
		body = append(body, "", theme.MutedText.Render("⏎ copy command  ↑↓/jk scroll  esc close"))
	default:
		body = append(body, "", theme.MutedText.Render("↑↓/jk scroll  pgup/pgdn page  ⏎ or esc close"))
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
func fixPathView(plan *pathfix.Plan, err error, applied, alreadyPresent bool, app *catalog.App, launchMethod launcher.Method, launchErr error, overrides map[string]string, width int) string {
	if plan == nil {
		return modalBox(width, theme.ErrorText.Render("internal error: no path-fix plan"))
	}

	if !applied {
		// Confirm phase: preview the exact file and line before writing.
		header := theme.GradientTitle("Add to $PATH?")

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
	header := theme.GradientTitle("$PATH")
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
	showLaunch := err == nil && app != nil && app.ResolvedBinaryName(overrides) != ""
	if showLaunch {
		bin := app.ResolvedBinaryName(overrides)
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

// Styles for the scrollable log block. Built once at package load
// because they're used on every renderLogViewport call (i.e. every
// frame the install/uninstall/upgrade modal is on screen) and have no
// per-frame inputs.
var (
	logPlaceholderStyle = lipgloss.NewStyle().
				Foreground(theme.ColorMuted).
				Italic(true).
				Padding(0, 1).
				Width(installLogWidth)

	logInnerStyle = lipgloss.NewStyle().
			Foreground(theme.ColorText).
			Background(theme.ColorPanel).
			Padding(0, 1)

	logScrollIndicatorStyle = lipgloss.NewStyle().
				Foreground(theme.ColorMuted).
				Align(lipgloss.Right).
				Width(installLogWidth)
)

// renderLogViewport boxes the scrollable viewport for the install log
// modals and shows a scroll indicator in the bottom-right when there's
// content to scroll. Shared between the running and result views so the
// two visually match.
func renderLogViewport(vp viewport.Model, hasOutput bool, emptyPlaceholder string) string {
	if !hasOutput {
		return logPlaceholderStyle.Render(emptyPlaceholder)
	}
	inner := logInnerStyle.Render(vp.View())
	// Scroll indicator — only shown once the content exceeds the viewport.
	if vp.TotalLineCount() <= vp.Height {
		return inner
	}
	indicator := logScrollIndicatorStyle.Render(fmt.Sprintf("%3.0f%%", vp.ScrollPercent()*100))
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
		BorderTopForeground(theme.ColorAccent).
		BorderLeftForeground(theme.ColorAccent).
		BorderRightForeground(theme.ColorAccentMid).
		BorderBottomForeground(theme.ColorAccentAlt).
		Padding(1, 3).
		Width(maxW).
		Render(content)
}

package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/internal/install"
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

func installResultView(res *install.Result, vp viewport.Model, width int) string {
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
	// still can't run the app until they add the dir to PATH, so we
	// tell them exactly what line to stick in ~/.zshrc (or ~/.bashrc).
	if res.Err == nil && res.PathWarning != nil {
		pw := res.PathWarning
		msg := fmt.Sprintf(
			"Installed to %s, but that directory isn't on your $PATH.\n"+
				"Add this to your shell rc (~/.zshrc or ~/.bashrc), then reopen the terminal:\n"+
				"  export PATH=\"%s:$PATH\"",
			pw.Dir, pw.Dir)
		body = append(body,
			"",
			lipgloss.NewStyle().
				Foreground(theme.ColorWarn).
				Render(msg))
	}
	body = append(body,
		"",
		theme.MutedText.Render(res.Command),
		"",
		outputBlock,
		"",
		theme.MutedText.Render("↑↓/jk scroll  pgup/pgdn page  esc close"),
	)
	return modalBox(width, strings.Join(body, "\n"))
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

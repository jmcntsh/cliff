package main

import (
	"fmt"
	"os"

	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "completions":
			os.Exit(cmdCompletions(os.Args[2:]))
		case "version", "--version", "-v":
			os.Exit(cmdVersion())
		case "help", "--help", "-h":
			os.Exit(cmdHelp())
		}
	}
	os.Exit(runTUI())
}

func runTUI() int {
	// Detect terminal background BEFORE tea takes over. Once tea enters
	// raw mode + alt screen, OSC 11 round-trips through stdin become
	// unreliable, so glamour's auto-style consistently picks dark.
	// CLIFF_BG=light|dark overrides for users on terminals that don't
	// answer OSC 11 (some SSH/tmux setups).
	dark := lipgloss.HasDarkBackground()
	switch os.Getenv("CLIFF_BG") {
	case "light":
		dark = false
	case "dark":
		dark = true
	}
	ui.SetDarkBackground(dark)

	res := catalog.LoadWithFallback(catalog.LoadOptions{})
	if res.Catalog == nil {
		fmt.Fprintln(os.Stderr, "load catalog:", res.Err)
		return 1
	}
	if res.Err != nil && os.Getenv("CLIFF_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "catalog source=%s (fetch note: %v)\n", res.Source, res.Err)
	}
	p := tea.NewProgram(ui.New(res.Catalog), tea.WithAltScreen())
	// ui needs a handle on the running program so the install streamer
	// goroutine can push installLineMsg events into tea's event loop.
	ui.SetProgram(p)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

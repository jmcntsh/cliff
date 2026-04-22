package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/jmcntsh/cliff/internal/browser"
	"github.com/jmcntsh/cliff/internal/submit"

	"github.com/mattn/go-isatty"
)

// cmdSubmit runs `cliff submit [name-or-repo]`. Builds a prefilled
// registry-issue URL via internal/submit and either opens it in a
// browser (interactive TTY, default) or prints it for the user to
// paste elsewhere (piped/scripted, or `--print`).
//
// The optional positional arg is treated as a name if it's a bare
// slug, or a repo if it contains a "/" — saves a user "cliff submit
// charmbracelet/glow" one step in the form. Unrecognized flags are
// rejected rather than silently dropped, mirroring parseInstallFlags.
func cmdSubmit(args []string) int {
	var printOnly bool
	var positional []string
	for _, a := range args {
		switch a {
		case "--print", "-p":
			printOnly = true
		case "--help", "-h":
			fmt.Print(submitHelpText)
			return 0
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(os.Stderr, "cliff: unknown flag: %s\n", a)
				return 2
			}
			positional = append(positional, a)
		}
	}

	req := submit.Request{}
	if len(positional) == 1 {
		arg := positional[0]
		if strings.Contains(arg, "/") {
			req.Repo = arg
		} else {
			req.Name = arg
		}
	} else if len(positional) > 1 {
		fmt.Fprintln(os.Stderr, "usage: cliff submit [<name-or-owner/repo>] [--print]")
		return 2
	}

	url := req.URL()

	// --print, or stdout isn't a terminal (piping into something that
	// needs the URL as text): just print and exit. Don't launch a
	// browser in non-interactive contexts; that's the kind of thing
	// that spawns GUI windows out of a CI job.
	if printOnly || !isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Println(url)
		return 0
	}

	fmt.Println("Opening the cliff submission form in your browser:")
	fmt.Println("  " + url)
	fmt.Println()
	if err := browser.Open(url); err != nil {
		fmt.Fprintln(os.Stderr, "couldn't open a browser:", err)
		fmt.Fprintln(os.Stderr, "paste the URL above into any browser to continue.")
		return 1
	}
	fmt.Println("(Finish the form on GitHub — the curator picks it up from there.)")
	return 0
}

const submitHelpText = `cliff submit — nominate an app for the cliff registry.

Usage:
  cliff submit                     open a blank submission form
  cliff submit <name>              prefill with a proposed slug
  cliff submit <owner>/<repo>      prefill with a GitHub repo
  cliff submit --print             print the URL without opening a browser
  cliff submit --help              show this message

The form is a GitHub issue on github.com/` + submit.RegistryRepo + `.
You'll need a GitHub account (the one you already use). The curator
reviews submissions there; accepted apps land in the next registry
build.
`

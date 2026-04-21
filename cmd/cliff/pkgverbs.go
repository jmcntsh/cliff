package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/internal/install"

	"github.com/sahilm/fuzzy"
)

// cmdInstall runs `cliff install <pkg>`. Looks up the app in the
// catalog, prints the exact command that will run (same honesty the
// TUI's confirm modal gives), then streams the install.
func cmdInstall(args []string) int {
	return runPkgVerb("install", args, func(app *catalog.App) string {
		if app.InstallSpec == nil {
			return ""
		}
		return app.InstallSpec.Shell()
	})
}

// cmdUninstall runs `cliff uninstall <pkg>`. Uses InstallSpec.UninstallShell
// which returns "" for script-type installs — those need a manifest-level
// [uninstall] block, not yet wired in. Surfaces a clear error in that case.
func cmdUninstall(args []string) int {
	return runPkgVerb("uninstall", args, func(app *catalog.App) string {
		if app.InstallSpec == nil {
			return ""
		}
		return app.InstallSpec.UninstallShell(install.BinaryName(app))
	})
}

// cmdUpgrade runs `cliff upgrade <pkg>`. Manager-authoritative: we ask
// brew/cargo/pipx/npm/go to do the upgrade and trust their reports;
// there's no cliff-side state of record (see commit 82c2833).
func cmdUpgrade(args []string) int {
	return runPkgVerb("upgrade", args, func(app *catalog.App) string {
		if app.InstallSpec == nil {
			return ""
		}
		return app.InstallSpec.UpgradeShell()
	})
}

// runPkgVerb is the shared body of install/uninstall/upgrade. The
// cmdFor closure tells us which shell command to run for the matched
// app; a "" return means the verb isn't supported for that install type
// (e.g. uninstalling a script-type install, or upgrading anything with
// type=script).
func runPkgVerb(verb string, args []string, cmdFor func(*catalog.App) string) int {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: cliff %s <pkg>\n", verb)
		return 2
	}
	query := args[0]

	res := catalog.LoadWithFallback(catalog.LoadOptions{})
	if res.Catalog == nil {
		fmt.Fprintln(os.Stderr, "cliff: load catalog:", res.Err)
		return 1
	}

	app := lookupApp(query, res.Catalog.Apps)
	if app == nil {
		fmt.Fprintf(os.Stderr, "cliff: no app %q in the registry\n", query)
		printSuggestions(query, res.Catalog.Apps)
		return 1
	}

	cmd := cmdFor(app)
	if cmd == "" {
		fmt.Fprintf(os.Stderr, "cliff: %s is not supported for %s (install type: %s)\n",
			verb, app.Name, installTypeOrUnknown(app))
		if app.InstallSpec != nil && app.InstallSpec.Type == "script" {
			fmt.Fprintln(os.Stderr, "  script-type apps need an author-provided uninstall/upgrade recipe, not yet supported.")
		}
		return 1
	}

	fmt.Printf("%s %s:\n  $ %s\n\n", verbGerund(verb), app.Name, cmd)

	result := install.StreamCmd(
		context.Background(),
		app,
		cmd,
		func(line string) { fmt.Println(line) },
	)

	fmt.Println()
	if result.ExitCode == 0 && result.Err == nil {
		fmt.Printf("✓ %sed %s\n", strings.TrimSuffix(verb, "e"), app.Name)
		return 0
	}
	fmt.Fprintf(os.Stderr, "× %s failed: exit %d\n", verb, result.ExitCode)
	if hint := install.Diagnose(result); hint != "" {
		fmt.Fprintln(os.Stderr, hint)
	}
	if result.ExitCode == 0 {
		return 1
	}
	return result.ExitCode
}

// lookupApp finds an app by case-insensitive match against Name, the
// repo basename, or the full Repo path. Returns nil if no exact match.
func lookupApp(query string, apps []catalog.App) *catalog.App {
	q := strings.ToLower(query)
	for i := range apps {
		a := &apps[i]
		if strings.EqualFold(a.Name, q) {
			return a
		}
		if strings.EqualFold(a.Repo, q) {
			return a
		}
		if slash := strings.LastIndex(a.Repo, "/"); slash >= 0 {
			if strings.EqualFold(a.Repo[slash+1:], q) {
				return a
			}
		}
	}
	return nil
}

// printSuggestions writes up to 5 fuzzy-matched "did you mean" lines
// to stderr. Quiet (no-op) if fuzzy finds nothing plausible.
func printSuggestions(query string, apps []catalog.App) {
	if len(apps) == 0 {
		return
	}
	names := make([]string, len(apps))
	for i, a := range apps {
		names[i] = a.Name
	}
	matches := fuzzy.Find(query, names)
	if len(matches) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr, "did you mean:")
	for i, m := range matches {
		if i >= 5 {
			break
		}
		fmt.Fprintf(os.Stderr, "  %s\n", m.Str)
	}
}

func installTypeOrUnknown(app *catalog.App) string {
	if app == nil || app.InstallSpec == nil {
		return "unknown"
	}
	return app.InstallSpec.Type
}

func verbGerund(verb string) string {
	switch verb {
	case "install":
		return "installing"
	case "uninstall":
		return "uninstalling"
	case "upgrade":
		return "upgrading"
	}
	return verb + "ing"
}

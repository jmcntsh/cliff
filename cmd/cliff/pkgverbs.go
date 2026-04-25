package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jmcntsh/cliff/internal/binmap"
	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/internal/install"
	"github.com/jmcntsh/cliff/internal/pathfix"

	"github.com/mattn/go-isatty"
	"github.com/sahilm/fuzzy"
)

// cmdInstall runs `cliff install <pkg>`. Looks up the app in the
// catalog, picks which install method to use (first with its tool
// available, or --via override), prints the exact command that will
// run (same honesty the TUI's confirm modal gives), then streams.
//
// Accepts --fix-path / --no-fix-path and --via <type> flags. When
// neither fix-path flag is given and stdin is a terminal, we prompt
// interactively; otherwise we fall back to just printing the hint
// (old v0.1.6 behavior), keeping non-interactive pipelines
// deterministic.
func cmdInstall(args []string) int {
	installArgs, mode, via, err := parseInstallFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cliff:", err)
		return 2
	}
	return runPkgVerb("install", installArgs, func(app *catalog.App, _ map[string]string) string {
		spec := app.PreferredInstallSpec(via, toolAvailable)
		if spec == nil {
			if via != "" {
				fmt.Fprintf(os.Stderr, "cliff: %s has no --via %s install method\n", app.Name, via)
				fmt.Fprintf(os.Stderr, "  available methods: %s\n", installMethodsList(app))
			}
			return ""
		}
		return spec.Shell()
	}, mode)
}

// toolAvailable reports whether the package-manager CLI for a given
// install type is on the user's $PATH. Used by PreferredInstallSpec
// to skip methods the user can't actually run. script returns true by
// convention — there's no tool to probe, the shell runs the command
// as-is, and the install's own failure surfaces if something's wrong.
func toolAvailable(installType string) bool {
	var bin string
	switch installType {
	case "brew", "cargo", "go", "npm", "pipx":
		bin = installType
	case "script":
		return true
	default:
		return false
	}
	_, err := exec.LookPath(bin)
	return err == nil
}

func installMethodsList(app *catalog.App) string {
	if app == nil {
		return ""
	}
	types := make([]string, 0, len(app.InstallSpecs))
	for _, s := range app.InstallSpecs {
		types = append(types, s.Type)
	}
	return strings.Join(types, ", ")
}

// fixPathMode controls how runPkgVerb handles a PathWarning from a
// successful install. Only meaningful for the install verb —
// uninstall/upgrade pass fixPathPromptNone.
type fixPathMode int

const (
	fixPathPromptNone  fixPathMode = iota // print hint only, never prompt or apply
	fixPathPromptAuto                     // prompt if TTY, else print hint
	fixPathAlwaysApply                    // --fix-path: apply without prompting
	fixPathNeverApply                     // --no-fix-path: always print hint, never prompt
)

// parseInstallFlags is a small hand-rolled flag parser so we don't
// pull in flag.Parse and its side effects on the global flagset.
// Recognized flags: --fix-path, --no-fix-path, --via <type> (or
// --via=<type>). Positional args are the package name. Extra flags
// are rejected rather than silently passed through, since the rest
// of the CLI doesn't take any.
func parseInstallFlags(args []string) (positional []string, mode fixPathMode, via string, err error) {
	mode = fixPathPromptAuto
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--fix-path":
			mode = fixPathAlwaysApply
		case a == "--no-fix-path":
			mode = fixPathNeverApply
		case a == "--via":
			if i+1 >= len(args) {
				return nil, mode, "", fmt.Errorf("--via requires a value (e.g. --via brew)")
			}
			via = args[i+1]
			i++
		case strings.HasPrefix(a, "--via="):
			via = strings.TrimPrefix(a, "--via=")
			if via == "" {
				return nil, mode, "", fmt.Errorf("--via requires a value (e.g. --via brew)")
			}
		case strings.HasPrefix(a, "-"):
			return nil, mode, "", fmt.Errorf("unknown flag: %s", a)
		default:
			positional = append(positional, a)
		}
	}
	return positional, mode, via, nil
}

// cmdUninstall runs `cliff uninstall <pkg>`. Prefers the manifest's
// [uninstall] block when present; falls back to the type-derived verb.
// Returns "" for script-type installs without a [uninstall] block —
// those require author-provided recipes (enforced at registry CI).
func cmdUninstall(args []string) int {
	return runPkgVerb("uninstall", args, func(app *catalog.App, overrides map[string]string) string {
		return app.UninstallCommandWithOverrides(overrides)
	}, fixPathPromptNone)
}

// cmdUpgrade runs `cliff upgrade <pkg>`. Manager-authoritative: we ask
// brew/cargo/pipx/npm/go to do the upgrade and trust their reports;
// there's no cliff-side state of record (see commit 82c2833). Prefers
// the manifest's [upgrade] block when present.
func cmdUpgrade(args []string) int {
	return runPkgVerb("upgrade", args, func(app *catalog.App, _ map[string]string) string {
		return app.UpgradeCommand()
	}, fixPathPromptNone)
}

// runPkgVerb is the shared body of install/uninstall/upgrade. The
// cmdFor closure tells us which shell command to run for the matched
// app; a "" return means the verb isn't supported for that install type
// (e.g. uninstalling a script-type install, or upgrading anything with
// type=script).
func runPkgVerb(verb string, args []string, cmdFor func(*catalog.App, map[string]string) string, fixMode fixPathMode) int {
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

	overrides := binmap.Load()

	cmd := cmdFor(app, overrides)
	if cmd == "" {
		fmt.Fprintf(os.Stderr, "cliff: %s is not supported for %s (install type: %s)\n",
			verb, app.Name, installTypeOrUnknown(app))
		if hasScriptMethod(app) {
			fmt.Fprintln(os.Stderr, "  script-type apps need an author-provided uninstall/upgrade recipe, not yet supported.")
		}
		return 1
	}

	if verb == "install" {
		if method := matchInstallMethod(app, cmd); method != "" {
			fmt.Printf("%s %s via %s:\n  $ %s\n\n", verbGerund(verb), app.Name, method, cmd)
		} else {
			fmt.Printf("%s %s:\n  $ %s\n\n", verbGerund(verb), app.Name, cmd)
		}
	} else {
		fmt.Printf("%s %s:\n  $ %s\n\n", verbGerund(verb), app.Name, cmd)
	}

	result := install.StreamCmd(
		context.Background(),
		app,
		cmd,
		func(line string) { fmt.Println(line) },
	)

	fmt.Println()
	if result.ExitCode == 0 && result.Err == nil {
		// Learn (or forget) the binary name based on the verb. On
		// install, remember the first detected binary so the next
		// run — and the next uninstall — use the right name. On
		// uninstall, forget the repo so a stale override doesn't
		// outlive the binary. Best-effort: a binmap write failure
		// must never fail a successful package op.
		switch verb {
		case "install":
			if len(result.DetectedBinaries) > 0 {
				_ = binmap.Remember(app.Repo, result.DetectedBinaries[0], app.BinaryName())
			}
		case "uninstall":
			_ = binmap.Forget(app.Repo)
		}
		if verb == "uninstall" {
			if code := verifyUninstalled(app, overrides); code != 0 {
				return code
			}
		}
		fmt.Printf("✓ %sed %s\n", strings.TrimSuffix(verb, "e"), app.Name)
		if pw := result.PathWarning; pw != nil {
			handlePathWarning(pw, fixMode)
		}
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

// verifyUninstalled checks that the app's binary is no longer reachable
// after an uninstall command reports success. Silent success from
// `rm -f` is a common source of "cliff thinks it uninstalled but
// didn't" — wrong GOBIN, asdf toolchain, etc. A post-check turns that
// into a loud failure with a pointer to where the binary still lives.
// We flag any lingering location (on $PATH or in a manager default
// dir) because either case means the recipe didn't fully remove it.
func verifyUninstalled(app *catalog.App, overrides map[string]string) int {
	bin := app.ResolvedBinaryName(overrides)
	if bin == "" {
		return 0
	}
	dir, onPath := install.LocateBinary(bin)
	if dir == "" {
		return 0
	}
	loc := filepath.Join(dir, bin)
	if onPath {
		fmt.Fprintf(os.Stderr, "× %s is still callable at %s — uninstall didn't take effect\n", bin, loc)
	} else {
		fmt.Fprintf(os.Stderr, "× %s still exists at %s (off $PATH, but the uninstall recipe missed it)\n", bin, loc)
	}
	if strings.Contains(loc, "/.asdf/shims/") {
		fmt.Fprintln(os.Stderr, "  looks like an asdf shim; try: asdf reshim")
	}
	return 1
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
	s := app.PrimaryInstallSpec()
	if s == nil {
		return "unknown"
	}
	if len(app.InstallSpecs) > 1 {
		return installMethodsList(app)
	}
	return s.Type
}

// hasScriptMethod reports whether any of the app's declared install
// methods is script. Used by the verb-not-supported error to give a
// targeted hint when at least one method is script-type — that's the
// case where a [uninstall] / [upgrade] block would lift the refusal.
func hasScriptMethod(app *catalog.App) bool {
	if app == nil {
		return false
	}
	for _, s := range app.InstallSpecs {
		if s.Type == "script" {
			return true
		}
	}
	return false
}

// matchInstallMethod returns the install type (brew / cargo / go / ...)
// whose derived Shell() equals cmd, or "" if none match. Used to
// annotate the install output with which method was picked, so the
// user always sees "installing chess-tui via cargo" rather than just
// guessing from the command line. For uninstall/upgrade commands the
// match won't find anything, and the caller skips the annotation.
func matchInstallMethod(app *catalog.App, cmd string) string {
	if app == nil {
		return ""
	}
	for _, s := range app.InstallSpecs {
		if s.Shell() == cmd {
			return s.Type
		}
	}
	return ""
}

// handlePathWarning is the CLI counterpart of the TUI's modeFixPath
// flow. Given a PathWarning and the user's requested mode, it either
// prints the hint (default safe path), prompts y/N on a TTY, or just
// applies. Failures fall back to printing the line to add by hand so
// the user is never left stuck.
func handlePathWarning(pw *install.PathWarning, mode fixPathMode) {
	if mode == fixPathPromptNone {
		return
	}
	plan, detectErr := pathfix.Detect(pw.Dir)
	if errors.Is(detectErr, pathfix.ErrShellUnsupported) {
		printPathHintFallback(plan, pw.Dir)
		return
	}
	if detectErr != nil || plan == nil {
		printPathHintFallback(plan, pw.Dir)
		return
	}

	shouldApply := false
	switch mode {
	case fixPathAlwaysApply:
		shouldApply = true
	case fixPathNeverApply:
		shouldApply = false
	case fixPathPromptAuto:
		// Only prompt when stdin is an interactive terminal AND
		// stdout is too (both matter: we need to read a keystroke
		// and be able to show the prompt). On pipes/redirects we
		// fall back to the non-interactive hint for reproducible
		// scripted use.
		if isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd()) {
			shouldApply = promptYesNo(
				fmt.Sprintf("Add %s to your $PATH via %s? [y/N] ", pw.Dir, plan.RcPath),
				false,
			)
		}
	}

	if !shouldApply {
		printPathHintFallback(plan, pw.Dir)
		return
	}

	if err := pathfix.Apply(plan); err != nil {
		fmt.Fprintf(os.Stderr, "\ncouldn't update %s: %v\n", plan.RcPath, err)
		printPathHintFallback(plan, pw.Dir)
		return
	}
	fmt.Printf("\n✓ Added to %s:\n  %s\n", plan.RcPath, plan.Line)
	fmt.Printf("Open a new terminal (or `source %s`) to use the tool.\n", plan.RcPath)
}

// printPathHintFallback is the "here, do it yourself" message when
// we don't or can't auto-edit: fish shells, errors, --no-fix-path,
// declined prompts, or non-TTY with no flag.
func printPathHintFallback(plan *pathfix.Plan, dir string) {
	fmt.Printf("\nInstalled to %s, but that directory isn't on your $PATH.\n", dir)
	if plan != nil && plan.Line != "" {
		fmt.Printf("Add this to %s (or your shell's rc file), then reopen the terminal:\n", plan.RcPath)
		fmt.Printf("  %s\n", plan.Line)
		return
	}
	fmt.Printf("Add this to your shell rc (~/.zshrc or ~/.bashrc), then reopen the terminal:\n")
	fmt.Printf("  export PATH=\"%s:$PATH\"\n", dir)
}

// promptYesNo reads a single line from stdin and returns true iff
// the user typed y/yes. Empty/EOF/anything else uses dflt. We use
// bufio.NewReader rather than bufio.NewScanner here because Scanner
// strips the trailing newline, which we don't care about, and the
// Reader variant is a wafer thinner.
func promptYesNo(prompt string, dflt bool) bool {
	fmt.Print(prompt)
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return dflt
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return dflt
	}
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

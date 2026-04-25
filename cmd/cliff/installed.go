package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/jmcntsh/cliff/internal/binmap"
	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/internal/install"
)

// cmdInstalled runs `cliff installed`. Prints the names of catalog
// apps cliff detects on the user's system, one per line, sorted —
// pipe-friendly so e.g. `cliff installed | xargs -n1 cliff upgrade`
// works without further parsing.
//
// Detection is the same logic that drives the TUI sidebar's
// "Installed" row: a binary on $PATH or in a manager default dir
// whose name matches the app's derived/overridden BinaryName. No
// disk state — survives external uninstalls and recognizes
// pre-cliff installs.
func cmdInstalled(args []string) int {
	if len(args) > 0 {
		fmt.Fprintln(os.Stderr, "usage: cliff installed")
		return 2
	}

	res := catalog.LoadWithFallback(catalog.LoadOptions{})
	if res.Catalog == nil {
		fmt.Fprintln(os.Stderr, "cliff: load catalog:", res.Err)
		return 1
	}

	overrides := binmap.Load()
	installed := install.InstalledAppsWithOverrides(res.Catalog.Apps, overrides)

	names := make([]string, 0, len(installed))
	for i := range res.Catalog.Apps {
		a := &res.Catalog.Apps[i]
		if installed[a.Repo] {
			names = append(names, a.Name)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Println(n)
	}
	return 0
}

// lint validates every *.toml manifest in a directory. Exit 0 if all
// manifests pass; exit 1 with details otherwise. Used locally and by
// CI in the registry repo.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmcntsh/cliff/registry/internal/manifest"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: lint <apps-dir>")
		os.Exit(2)
	}
	dir := os.Args[1]

	loaded, err := manifest.LoadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load: %v\n", err)
		os.Exit(1)
	}
	if len(loaded) == 0 {
		fmt.Fprintf(os.Stderr, "no manifests in %s\n", dir)
		os.Exit(1)
	}

	seen := make(map[string]string) // name → path
	var fails int
	for _, l := range loaded {
		base := filepath.Base(l.Path)
		expected := l.Manifest.Name + ".toml"
		if base != expected {
			fmt.Fprintf(os.Stderr, "%s: filename must match name (%q)\n", l.Path, expected)
			fails++
		}
		if prev, dup := seen[l.Manifest.Name]; dup {
			fmt.Fprintf(os.Stderr, "%s: duplicate name (also in %s)\n", l.Path, prev)
			fails++
		}
		seen[l.Manifest.Name] = l.Path
		if err := l.Manifest.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", l.Path, err)
			fails++
		}
	}

	if fails > 0 {
		fmt.Fprintf(os.Stderr, "\n%d manifest(s) failed\n", fails)
		os.Exit(1)
	}
	fmt.Printf("ok: %d manifest(s)\n", len(loaded))
}

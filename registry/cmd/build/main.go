// build compiles every manifest in <apps-dir> into a single index.json
// suitable for the cliff client to fetch. The output schema matches
// catalog.Catalog so the client can deserialize it with the same types
// it uses for the embedded fallback.
//
// Stars and last-commit timestamps are not yet snapshotted from GitHub
// here; that's a future enrichment step. For now stars=0 for every
// manifest-sourced entry and the client just won't sort them at the
// top — which is fine because the curated seed is small.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/registry/internal/manifest"
)

const schemaVersion = 1

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: build <apps-dir> <out.json>")
		os.Exit(2)
	}
	appsDir, outPath := os.Args[1], os.Args[2]

	loaded, err := manifest.LoadDir(appsDir)
	if err != nil {
		die("load: %v", err)
	}

	var (
		apps    []catalog.App
		fails   int
		catSeen = map[string]int{}
	)
	for _, l := range loaded {
		if err := l.Manifest.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", l.Path, err)
			fails++
			continue
		}
		app := l.Manifest.ToApp()
		apps = append(apps, app)
		catSeen[app.Category]++
	}
	if fails > 0 {
		die("%d manifest(s) failed validation; refusing to build index", fails)
	}

	cats := make([]catalog.Category, 0, len(catSeen))
	for name, n := range catSeen {
		cats = append(cats, catalog.Category{Name: name, Count: n})
	}
	sort.Slice(cats, func(i, j int) bool { return cats[i].Name < cats[j].Name })

	cat := catalog.Catalog{
		SchemaVersion: schemaVersion,
		GeneratedAt:   time.Now().UTC(),
		SourceCommit:  "registry@local",
		Apps:          apps,
		Categories:    cats,
	}

	buf, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		die("marshal: %v", err)
	}
	buf = append(buf, '\n')
	if err := os.WriteFile(outPath, buf, 0o644); err != nil {
		die("write %s: %v", outPath, err)
	}

	fmt.Printf("wrote %s (%d apps, %d categories)\n", outPath, len(apps), len(cats))
	if strings.HasSuffix(outPath, "/index.json") || strings.HasSuffix(outPath, `\index.json`) {
		// nothing extra to do; just a friendly note
	}
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// embeddedIndex is a build-time snapshot of the published registry index
// (registry.cliff.sh/index.json), shipped inside the binary so the TUI
// has something to show on first launch before the live fetch completes
// and whenever the network is unavailable.
//
// This file is refreshed automatically by the refresh-snapshot workflow
// (see .github/workflows/refresh-snapshot.yml), which opens a PR
// whenever the live registry diverges. To force a refresh outside the
// normal cadence, run the workflow manually from the Actions tab, or —
// as a one-off — curl the live file yourself:
//
//	curl -fsSL https://registry.cliff.sh/index.json | \
//	  jq -S . > internal/catalog/data/index.json
//
// The `jq -S` pass is what the workflow also does; keep the manual
// path aligned so a hand-bump doesn't immediately re-trigger the
// workflow with "unchanged data, whitespace diff."
//
//go:embed data/index.json
var embeddedIndex []byte

// Load parses the embedded registry snapshot. It's the last-resort
// fallback in LoadWithFallback.
func Load() (*Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(embeddedIndex, &c); err != nil {
		return nil, fmt.Errorf("parse embedded catalog: %w", err)
	}
	return &c, nil
}

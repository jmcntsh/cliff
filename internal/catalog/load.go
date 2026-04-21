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
// Refresh with:
//
//	curl -fsSL https://registry.cliff.sh/index.json -o internal/catalog/data/index.json
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

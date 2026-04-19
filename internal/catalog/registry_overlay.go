package catalog

import (
	_ "embed"
	"encoding/json"
)

// registryJSON is a build-time snapshot of the published index from
// jmcntsh/cliff-registry, embedded so the awesome-tuis fallback catalog
// can still show InstallSpecs for curated entries when the live registry
// (registry.cliff.sh) isn't reachable. Refresh with:
//
//	curl -fsSL https://registry.cliff.sh/index.json -o internal/catalog/data/registry.json
//
//go:embed data/registry.json
var registryJSON []byte

// overlayRegistry merges InstallSpec (and the richer manifest fields) from
// the embedded registry onto matching entries in cat. Match is by Repo;
// unmatched registry entries are ignored on purpose — this is an overlay
// to enhance the scrape, not a source of new apps.
func overlayRegistry(cat *Catalog) {
	if cat == nil || len(registryJSON) == 0 {
		return
	}
	var reg Catalog
	if err := json.Unmarshal(registryJSON, &reg); err != nil {
		return
	}
	byRepo := make(map[string]*App, len(reg.Apps))
	for i := range reg.Apps {
		byRepo[reg.Apps[i].Repo] = &reg.Apps[i]
	}
	for i := range cat.Apps {
		src, ok := byRepo[cat.Apps[i].Repo]
		if !ok {
			continue
		}
		if cat.Apps[i].InstallSpec == nil && src.InstallSpec != nil {
			cat.Apps[i].InstallSpec = src.InstallSpec
		}
		if src.Author != "" {
			cat.Apps[i].Author = src.Author
		}
		if src.Readme != "" {
			cat.Apps[i].Readme = src.Readme
		}
		if len(src.Tags) > 0 && len(cat.Apps[i].Tags) == 0 {
			cat.Apps[i].Tags = src.Tags
		}
	}
}

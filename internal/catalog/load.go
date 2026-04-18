package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed data/catalog.json
var embedded []byte

func Load() (*Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(embedded, &c); err != nil {
		return nil, fmt.Errorf("parse embedded catalog: %w", err)
	}
	return &c, nil
}

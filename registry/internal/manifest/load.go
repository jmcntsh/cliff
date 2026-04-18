package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

type Loaded struct {
	Path     string
	Manifest Manifest
}

// LoadDir reads every *.toml under dir (non-recursively) into Loaded
// entries, sorted by filename. It does not validate; callers should
// invoke Validate() on each manifest.
func LoadDir(dir string) ([]Loaded, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %q: %w", dir, err)
	}

	var out []Loaded
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		var m Manifest
		if _, err := toml.DecodeFile(path, &m); err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		out = append(out, Loaded{Path: path, Manifest: m})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

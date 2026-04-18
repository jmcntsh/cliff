package main

import (
	"os"
	"testing"
)

func TestParseReadme(t *testing.T) {
	md, err := os.ReadFile("testdata/awesome-tuis.md")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	entries := parseReadme(md)
	if len(entries) < 100 {
		t.Errorf("parsed %d entries; expected >100", len(entries))
	}

	cats := make(map[string]int)
	for _, e := range entries {
		if e.Category == "" {
			t.Errorf("entry %s/%s has empty category", e.Owner, e.Repo)
		}
		if e.Owner == "" || e.Repo == "" {
			t.Errorf("entry %s has empty owner/repo", e.Name)
		}
		cats[e.Category]++
	}
	if len(cats) < 10 {
		t.Errorf("parsed %d categories; expected >=10", len(cats))
	}

	var found bool
	for _, e := range entries {
		if e.Owner == "jesseduffield" && e.Repo == "lazygit" {
			found = true
			if e.Description == "" {
				t.Error("lazygit: empty description")
			}
			break
		}
	}
	if !found {
		t.Error("expected jesseduffield/lazygit in parsed entries")
	}
}

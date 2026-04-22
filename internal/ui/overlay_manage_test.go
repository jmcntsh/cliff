package ui

import (
	"testing"

	"github.com/jmcntsh/cliff/internal/catalog"
)

// TestManageStep_SkipsDisabled verifies that manageStep hops over
// disabled actions and clamps at the ends rather than wrapping. This
// is the load-bearing interaction for the manage picker: if the
// cursor can land on a disabled slot, Enter becomes a silent no-op
// from the user's perspective, which is exactly the "⏎ does a
// surprise thing" footgun CLAUDE.md §3 calls out.
func TestManageStep_SkipsDisabled(t *testing.T) {
	tests := []struct {
		name    string
		actions []manageAction
		from    int
		delta   int
		want    int
	}{
		{
			name: "right skips disabled middle",
			actions: []manageAction{
				{enabled: true}, {enabled: false}, {enabled: true},
			},
			from: 0, delta: +1, want: 2,
		},
		{
			name: "left skips disabled middle",
			actions: []manageAction{
				{enabled: true}, {enabled: false}, {enabled: true},
			},
			from: 2, delta: -1, want: 0,
		},
		{
			name: "right clamps at end",
			actions: []manageAction{
				{enabled: true}, {enabled: true}, {enabled: true},
			},
			from: 2, delta: +1, want: 2,
		},
		{
			name: "left clamps at start",
			actions: []manageAction{
				{enabled: true}, {enabled: true}, {enabled: true},
			},
			from: 0, delta: -1, want: 0,
		},
		{
			name: "right from enabled past trailing disabled clamps",
			actions: []manageAction{
				{enabled: true}, {enabled: false}, {enabled: false},
			},
			from: 0, delta: +1, want: 0,
		},
		{
			name:    "empty slice returns zero",
			actions: nil,
			from:    0, delta: +1, want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manageStep(tt.actions, tt.from, tt.delta)
			if got != tt.want {
				t.Errorf("manageStep(%v, from=%d, delta=%d) = %d, want %d",
					tt.actions, tt.from, tt.delta, got, tt.want)
			}
		})
	}
}

// TestManageActionsFor_DefaultCursorOnFirstEnabled ensures the cursor
// starts on Update when Update is enabled, falls through to Uninstall
// when Update isn't available, and lands on Readme as the last-resort
// default when neither is available (script-type manifests that
// forgot the upgrade + uninstall blocks). Readme is always enabled
// because it's the fallback path the picker is guaranteed to offer.
func TestManageActionsFor_DefaultCursorOnFirstEnabled(t *testing.T) {
	brewSpec := &catalog.InstallSpec{Type: "brew", Package: "foo"}
	// A script-type install with no override recipes: UpgradeShell
	// and UninstallShell both return "" for Type=script, so both
	// Update and Uninstall picker items should come out disabled.
	scriptNoRecipes := &catalog.InstallSpec{Type: "script", Command: "curl x | sh"}

	tests := []struct {
		name       string
		app        *catalog.App
		wantCursor int
		wantUpdate bool
		wantUninst bool
	}{
		{
			name:       "brew app has update + uninstall",
			app:        &catalog.App{InstallSpec: brewSpec},
			wantCursor: 0,
			wantUpdate: true,
			wantUninst: true,
		},
		{
			name:       "script-only app has just readme",
			app:        &catalog.App{InstallSpec: scriptNoRecipes},
			wantCursor: 2,
			wantUpdate: false,
			wantUninst: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions, cursor := manageActionsFor(tt.app)
			if cursor != tt.wantCursor {
				t.Errorf("cursor = %d, want %d", cursor, tt.wantCursor)
			}
			if len(actions) != 3 {
				t.Fatalf("expected 3 actions, got %d", len(actions))
			}
			if actions[0].kind != manageUpdate || actions[0].enabled != tt.wantUpdate {
				t.Errorf("Update action: got enabled=%v, want %v", actions[0].enabled, tt.wantUpdate)
			}
			if actions[1].kind != manageUninstall || actions[1].enabled != tt.wantUninst {
				t.Errorf("Uninstall action: got enabled=%v, want %v", actions[1].enabled, tt.wantUninst)
			}
			if actions[2].kind != manageReadme || !actions[2].enabled {
				t.Errorf("Readme action should always be enabled")
			}
		})
	}
}

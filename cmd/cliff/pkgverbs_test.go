package main

import (
	"reflect"
	"testing"
)

func TestParseInstallFlags(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantArgs []string
		wantMode fixPathMode
		wantVia  string
		wantErr  bool
	}{
		{
			name:     "bare package",
			args:     []string{"tetrigo"},
			wantArgs: []string{"tetrigo"},
			wantMode: fixPathPromptAuto,
		},
		{
			name:     "--fix-path before pkg",
			args:     []string{"--fix-path", "tetrigo"},
			wantArgs: []string{"tetrigo"},
			wantMode: fixPathAlwaysApply,
		},
		{
			name:     "--fix-path after pkg",
			args:     []string{"tetrigo", "--fix-path"},
			wantArgs: []string{"tetrigo"},
			wantMode: fixPathAlwaysApply,
		},
		{
			name:     "--no-fix-path",
			args:     []string{"--no-fix-path", "tetrigo"},
			wantArgs: []string{"tetrigo"},
			wantMode: fixPathNeverApply,
		},
		{
			name:     "--via space form",
			args:     []string{"--via", "brew", "chess-tui"},
			wantArgs: []string{"chess-tui"},
			wantMode: fixPathPromptAuto,
			wantVia:  "brew",
		},
		{
			name:     "--via=value form",
			args:     []string{"--via=cargo", "chess-tui"},
			wantArgs: []string{"chess-tui"},
			wantMode: fixPathPromptAuto,
			wantVia:  "cargo",
		},
		{
			name:    "--via without value errors",
			args:    []string{"--via"},
			wantErr: true,
		},
		{
			name:    "--via= with empty value errors",
			args:    []string{"--via=", "chess-tui"},
			wantErr: true,
		},
		{
			name:    "unknown flag errors",
			args:    []string{"--bogus", "tetrigo"},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotArgs, gotMode, gotVia, err := parseInstallFlags(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got args=%v mode=%v via=%v", gotArgs, gotMode, gotVia)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !reflect.DeepEqual(gotArgs, tc.wantArgs) {
				t.Errorf("args = %v, want %v", gotArgs, tc.wantArgs)
			}
			if gotMode != tc.wantMode {
				t.Errorf("mode = %v, want %v", gotMode, tc.wantMode)
			}
			if gotVia != tc.wantVia {
				t.Errorf("via = %q, want %q", gotVia, tc.wantVia)
			}
		})
	}
}

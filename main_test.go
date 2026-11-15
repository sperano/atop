package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/sperano/vigie/internal/config"
	"github.com/sperano/vigie/internal/source"
)

func TestHiddenStoreUsesDefaultLegacyFallback(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	t.Setenv("HOME", t.TempDir())
	store, err := hiddenStore("")
	if err != nil {
		t.Fatal(err)
	}
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(userConfigDir, "vigie", "hidden.json"); store.Path != want {
		t.Errorf("Path = %q, want %q", store.Path, want)
	}
	if want := filepath.Join(userConfigDir, "atop", "hidden.json"); store.LegacyPath != want {
		t.Errorf("LegacyPath = %q, want %q", store.LegacyPath, want)
	}
}

func TestHiddenStoreCustomPathHasNoLegacyFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rows.json")
	store, err := hiddenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if store.Path != path || store.LegacyPath != "" {
		t.Errorf("hiddenStore = %+v, want Path %q and no LegacyPath", store, path)
	}
}

func TestBuildSources(t *testing.T) {
	agentAndWork := []string{source.SourceKelosSession, source.SourceKelosTask, source.SourcePR, source.SourceVikunja}
	tests := []struct {
		name  string
		serve bool
		want  []string
	}{
		{"tui reads local sessions", false, append([]string{source.SourceLocal}, agentAndWork...)},
		{"serve leaves local sessions out", true, agentAndWork},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sourceNames(buildSources(config.Config{Serve: tt.serve})); !slices.Equal(got, tt.want) {
				t.Errorf("sources = %q, want %q", got, tt.want)
			}
		})
	}
}

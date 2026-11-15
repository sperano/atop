package main

import (
	"os"
	"path/filepath"
	"testing"
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

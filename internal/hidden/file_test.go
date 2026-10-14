package hidden

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestFileRoundTrip(t *testing.T) {
	t.Parallel()
	f := File{Path: filepath.Join(t.TempDir(), "nested", "dir", fileName)}
	want := []Entry{entryOf(prRow), entryOf(taskRow)}
	if err := f.Save(New(want...)); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := f.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !slices.Equal(got.Entries(), want) {
		t.Errorf("loaded %v, want %v", got.Entries(), want)
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(f.Path), tempPattern))
	if len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v", leftovers)
	}
}

func TestFileSaveEmptyWritesArray(t *testing.T) {
	t.Parallel()
	f := File{Path: filepath.Join(t.TempDir(), fileName)}
	if err := f.Save(Set{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(f.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(data)); got != "[]" {
		t.Errorf("empty set saved as %q, want []", got)
	}
}

func TestFileLoad(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string // "" leaves the file missing
		want    []Entry
		wantErr bool
	}{
		{"missing file is empty", "", nil, false},
		{"entries", `[{"source":"pr","name":"atop#3"}]`, []Entry{entryOf(prRow)}, false},
		{"null", "null", nil, false},
		{"malformed", "{", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := File{Path: filepath.Join(t.TempDir(), fileName)}
			if tt.content != "" {
				if err := os.WriteFile(f.Path, []byte(tt.content), filePerm); err != nil {
					t.Fatal(err)
				}
			}
			got, err := f.Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load error = %v, wantErr %v", err, tt.wantErr)
			}
			if !slices.Equal(got.Entries(), tt.want) {
				t.Errorf("loaded %v, want %v", got.Entries(), tt.want)
			}
		})
	}
}

func TestFileSaveReplacesExisting(t *testing.T) {
	t.Parallel()
	f := File{Path: filepath.Join(t.TempDir(), fileName)}
	for _, set := range []Set{New(entryOf(prRow), entryOf(taskRow)), New(entryOf(taskRow))} {
		if err := f.Save(set); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}
	got, err := f.Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := []Entry{entryOf(taskRow)}; !slices.Equal(got.Entries(), want) {
		t.Errorf("loaded %v, want %v", got.Entries(), want)
	}
}

func TestFileSaveFailsWhenDirIsAFile(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, nil, filePerm); err != nil {
		t.Fatal(err)
	}
	if err := (File{Path: filepath.Join(blocker, fileName)}).Save(New(entryOf(prRow))); err == nil {
		t.Error("Save under a regular file succeeded")
	}
}

func TestDefaultPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	t.Setenv("HOME", "/home/someone")
	path, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(appDir, fileName); !strings.HasSuffix(path, want) {
		t.Errorf("DefaultPath() = %q, want it to end in %q", path, want)
	}
}

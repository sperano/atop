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
		{"blank", " \n", nil, false},
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

func TestFileUpdate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		content   string // "" leaves the file missing
		change    func(Set) Set
		want      []Entry
		wantErr   bool
		wantWrite bool
	}{
		{
			"keeps what another atop hid",
			`[{"source":"vikunja","name":"#874"}]`,
			func(s Set) Set { return s.Hide(prRow) },
			[]Entry{entryOf(prRow), entryOf(taskRow)}, false, true,
		},
		{
			"no change, no write",
			`[{"source":"pr","name":"atop#3"}]`,
			func(s Set) Set { return s.Hide(prRow) },
			[]Entry{entryOf(prRow)}, false, false,
		},
		{"missing file", "", func(s Set) Set { return s.Hide(prRow) }, []Entry{entryOf(prRow)}, false, true},
		{"malformed file is left alone", "{", func(s Set) Set { return s.Hide(prRow) }, nil, true, false},
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
			got, err := f.Update(tt.change)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Update error = %v, wantErr %v", err, tt.wantErr)
			}
			if !slices.Equal(got.Entries(), tt.want) {
				t.Errorf("Update returned %v, want %v", got.Entries(), tt.want)
			}
			data, _ := os.ReadFile(f.Path)
			if written := string(data) != tt.content; written != tt.wantWrite {
				t.Errorf("file written = %v, want %v (now %q)", written, tt.wantWrite, data)
			}
		})
	}
}

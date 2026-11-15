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
		{"entries", `[{"source":"pr","name":"vigie#3"}]`, []Entry{entryOf(prRow)}, false},
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

func TestDefaultFilePaths(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	t.Setenv("HOME", "/home/someone")
	f, err := DefaultFile()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(appDir, fileName); !strings.HasSuffix(f.Path, want) {
		t.Errorf("Path = %q, want it to end in %q", f.Path, want)
	}
	if want := filepath.Join(legacyAppDir, fileName); !strings.HasSuffix(f.LegacyPath, want) {
		t.Errorf("LegacyPath = %q, want it to end in %q", f.LegacyPath, want)
	}
}

func TestFileLoadsLegacyAndMigratesOnChange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := File{
		Path:       filepath.Join(dir, appDir, fileName),
		LegacyPath: filepath.Join(dir, legacyAppDir, fileName),
	}
	if err := os.MkdirAll(filepath.Dir(f.LegacyPath), dirPerm); err != nil {
		t.Fatal(err)
	}
	legacyData := []byte("[{\"source\":\"pr\",\"name\":\"vigie#3\"}]\n")
	if err := os.WriteFile(f.LegacyPath, legacyData, filePerm); err != nil {
		t.Fatal(err)
	}

	got, err := f.Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := []Entry{entryOf(prRow)}; !slices.Equal(got.Entries(), want) {
		t.Errorf("loaded %v, want %v", got.Entries(), want)
	}
	if _, err := os.Stat(f.Path); !os.IsNotExist(err) {
		t.Fatalf("new file exists before a change: %v", err)
	}

	got, err = f.Update(func(s Set) Set { return s.Hide(taskRow) })
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{entryOf(prRow), entryOf(taskRow)}
	if !slices.Equal(got.Entries(), want) {
		t.Errorf("updated %v, want %v", got.Entries(), want)
	}
	if current, err := (File{Path: f.Path}).Load(); err != nil {
		t.Fatal(err)
	} else if !slices.Equal(current.Entries(), want) {
		t.Errorf("migrated %v, want %v", current.Entries(), want)
	}
	if data, err := os.ReadFile(f.LegacyPath); err != nil {
		t.Fatal(err)
	} else if !slices.Equal(data, legacyData) {
		t.Errorf("legacy file changed to %q", data)
	}
}

func TestFilePrefersCurrentOverLegacy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := File{
		Path:       filepath.Join(dir, appDir, fileName),
		LegacyPath: filepath.Join(dir, legacyAppDir, fileName),
	}
	if err := (File{Path: f.Path}).Save(New(entryOf(prRow))); err != nil {
		t.Fatal(err)
	}
	if err := (File{Path: f.LegacyPath}).Save(New(entryOf(taskRow))); err != nil {
		t.Fatal(err)
	}
	got, err := f.Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := []Entry{entryOf(prRow)}; !slices.Equal(got.Entries(), want) {
		t.Errorf("loaded %v, want current file %v", got.Entries(), want)
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
			"keeps what another vigie hid",
			`[{"source":"vikunja","name":"#874"}]`,
			func(s Set) Set { return s.Hide(prRow) },
			[]Entry{entryOf(prRow), entryOf(taskRow)}, false, true,
		},
		{
			"no change, no write",
			`[{"source":"pr","name":"vigie#3"}]`,
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

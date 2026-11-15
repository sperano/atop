package hidden

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Where the hidden rows live by default, under the user's config directory.
const (
	appDir       = "vigie"
	legacyAppDir = "atop"
	fileName     = "hidden.json"
)

const (
	dirPerm  = 0o755
	filePerm = 0o644
	// tempPattern names the file written before it replaces the real one.
	tempPattern = ".hidden-*.json"
	jsonIndent  = "  "
)

// DefaultPath is <user config dir>/vigie/hidden.json.
func DefaultPath() (string, error) {
	f, err := DefaultFile()
	return f.Path, err
}

// DefaultFile stores rows under vigie and falls back to atop until the first
// change writes the rows to the new path.
func DefaultFile() (File, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return File{}, err
	}
	return File{
		Path:       filepath.Join(dir, appDir, fileName),
		LegacyPath: filepath.Join(dir, legacyAppDir, fileName),
	}, nil
}

// File stores a Set as a JSON array of entries.
type File struct {
	Path       string
	LegacyPath string
}

// Load reads the set. When Path is missing, LegacyPath is read instead. A
// missing or blank file is an empty set.
func (f File) Load() (Set, error) {
	path := f.Path
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) && f.LegacyPath != "" {
		path = f.LegacyPath
		data, err = os.ReadFile(path)
	}
	if errors.Is(err, fs.ErrNotExist) {
		return Set{}, nil
	}
	if err != nil {
		return Set{}, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return Set{}, nil
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return Set{}, fmt.Errorf("%s: %w", path, err)
	}
	return New(entries...), nil
}

// Update applies change to the set as stored now, writes the result when
// it differs, and returns it. Reading first keeps the changes made since
// by another vigie or by hand; a file that cannot be read is left alone.
func (f File) Update(change func(Set) Set) (Set, error) {
	current, err := f.Load()
	if err != nil {
		return Set{}, err
	}
	next := change(current)
	if next.Equal(current) {
		return next, nil
	}
	return next, f.Save(next)
}

// Save writes the set, creating the directory if needed. It syncs a
// temporary file and renames it over the old one, so that a crash cannot
// leave half a file.
func (f File) Save(s Set) error {
	data, err := json.MarshalIndent(nonNil(s.Entries()), "", jsonIndent)
	if err != nil {
		return err
	}
	dir := filepath.Dir(f.Path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // fails harmlessly once renamed
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), filePerm); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), f.Path)
}

// nonNil makes an empty set encode as [] rather than null.
func nonNil(entries []Entry) []Entry {
	if entries == nil {
		return []Entry{}
	}
	return entries
}

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
	appDir   = "atop"
	fileName = "hidden.json"
)

const (
	dirPerm  = 0o755
	filePerm = 0o644
	// tempPattern names the file written before it replaces the real one.
	tempPattern = ".hidden-*.json"
	jsonIndent  = "  "
)

// DefaultPath is <user config dir>/atop/hidden.json.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appDir, fileName), nil
}

// File stores a Set as a JSON array of entries.
type File struct {
	Path string
}

// Load reads the set; a missing or blank file is an empty set.
func (f File) Load() (Set, error) {
	data, err := os.ReadFile(f.Path)
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
		return Set{}, fmt.Errorf("%s: %w", f.Path, err)
	}
	return New(entries...), nil
}

// Update applies change to the set as stored now, writes the result when
// it differs, and returns it. Reading first keeps the changes made since
// by another atop or by hand; a file that cannot be read is left alone.
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

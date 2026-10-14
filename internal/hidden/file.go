package hidden

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

// Load reads the set; a missing file is an empty set.
func (f File) Load() (Set, error) {
	data, err := os.ReadFile(f.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return Set{}, nil
	}
	if err != nil {
		return Set{}, err
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return Set{}, fmt.Errorf("%s: %w", f.Path, err)
	}
	return New(entries...), nil
}

// Save writes the set, creating the directory if needed. It writes a
// temporary file and renames it, so that a crash cannot leave half a file.
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

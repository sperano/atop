//go:build webdist

package web

import (
	"io/fs"
	"strings"
	"testing"
)

func TestFSIsTheBuiltUI(t *testing.T) {
	t.Parallel()
	data, err := fs.ReadFile(FS(), "index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if want := `<div id="app">`; !strings.Contains(string(data), want) {
		t.Errorf("index.html has no %s:\n%s", want, data)
	}
	if _, err := fs.Stat(FS(), ".vite/license.md"); err == nil {
		t.Error(".vite/license.md is embedded; the server would serve it")
	}
}

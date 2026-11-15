//go:build !webdist

package web

import (
	"io/fs"
	"strings"
	"testing"
)

func TestFSIsThePlaceholder(t *testing.T) {
	t.Parallel()
	data, err := fs.ReadFile(FS(), "index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if want := "built without the web UI"; !strings.Contains(string(data), want) {
		t.Errorf("index.html does not say %q:\n%s", want, data)
	}
}

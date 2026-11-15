//go:build webdist

package web

import "embed"

// filesRoot is the embedded directory that FS serves from.
const filesRoot = "dist"

// The embed directive skips dot directories, so dist/.vite (the bundle's licenses,
// shipped under /licenses in the image) is not served.
//
//go:embed dist
var files embed.FS

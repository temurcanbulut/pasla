// Package ui embeds the web interface so the whole app ships as one file.
package ui

import (
	"embed"
	"io/fs"
)

//go:embed static
var files embed.FS

// Static is the web root.
var Static, _ = fs.Sub(files, "static")

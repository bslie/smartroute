package webserver

import (
	"embed"
	"io/fs"
)

//go:embed static
var staticRoot embed.FS

func subFS() fs.FS {
	f, err := fs.Sub(staticRoot, "static")
	if err != nil {
		panic(err)
	}
	return f
}

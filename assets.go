package omnithreads

import (
	"embed"
	"io/fs"
)

//go:embed all:web
var web embed.FS

//go:embed assets/app-icon.png
var Icon []byte

func Assets() fs.FS {
	f, e := fs.Sub(web, "web")
	if e != nil {
		panic(e)
	}
	return f
}

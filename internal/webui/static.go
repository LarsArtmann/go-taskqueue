package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFS embed.FS

func staticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("webui: embed static: " + err.Error())
	}

	return http.FileServerFS(sub)
}

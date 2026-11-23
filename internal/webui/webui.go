// Package webui holds the built frontend. Run "make web" to fill dist/.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var dist embed.FS

// Handler serves the built frontend. Before the frontend has been built it
// answers with a hint instead of an empty page.
func Handler() http.Handler {
	files, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist is embedded at compile time; this cannot fail at runtime
	}
	if _, err := fs.Stat(files, "index.html"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "The frontend is not built into this binary. Run \"make build\", or use \"make dev\" and open the Vite URL.", http.StatusNotFound)
		})
	}
	return http.FileServerFS(files)
}

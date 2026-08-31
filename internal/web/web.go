// Package web serves the client.
//
// The browser is the client, and the server it talks to serves it: there is no
// separate program to install, choose between, or keep in step with a server
// version (SPEC.md §13.1). The files are embedded in the binary, so an
// instance is one executable and one data file.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// all: so the directory still matches while it holds nothing but the .gitkeep
// that keeps it in the repository. The client is a build artefact: run
// ./build.sh, or `npm run build` on its own, to fill it.
//
//go:embed all:app
var files embed.FS

// Handler serves the client, falling back to index.html for any path the
// client routes itself.
func Handler() http.Handler {
	sub, err := fs.Sub(files, "app")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	_, unbuilt := fs.Stat(sub, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(sub, path); err != nil {
			// Paths the server owns are never client routes. /mcp in
			// particular must plainly not exist while the agent interface is
			// disabled (SPEC.md §14.6), rather than answering with the shell.
			if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/mcp" {
				http.NotFound(w, r)
				return
			}
			// A client-side route such as /SOL-42: serve the shell and let the
			// client render it.
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		// The client is a build artefact, so a binary built without running the
		// client build has nothing to serve. Say so rather than 404.
		if unbuilt != nil {
			http.Error(w, "the web client has not been built: run ./build.sh",
				http.StatusInternalServerError)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

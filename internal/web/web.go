// Package web serves the client.
//
// The browser is the client, and the server it talks to serves it: there is no
// separate program to install, choose between, or keep in step with a server
// version (SPEC.md §13.1). The files are embedded in the binary, so an
// instance is one executable and one data file.
package web

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"io/fs"
	"net/http"
	"strings"
	"time"
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
	return handler(sub)
}

// asset is one file, read once at startup: its bytes, gzipped too when that
// saves anything, and a fingerprint of each.
type asset struct {
	name       string
	plain, gz  []byte
	tag, gzTag string
}

func handler(sub fs.FS) http.Handler {
	assets := map[string]*asset{}
	fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := fs.ReadFile(sub, path)
		if err != nil {
			return err
		}
		a := &asset{name: path, plain: body, tag: fingerprint(body)}
		if gz := compress(body); len(gz) < len(body)*9/10 {
			a.gz, a.gzTag = gz, fingerprint(gz)
		}
		assets[path] = a
		return nil
	})
	shell := assets["index.html"]
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a := assets[strings.TrimPrefix(r.URL.Path, "/")]
		if r.URL.Path == "/" || a == nil {
			// Paths the server owns are never client routes. /mcp in
			// particular must plainly not exist while the agent interface is
			// disabled (SPEC.md §14.6), rather than answering with the shell.
			if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/mcp" {
				http.NotFound(w, r)
				return
			}
			// A client-side route such as /SOL-42: serve the shell and let the
			// client render it.
			a = shell
		}
		// The client is a build artefact, so a binary built without running the
		// client build has nothing to serve. Say so rather than 404.
		if a == nil {
			http.Error(w, "the web client has not been built: run ./build.sh",
				http.StatusInternalServerError)
			return
		}
		// The names are fixed, not hashed, so every load asks again; the
		// fingerprint makes the answer a 304 until the binary changes.
		h := w.Header()
		h.Set("Cache-Control", "no-cache")
		h.Set("Vary", "Accept-Encoding")
		body, tag := a.plain, a.tag
		if a.gz != nil && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			body, tag = a.gz, a.gzTag
			h.Set("Content-Encoding", "gzip")
		}
		h.Set("ETag", tag)
		http.ServeContent(w, r, a.name, time.Time{}, bytes.NewReader(body))
	})
}

func fingerprint(b []byte) string {
	sum := sha256.Sum256(b)
	return `"` + base64.RawURLEncoding.EncodeToString(sum[:15]) + `"`
}

func compress(b []byte) []byte {
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	zw.Write(b)
	zw.Close()
	return buf.Bytes()
}

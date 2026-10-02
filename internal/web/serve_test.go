package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestTheClientIsCompressedAndRevalidated(t *testing.T) {
	h := handler(fstest.MapFS{
		"index.html": {Data: []byte("<!doctype html><script src=/app.js></script>")},
		"app.js":     {Data: []byte(strings.Repeat("console.log('campfire');\n", 400))},
	})
	get := func(path string, header ...string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		for i := 0; i < len(header); i += 2 {
			r.Header.Set(header[i], header[i+1])
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}

	js := get("/app.js", "Accept-Encoding", "gzip")
	if js.Header().Get("Content-Encoding") != "gzip" || js.Body.Len() >= 400*25/2 {
		t.Fatalf("app.js went out as %d bytes, encoded %q", js.Body.Len(), js.Header().Get("Content-Encoding"))
	}
	if again := get("/app.js", "Accept-Encoding", "gzip", "If-None-Match", js.Header().Get("ETag")); again.Code != http.StatusNotModified {
		t.Fatalf("a second load of an unchanged file was %d, not 304", again.Code)
	}
	if plain := get("/app.js"); plain.Header().Get("Content-Encoding") != "" || plain.Body.Len() != 400*25 {
		t.Fatal("a client that takes no gzip got something other than the file")
	}
	if shell := get("/SOL-42"); !strings.Contains(shell.Body.String(), "doctype") {
		t.Fatal("a client route did not get the shell")
	}
	if api := get("/api/nothing"); api.Code != http.StatusNotFound {
		t.Fatalf("an unknown API path was %d, not 404", api.Code)
	}

	unbuilt := httptest.NewRecorder()
	handler(fstest.MapFS{".gitkeep": {}}).ServeHTTP(unbuilt, httptest.NewRequest("GET", "/", nil))
	if unbuilt.Code != http.StatusInternalServerError {
		t.Fatalf("an unbuilt client answered %d", unbuilt.Code)
	}
}

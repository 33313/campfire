package api

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A refetch that finds nothing new is a 304 with no body, and a body worth
// compressing is compressed (SPEC.md §12.4).
func TestRepliesAreFingerprintedAndCompressed(t *testing.T) {
	get := func(body any, header ...string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/teams/1/board", nil)
		for i := 0; i < len(header); i += 2 {
			r.Header.Set(header[i], header[i+1])
		}
		w := httptest.NewRecorder()
		reply(w, r, body)
		return w
	}
	board := map[string]any{"title": strings.Repeat("a card on a board, ", 200)}

	first := get(board, "Accept-Encoding", "gzip, br")
	if first.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("a large body went out uncompressed")
	}
	zr, err := gzip.NewReader(first.Body)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.NewDecoder(zr).Decode(&back); err != nil || back["title"] != board["title"] {
		t.Fatalf("the compressed body does not decode to what was sent: %v", err)
	}

	tag := first.Header().Get("ETag")
	again := get(board, "If-None-Match", tag)
	if again.Code != http.StatusNotModified || again.Body.Len() != 0 {
		t.Fatalf("the same body asked for again: %d with %d bytes", again.Code, again.Body.Len())
	}
	changed := get(map[string]any{"title": "moved"}, "If-None-Match", tag)
	if changed.Code != http.StatusOK || changed.Header().Get("ETag") == tag {
		t.Fatal("a changed body was not sent, or kept the old fingerprint")
	}
	if b, _ := io.ReadAll(changed.Body); !strings.Contains(string(b), "moved") {
		t.Fatal("a small body should go out as it is")
	}
}

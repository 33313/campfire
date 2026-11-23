package updates

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestStageAndReplace(t *testing.T) {
	const asset = "campfire-linux-amd64"
	for _, corrupt := range []bool{false, true} {
		t.Run(fmt.Sprintf("corrupt=%t", corrupt), func(t *testing.T) {
			const before, after = "old binary", "new binary"
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1.2.3/SHA256SUMS":
					fmt.Fprintf(w, "%x  %s\n", sha256.Sum256([]byte(after)), asset)
				case "/v1.2.3/" + asset:
					if corrupt {
						fmt.Fprint(w, "damaged binary")
					} else {
						fmt.Fprint(w, after)
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			dir := t.TempDir()
			exe := filepath.Join(dir, "campfire")
			if err := os.WriteFile(exe, []byte(before), 0o755); err != nil {
				t.Fatal(err)
			}
			check := func(path, want string) {
				t.Helper()
				if got, err := os.ReadFile(path); err != nil || string(got) != want {
					t.Fatalf("%s: %q, %v; want %q", path, got, err, want)
				}
			}
			staged, err := stage(context.Background(), srv.URL+"/", exe, "v1.2.3", asset)
			check(exe, before)
			if corrupt {
				if err == nil {
					t.Fatal("accepted a download with the wrong checksum")
				}
				if files, err := os.ReadDir(dir); err != nil || len(files) != 1 {
					t.Fatalf("staged file was not cleaned up: %v, %v", files, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := Replace(exe, staged); err != nil {
				t.Fatal(err)
			}
			check(exe, after)
			check(exe+".previous", before)
		})
	}
}

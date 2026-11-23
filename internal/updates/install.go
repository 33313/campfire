package updates

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const releaseDownloads = "https://github.com/33313/campfire/releases/download/"

var releaseTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

func assetName(goos, goarch string) string {
	if goarch != "amd64" && goarch != "arm64" {
		return ""
	}
	switch goos {
	case "linux", "darwin":
		return "campfire-" + goos + "-" + goarch
	case "windows":
		return "campfire-windows-" + goarch + ".exe"
	default:
		return ""
	}
}

// Executable returns the replacement target for a standalone release. The
// directory, rather than the executable itself, must allow replacement.
func Executable(current string) (string, error) {
	if os.Getenv("CAMPFIRE_CONTAINER") != "" {
		return "", errors.New("update the container image to install a new version")
	}
	if !releaseTag.MatchString(current) || assetName(runtime.GOOS, runtime.GOARCH) == "" {
		return "", errors.New("self-update requires a supported release binary")
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(exe), ".campfire-write-*")
	if err != nil {
		return "", errors.New("the binary directory is not writable; update the binary manually")
	}
	f.Close()
	os.Remove(f.Name())
	return exe, nil
}

// Stage downloads a stable release of this program from its own repository.
// It returns a verified temporary file on the same filesystem as the target.
// The caller owns that file and must remove it if the update is abandoned.
func Stage(ctx context.Context, exe, version string) (string, error) {
	return stage(ctx, releaseDownloads, exe, version, assetName(runtime.GOOS, runtime.GOARCH))
}

func stage(ctx context.Context, downloads, exe, version, asset string) (string, error) {
	if !releaseTag.MatchString(version) || asset == "" {
		return "", errors.New("no supported release binary")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	base := downloads + version + "/"
	manifest, err := download(ctx, base+"SHA256SUMS")
	if err != nil {
		return "", err
	}
	defer manifest.Close()
	want, err := checksum(io.LimitReader(manifest, 64<<10), asset)
	if err != nil {
		return "", err
	}
	body, err := download(ctx, base+asset)
	if err != nil {
		return "", err
	}
	defer body.Close()
	f, err := os.CreateTemp(filepath.Dir(exe), ".campfire-update-*")
	if err != nil {
		return "", err
	}
	keep := false
	defer func() {
		f.Close()
		if !keep {
			os.Remove(f.Name())
		}
	}()
	hash := sha256.New()
	const maxBinary = 128 << 20
	n, err := io.Copy(io.MultiWriter(f, hash), io.LimitReader(body, maxBinary+1))
	if err != nil {
		return "", err
	}
	if n == 0 || n > maxBinary || hex.EncodeToString(hash.Sum(nil)) != want {
		return "", errors.New("release binary did not match its SHA-256 checksum")
	}
	info, err := os.Stat(exe)
	if err != nil {
		return "", err
	}
	if err := f.Chmod(info.Mode().Perm()); err != nil {
		return "", err
	}
	if err := f.Sync(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	keep = true
	return f.Name(), nil
}

func checksum(r io.Reader, asset string) (string, error) {
	s := bufio.NewScanner(r)
	var sum string
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != asset {
			continue
		}
		if b, err := hex.DecodeString(fields[0]); err != nil || len(b) != sha256.Size || sum != "" {
			return "", errors.New("invalid release checksum")
		}
		sum = strings.ToLower(fields[0])
	}
	if err := s.Err(); err != nil {
		return "", err
	}
	if sum == "" {
		return "", errors.New("the release has no checksum for this platform")
	}
	return sum, nil
}

func download(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := http.Client{CheckRedirect: func(next *http.Request, via []*http.Request) error {
		if next.URL.Scheme != "https" || len(via) >= 10 {
			return errors.New("invalid release download redirect")
		}
		return nil
	}}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("release download returned %s", res.Status)
	}
	return res.Body, nil
}

package termview

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Where the browser comes from when the machine has none. Chrome for Testing is
// Google's build made for automation: versioned, archived, and published with the
// versions listed in one document. The headless shell is the part that matters here -
// a renderer with a DevTools endpoint, a third of the size of the full browser.
const (
	versionsURL   = "https://googlechromelabs.github.io/chrome-for-testing/last-known-good-versions-with-downloads.json"
	downloadName  = "chrome-headless-shell"
	downloadStale = 30 * time.Minute // how long a download may take in total
)

// platforms maps this build to the name Chrome for Testing publishes under. The
// combinations missing from it - Linux on arm, the 32-bit and BSD targets - have no
// build to download, which the error says rather than fetching something that will
// not run.
var platforms = map[string]string{
	"linux/amd64":   "linux64",
	"darwin/amd64":  "mac-x64",
	"darwin/arm64":  "mac-arm64",
	"windows/amd64": "win64",
	"windows/386":   "win32",
}

func platformName() (string, error) {
	key := runtime.GOOS + "/" + runtime.GOARCH
	if p, ok := platforms[key]; ok {
		return p, nil
	}
	return "", fmt.Errorf("no Chrome for Testing build is published for %s: install a browser and name it with --terminal-browser", key)
}

// versionsDoc is the part of the published document this needs: the stable channel's
// version and, per platform, where its archive is.
type versionsDoc struct {
	Channels map[string]struct {
		Version   string `json:"version"`
		Downloads map[string][]struct {
			Platform string `json:"platform"`
			URL      string `json:"url"`
		} `json:"downloads"`
	} `json:"channels"`
}

// browserDir is where downloaded browsers live: beside the analysis cache, so one
// cache directory holds everything depphunter puts on disk.
func browserDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "depphunter", "browsers"), nil
}

// download fetches the stable headless shell into dir and returns the binary's path.
// An install that is already there is used as it is: the version is part of its name,
// so a new one arrives under a new name rather than overwriting a browser in use.
//
// sha256Want, when set, is the digest the archive must have. Chrome for Testing
// publishes no checksums, so there is nothing to compare against by default; the
// digest of what was fetched is reported through progress, to be pinned later.
func download(ctx context.Context, dir, baseURL, sha256Want string, progress func(string)) (string, error) {
	platform, err := platformName()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, downloadStale)
	defer cancel()

	version, archiveURL, err := latest(ctx, baseURL, platform)
	if err != nil {
		return "", err
	}
	install := filepath.Join(dir, fmt.Sprintf("%s-%s-%s", downloadName, platform, version))
	if bin, err := binaryIn(install); err == nil {
		return bin, nil // already here from an earlier run
	}

	progress(fmt.Sprintf("downloading %s %s (about 90 MB) to %s", downloadName, version, dir))
	archive, digest, err := fetch(ctx, archiveURL)
	if archive != "" {
		defer os.Remove(archive)
	}
	if err != nil {
		return "", err
	}
	if sha256Want != "" && !strings.EqualFold(digest, sha256Want) {
		return "", fmt.Errorf("%s: sha256 %s, want %s", archiveURL, digest, sha256Want)
	}
	progress("downloaded " + version + ", sha256 " + digest)

	// Unpack beside the final directory and move it into place, so an interrupted
	// download never leaves something that looks installed.
	tmp, err := os.MkdirTemp(dir, ".unpack-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	if err := unzip(archive, tmp); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, install); err != nil {
		if _, statErr := os.Stat(install); statErr != nil {
			return "", err // it is not there either: the rename really failed
		}
	}
	return binaryIn(install)
}

// latest reads the published versions document and returns the stable version and the
// archive for this platform.
func latest(ctx context.Context, baseURL, platform string) (version, archive string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("reading %s: %w", baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("%s: %s", baseURL, resp.Status)
	}
	var doc versionsDoc
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&doc); err != nil {
		return "", "", fmt.Errorf("%s: %w", baseURL, err)
	}
	stable, ok := doc.Channels["Stable"]
	if !ok {
		return "", "", fmt.Errorf("%s: no stable channel", baseURL)
	}
	for _, d := range stable.Downloads[downloadName] {
		if d.Platform == platform {
			return stable.Version, d.URL, nil
		}
	}
	return "", "", fmt.Errorf("%s: no %s for %s", baseURL, downloadName, platform)
}

// fetch downloads url to a temporary file and returns its path and sha256.
func fetch(ctx context.Context, url string) (path, digest string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("downloading %s: %s", url, resp.Status)
	}
	f, err := os.CreateTemp("", "depphunter-browser-*.zip")
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	sum := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, sum), resp.Body); err != nil {
		return f.Name(), "", fmt.Errorf("downloading %s: %w", url, err)
	}
	return f.Name(), hex.EncodeToString(sum.Sum(nil)), nil
}

// unzip unpacks archive into dir. Entry names are checked rather than trusted: an
// archive naming "../" would otherwise write outside the directory it was given.
func unzip(archive, dir string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		name := filepath.Clean(filepath.FromSlash(f.Name))
		if name == "." || strings.HasPrefix(name, "..") || filepath.IsAbs(name) ||
			strings.Contains(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("%s: unsafe path %q", archive, f.Name)
		}
		target := filepath.Join(dir, name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := writeEntry(f, target); err != nil {
			return err
		}
	}
	return nil
}

func writeEntry(f *zip.File, target string) error {
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	// The archive carries the executable bit, which is the whole point for the binary.
	mode := f.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, io.LimitReader(src, 1<<30))
	return err
}

// binaryIn finds the browser inside an unpacked install.
func binaryIn(dir string) (string, error) {
	want := downloadName
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	var found string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == want {
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("%s: no %s inside", dir, want)
	}
	return found, nil
}

// errNoBrowser is what findBrowser wraps, so Run can tell "none installed" - which a
// download can fix - from a browser that was named and did not work.
var errNoBrowser = errors.New("no Chromium-based browser")

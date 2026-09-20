package termview

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// publisher stands in for Chrome for Testing: the versions document and the archive
// it points at, served from one test server.
func publisher(t *testing.T, entries map[string]string) (baseURL, digest string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range entries {
		mode := os.FileMode(0o644)
		if strings.HasSuffix(name, downloadName) || strings.HasSuffix(name, downloadName+".exe") {
			mode = 0o755
		}
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(mode)
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	archive := buf.Bytes()
	sum := sha256.Sum256(archive)

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	platform, err := platformName()
	if err != nil {
		t.Skipf("no published build for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	mux.HandleFunc("/versions.json", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"channels":{"Stable":{"version":"131.0.6778.85","downloads":{
			"chrome":[{"platform":%q,"url":"%s/chrome.zip"}],
			"chrome-headless-shell":[{"platform":%q,"url":"%s/shell.zip"}]}}}}`,
			platform, srv.URL, platform, srv.URL)
	})
	mux.HandleFunc("/shell.zip", func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
	return srv.URL + "/versions.json", hex.EncodeToString(sum[:])
}

func shellEntries() map[string]string {
	name := downloadName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return map[string]string{
		downloadName + "-" + runtime.GOOS + "/" + name: "#!/bin/sh\nexit 0\n",
		downloadName + "-" + runtime.GOOS + "/LICENSE": "license",
	}
}

func TestDownload(t *testing.T) {
	base, digest := publisher(t, shellEntries())
	dir := t.TempDir()
	var messages []string
	bin, err := download(context.Background(), dir, base, "", func(m string) { messages = append(messages, m) })
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(bin) != downloadName && filepath.Base(bin) != downloadName+".exe" {
		t.Errorf("binary is %s", bin)
	}
	info, err := os.Stat(bin)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Error("the browser was unpacked without its executable bit")
	}
	// The version is part of the directory name, so a new browser never overwrites
	// one that is running.
	if !strings.Contains(bin, "131.0.6778.85") {
		t.Errorf("%s does not carry the version", bin)
	}
	// The digest of what was fetched is reported, since nothing publishes one to
	// compare against.
	if !strings.Contains(strings.Join(messages, "\n"), digest) {
		t.Errorf("messages %q do not name the digest %s", messages, digest)
	}
}

func TestDownloadReusesWhatIsThere(t *testing.T) {
	base, _ := publisher(t, shellEntries())
	dir := t.TempDir()
	first, err := download(context.Background(), dir, base, "", func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(filepath.Dir(first), "marker")
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := download(context.Background(), dir, base, "", func(m string) {
		t.Errorf("downloaded again: %s", m)
	})
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Errorf("got %s, want the install already there (%s)", second, first)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("the existing install was replaced")
	}
}

func TestDownloadChecksTheDigest(t *testing.T) {
	base, digest := publisher(t, shellEntries())
	dir := t.TempDir()
	_, err := download(context.Background(), dir, base, strings.Repeat("00", 32), func(string) {})
	if err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("error %v, want a digest mismatch", err)
	}
	// Nothing is left behind for a later run to trust.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".unpack-") {
			t.Errorf("%s was installed although the digest was wrong", e.Name())
		}
	}
	if _, err := download(context.Background(), dir, base, digest, func(string) {}); err != nil {
		t.Errorf("the right digest was rejected: %v", err)
	}
}

func TestUnzipRejectsEscapingPaths(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("../escaped")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte("nope"))
	zw.Close()
	archive := filepath.Join(dir, "a.zip")
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := unzip(archive, filepath.Join(dir, "out")); err == nil {
		t.Fatal("an archive writing outside its directory was accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "escaped")); err == nil {
		t.Error("the archive wrote outside the directory it was given")
	}
}

func TestHeadlessShellFlags(t *testing.T) {
	// The shell is headless already and rejects being asked to be.
	shell := strings.Join(browserArgs("/opt/chrome-headless-shell", "/tmp/p", 800, 600, false), " ")
	if strings.Contains(shell, "--headless") {
		t.Errorf("the headless shell was asked to go headless: %s", shell)
	}
	full := strings.Join(browserArgs("/usr/bin/chromium", "/tmp/p", 800, 600, false), " ")
	if !strings.Contains(full, "--headless=new") {
		t.Errorf("the full browser was not asked to go headless: %s", full)
	}
	for _, both := range []string{shell, full} {
		if !strings.Contains(both, "--remote-debugging-pipe") {
			t.Errorf("no debugging pipe: %s", both)
		}
	}
}

func TestPlatformName(t *testing.T) {
	name, err := platformName()
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64":
		if err != nil || name != "linux64" {
			t.Errorf("got %q, %v", name, err)
		}
	case "linux/arm64":
		// Chrome for Testing publishes no build for it, which has to be said rather
		// than fetched anyway.
		if err == nil {
			t.Errorf("got %q for a platform with no build", name)
		}
	}
}

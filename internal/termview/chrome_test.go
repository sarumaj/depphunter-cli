package termview

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestFindBrowser(t *testing.T) {
	missing := func(string) (string, error) { return "", errors.New("not found") }
	noFile := func(string) (os.FileInfo, error) { return nil, fs.ErrNotExist }

	onPath := func(bins ...string) func(string) (string, error) {
		return func(b string) (string, error) {
			for _, x := range bins {
				if x == b {
					return "/usr/bin/" + b, nil
				}
			}
			return "", errors.New("not found")
		}
	}
	// Chromium is preferred over the other engines, but any of them will do.
	if got, err := findBrowser(onPath("google-chrome", "chromium"), noFile); err != nil || got != "/usr/bin/chromium" {
		t.Errorf("got %q, %v", got, err)
	}
	if got, err := findBrowser(onPath("brave-browser"), noFile); err != nil || got != "/usr/bin/brave-browser" {
		t.Errorf("got %q, %v", got, err)
	}
	// Nothing on PATH: the fixed install locations are tried next.
	found := func(want string) func(string) (os.FileInfo, error) {
		return func(p string) (os.FileInfo, error) {
			if p == want {
				return nil, nil
			}
			return nil, fs.ErrNotExist
		}
	}
	if len(browsers.fixed) > 0 {
		if got, err := findBrowser(missing, found(browsers.fixed[0])); err != nil || got != browsers.fixed[0] {
			t.Errorf("got %q, %v", got, err)
		}
	}
	_, err := findBrowser(missing, noFile)
	if err == nil || !strings.Contains(err.Error(), "--terminal-browser") {
		t.Errorf("error %v does not say how to name a browser", err)
	}
}

func TestBrowserArgs(t *testing.T) {
	args := strings.Join(browserArgs("/tmp/profile", 800, 600, false), " ")
	for _, want := range []string{
		"--headless=new",
		"--remote-debugging-pipe", // no debugging port for others to connect to
		"--user-data-dir=/tmp/profile",
		"--enable-unsafe-swiftshader", // WebGL without a GPU
		"--window-size=800,600",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("missing %s in %s", want, args)
		}
	}
	if strings.Contains(args, "--no-sandbox") {
		t.Error("the sandbox is off although we are not root")
	}
	if !strings.Contains(strings.Join(browserArgs("/tmp/p", 1, 1, true), " "), "--no-sandbox") {
		t.Error("as root the browser will not start with its sandbox on")
	}
}

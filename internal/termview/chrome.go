package termview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// browsers holds the binaries to look for on PATH, in order of preference, and the
// fixed locations to try when none of them is on PATH (macOS and Windows install
// browsers outside it).
var browsers = struct {
	path  []string
	fixed []string
}{
	path: []string{
		"chromium", "chromium-browser", "google-chrome", "google-chrome-stable",
		"chrome", "brave-browser", "microsoft-edge", "thorium-browser",
	},
	fixed: fixedBrowserPaths(),
}

func fixedBrowserPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		}
	case "windows":
		var paths []string
		for _, dir := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LocalAppData")} {
			if dir == "" {
				continue
			}
			paths = append(paths,
				filepath.Join(dir, `Google\Chrome\Application\chrome.exe`),
				filepath.Join(dir, `Chromium\Application\chrome.exe`),
				filepath.Join(dir, `Microsoft\Edge\Application\msedge.exe`),
			)
		}
		return paths
	}
	return nil
}

// findBrowser returns the Chromium-based browser to drive. Only Chromium engines are
// looked for: the DevTools protocol this package speaks is theirs.
func findBrowser(lookPath func(string) (string, error), stat func(string) (os.FileInfo, error)) (string, error) {
	for _, name := range browsers.path {
		if p, err := lookPath(name); err == nil {
			return p, nil
		}
	}
	for _, p := range browsers.fixed {
		if _, err := stat(p); err == nil {
			return p, nil
		}
	}
	return "", errors.New("no Chromium-based browser found: install chromium or google-chrome, " +
		"or name one with --terminal-browser")
}

// browserArgs are the flags the headless browser is started with. profileDir is a
// throw-away user data directory, so the user's own profile is never touched.
func browserArgs(profileDir string, width, height int, root bool) []string {
	args := []string{
		"--headless=new",
		"--remote-debugging-pipe",
		"--user-data-dir=" + profileDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
		"--disable-sync",
		"--disable-background-networking",
		"--mute-audio",
		"--hide-scrollbars",
		// The map is WebGL, and a headless browser usually has no GPU to draw it
		// with: this allows the software renderer recent Chromium versions refuse
		// to fall back to unless asked. A machine that does have a GPU still uses it.
		"--enable-unsafe-swiftshader",
		fmt.Sprintf("--window-size=%d,%d", width, height),
	}
	if root {
		// Chromium refuses to start as root with its sandbox on, which is how it is
		// usually run inside a container.
		args = append(args, "--no-sandbox")
	}
	return args
}

// browserProcess is the headless browser and the DevTools connection to it.
type browserProcess struct {
	cmd     *exec.Cmd
	client  *client
	profile string
	exited  chan struct{}
}

// startBrowser launches bin headless and connects to it. The page to show is not
// passed on the command line: the URL carries the server's token, and the command line
// of a process is readable by anything on the machine.
func startBrowser(ctx context.Context, bin string, width, height int,
	onEvent func(sessionID, method string, params json.RawMessage)) (*browserProcess, error) {

	profile, err := os.MkdirTemp("", "depphunter-term-")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, browserArgs(profile, width, height, os.Geteuid() == 0)...)
	cmd.Stdout, cmd.Stderr = nil, nil // the browser's chatter would corrupt the frames
	out, in, err := pipes(cmd)
	if err != nil {
		os.RemoveAll(profile)
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		os.RemoveAll(profile)
		return nil, fmt.Errorf("%s: %w", bin, err)
	}
	// The child holds its own ends now; keeping ours open would hide its exit.
	for _, f := range cmd.ExtraFiles {
		f.Close()
	}
	cmd.ExtraFiles = nil

	b := &browserProcess{cmd: cmd, profile: profile, exited: make(chan struct{})}
	b.client = newClient(out, in, onEvent)
	go func() {
		cmd.Wait()
		close(b.exited)
	}()
	return b, nil
}

// open creates a page showing url and attaches to it, returning the session to send
// page commands in. The size is not set here - a target only takes one when it opens
// its own window - but through the device metrics the view overrides anyway.
func (b *browserProcess) open(ctx context.Context, url string) (string, error) {
	var created struct {
		TargetID string `json:"targetId"`
	}
	if err := b.client.call(ctx, "", "Target.createTarget", map[string]any{"url": url}, &created); err != nil {
		return "", err
	}
	var attached struct {
		SessionID string `json:"sessionId"`
	}
	err := b.client.call(ctx, "", "Target.attachToTarget",
		map[string]any{"targetId": created.TargetID, "flatten": true}, &attached)
	return attached.SessionID, err
}

// stop closes the browser and removes its throw-away profile.
func (b *browserProcess) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	b.client.call(ctx, "", "Browser.close", nil, nil) // killed below if it will not go
	select {
	case <-b.exited:
	case <-ctx.Done():
		if b.cmd.Process != nil {
			b.cmd.Process.Kill()
		}
		<-b.exited
	}
	b.client.close()
	os.RemoveAll(b.profile)
}

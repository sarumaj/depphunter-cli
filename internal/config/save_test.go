package config

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveUIPreservesTheRest(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, ProjectFile)
	write(t, file, "# project settings\nexclude: [testdata] # keep fixtures out\nui:\n  theme: dark # my eyes\n  path_filter: old/**\n")

	ui := Default().UI
	ui.Theme, ui.ColorBy, ui.ExpandDepth = "light", "size", 3
	ui.HideLanguages = []string{"Markdown"}
	if err := SaveUI(file, ui); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(file)
	out := string(data)
	for _, want := range []string{"# project settings", "exclude: [testdata] # keep fixtures out", "theme: light # my eyes", "color_by: size", "expand_depth: 3", "- Markdown"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "path_filter") {
		t.Errorf("cleared filter still saved:\n%s", out)
	}

	cfg, err := Load([]string{root}, func(string) string { return "" }, "", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.Theme != "light" || cfg.UI.ExpandDepth != 3 || len(cfg.UI.HideLanguages) != 1 || len(cfg.Exclude) != 1 {
		t.Errorf("round trip: %+v", cfg)
	}
	if cfg.ConfigFile != file {
		t.Errorf("ConfigFile = %q", cfg.ConfigFile)
	}
}

func TestSaveUICreatesFileAndRejectsBadValues(t *testing.T) {
	file := filepath.Join(t.TempDir(), ProjectFile)
	if err := SaveUI(file, Default().UI); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(file); !strings.HasPrefix(string(data), "ui:\n") {
		t.Errorf("new file:\n%s", data)
	}
	bad := Default().UI
	bad.Theme = "neon"
	if err := SaveUI(file, bad); err == nil {
		t.Error("invalid theme saved")
	}
}

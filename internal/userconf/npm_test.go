package userconf

import (
	"path/filepath"
	"testing"
)

// Yarn Berry's file in the home directory takes the name YARN_RC_FILENAME gives
// it; Bun's global bunfig is $XDG_CONFIG_HOME's when it exists, else the home's.
//
// Verifies: REQ-SUP-064
func TestYarnAndBunLocations(t *testing.T) {
	home, xdg := t.TempDir(), t.TempDir()
	m := machine(t, home, "linux", nil)
	if got := m.YarnUserConfig(); got != filepath.Join(home, ".yarnrc.yml") {
		t.Errorf("yarnrc: %s", got)
	}
	if got := m.YarnClassicConfig(); got != filepath.Join(home, ".yarnrc") {
		t.Errorf("yarn 1: %s", got)
	}
	m = machine(t, home, "linux", map[string]string{"YARN_RC_FILENAME": ".ci.yml", "XDG_CONFIG_HOME": xdg})
	if got := m.YarnUserConfig(); got != filepath.Join(home, ".ci.yml") {
		t.Errorf("YARN_RC_FILENAME: %s", got)
	}
	if got := m.BunConfig(); got != filepath.Join(home, ".bunfig.toml") {
		t.Errorf("bunfig without an XDG one: %s", got)
	}
	touch(t, filepath.Join(xdg, ".bunfig.toml"))
	if got := m.BunConfig(); got != filepath.Join(xdg, ".bunfig.toml") {
		t.Errorf("bunfig in XDG_CONFIG_HOME: %s", got)
	}
	m = machine(t, home, "linux", map[string]string{"YARN_RC_FILENAME": "../escape.yml"})
	if got := m.YarnRCFilename(); got != ".yarnrc.yml" {
		t.Errorf("a path taken for a file name: %s", got)
	}
}

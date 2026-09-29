package userconf

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// directoryIn makes directory (and its parents) under base.
func directoryIn(t *testing.T, base string, elements ...string) string {
	t.Helper()
	directory := filepath.Join(append([]string{base}, elements...)...)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	return directory
}

// The directories above the analyzed one split at the top of its checkout: those up
// to the nearest directory holding a .git (a directory, or a worktree's file) are
// the repository's, the rest this machine's; a .git in the analyzed directory
// itself, or none at all, leaves every directory above it outside.
//
// Verifies: REQ-SUP-079
func TestDirectoriesAbove(t *testing.T) {
	base := t.TempDir()
	outer := directoryIn(t, base, "outer")
	checkout := directoryIn(t, outer, "checkout")
	directoryIn(t, checkout, ".git")
	sub := directoryIn(t, checkout, "sub")
	project := directoryIn(t, sub, "project")

	m := machine(t, "", "linux", nil)
	if repository, outside := m.DirectoriesAbove(); repository != nil || outside != nil {
		t.Errorf("without a directory: %v %v", repository, outside)
	}
	m.Directory = project
	repository, outside := m.DirectoriesAbove()
	if !reflect.DeepEqual(repository, []string{sub, checkout}) {
		t.Errorf("repository: %v", repository)
	}
	if len(outside) < 2 || outside[0] != outer || outside[1] != base || outside[len(outside)-1] != filepath.Dir(outside[len(outside)-1]) {
		t.Errorf("outside: %v", outside)
	}

	m.Directory = checkout
	if repository, outside := m.DirectoriesAbove(); repository != nil || len(outside) == 0 || outside[0] != outer {
		t.Errorf("the checkout's top: %v %v", repository, outside)
	}

	worktree := directoryIn(t, base, "worktree", "inner")
	touch(t, filepath.Join(base, "worktree", ".git"))
	m.Directory = worktree
	if repository, _ := m.DirectoriesAbove(); !reflect.DeepEqual(repository, []string{filepath.Join(base, "worktree")}) {
		t.Errorf("a worktree's .git file: %v", repository)
	}
}

// Yarn's files: the one YARN_RC_FILENAME names in each directory above the analyzed
// one outside its checkout, closest first, then the home's .yarnrc.yml when the
// walk did not reach it under that name.
//
// Verifies: REQ-SUP-079, REQ-SUP-064
func TestYarnConfigs(t *testing.T) {
	base := t.TempDir()
	outer := directoryIn(t, base, "outer")
	checkout := directoryIn(t, outer, "checkout")
	directoryIn(t, checkout, ".git")
	project := directoryIn(t, checkout, "project")
	for _, name := range []string{
		filepath.Join(base, ".ci.yml"), filepath.Join(outer, ".ci.yml"), filepath.Join(outer, ".yarnrc.yml"),
		filepath.Join(checkout, ".ci.yml"), filepath.Join(checkout, ".yarnrc.yml"),
	} {
		touch(t, name)
	}
	m := machine(t, outer, "linux", map[string]string{"YARN_RC_FILENAME": ".ci.yml"})
	m.Directory = project
	if got, want := m.YarnConfigs(), []string{filepath.Join(outer, ".ci.yml"), filepath.Join(base, ".ci.yml"), filepath.Join(outer, ".yarnrc.yml")}; !reflect.DeepEqual(got, want) {
		t.Errorf("YARN_RC_FILENAME: %v, want %v", got, want)
	}
	m = machine(t, outer, "linux", nil)
	m.Directory = project
	if got, want := m.YarnConfigs(), []string{filepath.Join(outer, ".yarnrc.yml")}; !reflect.DeepEqual(got, want) {
		t.Errorf("home in the walk: %v, want %v", got, want)
	}
	m.Directory = ""
	if got, want := m.YarnConfigs(), []string{filepath.Join(outer, ".yarnrc.yml")}; !reflect.DeepEqual(got, want) {
		t.Errorf("without a directory: %v, want %v", got, want)
	}
}

// NuGet's additional user files follow the user's NuGet.Config: every *.config and
// *.Config of the config directory beside it, not a NuGet.Config, in ordinal order;
// on Windows every *.config in any case, sorted without regard to case.
//
// Verifies: REQ-SUP-064
func TestNuGetAdditionalConfigs(t *testing.T) {
	home, appData := t.TempDir(), t.TempDir()
	for _, directory := range []string{filepath.Join(home, ".nuget", "NuGet", "config"), filepath.Join(appData, "NuGet", "config")} {
		for _, name := range []string{"B.config", "a.Config", "c.CONFIG", "nuget.config", "notes.txt"} {
			touch(t, filepath.Join(directory, name))
		}
		directoryIn(t, directory, "d.config")
	}
	unix := filepath.Join(home, ".nuget", "NuGet")
	want := []string{
		filepath.Join(unix, "NuGet.Config"), filepath.Join(home, ".config", "NuGet", "NuGet.Config"),
		filepath.Join(unix, "config", "B.config"), filepath.Join(unix, "config", "a.Config"), filepath.Join(unix, "config", "nuget.config"),
	}
	if got := machine(t, home, "linux", nil).NuGetConfigs(); !reflect.DeepEqual(got, want) {
		t.Errorf("linux: %v, want %v", got, want)
	}
	windows := filepath.Join(appData, "NuGet")
	want = []string{
		filepath.Join(windows, "NuGet.Config"),
		filepath.Join(windows, "config", "a.Config"), filepath.Join(windows, "config", "B.config"), filepath.Join(windows, "config", "c.CONFIG"),
	}
	if got := machine(t, home, "windows", map[string]string{"APPDATA": appData}).NuGetConfigs(); !reflect.DeepEqual(got, want) {
		t.Errorf("windows: %v, want %v", got, want)
	}
}

// NuGet reads one file per directory it walks through: elsewhere than on Windows the
// first of nuget.config, NuGet.config and NuGet.Config; those of the directories
// above the analyzed one outside its checkout are this machine's, closest first.
//
// Verifies: REQ-SUP-079
func TestNuGetConfigsAbove(t *testing.T) {
	base := t.TempDir()
	outer := directoryIn(t, base, "outer")
	checkout := directoryIn(t, outer, "checkout")
	directoryIn(t, checkout, ".git")
	project := directoryIn(t, checkout, "project")
	touch(t, filepath.Join(base, "NuGet.Config"))
	touch(t, filepath.Join(outer, "nuget.config"))
	touch(t, filepath.Join(checkout, "NuGet.Config"))
	m := machine(t, "", "linux", nil)
	m.Directory = project
	got := m.NuGetConfigsAbove()
	want := []string{filepath.Join(outer, "nuget.config"), filepath.Join(base, "NuGet.Config")}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		// A file system without regard to case finds nuget.config under any name.
		if !strings.EqualFold(got[i], want[i]) {
			t.Errorf("got %v, want %v", got, want)
		}
	}
	if got := m.NuGetConfigIn(project); got != "" {
		t.Errorf("a directory without one: %q", got)
	}
}

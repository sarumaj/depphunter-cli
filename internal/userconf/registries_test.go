package userconf

import (
	"path/filepath"
	"reflect"
	"testing"
)

// dub's user settings are DUB_HOME's, DPATH/dub's, %APPDATA%\dub's on Windows (or
// ~/.dub without %APPDATA%), else ~/.dub's; the system's are %ProgramData%\dub's
// on Windows, else /etc/dub's and /var/lib/dub's.
//
// Verifies: REQ-SUP-064
func TestDubSettings(t *testing.T) {
	home, dir, appData, programData := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	for _, tc := range []struct {
		goos string
		vars map[string]string
		user string
	}{
		{"linux", nil, filepath.Join(home, ".dub")},
		{"darwin", nil, filepath.Join(home, ".dub")},
		{"linux", map[string]string{"DUB_HOME": dir, "DPATH": appData}, dir},
		{"linux", map[string]string{"DPATH": dir}, filepath.Join(dir, "dub")},
		{"windows", map[string]string{"APPDATA": appData, "ProgramData": programData}, filepath.Join(appData, "dub")},
		{"windows", nil, filepath.Join(home, ".dub")},
	} {
		m := machine(t, home, tc.goos, tc.vars)
		want := []string{filepath.Join(tc.user, "settings.json")}
		switch {
		case tc.goos != "windows":
			want = append(want, filepath.Join(SystemRoot, "etc", "dub", "settings.json"), filepath.Join(SystemRoot, "var", "lib", "dub", "settings.json"))
		case tc.vars["ProgramData"] != "":
			want = append(want, filepath.Join(programData, "dub", "settings.json"))
		}
		if got := m.DubSettings(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s %v:\n got %v\nwant %v", tc.goos, tc.vars, got, want)
		}
	}
}

// The dists installed in ~/quicklisp, by name.
//
// Verifies: REQ-SUP-064
func TestQuicklispDists(t *testing.T) {
	home := t.TempDir()
	for _, name := range []string{"quicklisp/dists/ultralisp/distinfo.txt", "quicklisp/dists/quicklisp/distinfo.txt", "quicklisp/dists/quicklisp/releases.txt"} {
		touch(t, filepath.Join(home, filepath.FromSlash(name)))
	}
	want := []string{
		filepath.Join(home, "quicklisp", "dists", "quicklisp", "distinfo.txt"),
		filepath.Join(home, "quicklisp", "dists", "ultralisp", "distinfo.txt"),
	}
	if got := machine(t, home, "linux", nil).QuicklispDists(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := machine(t, "", "linux", nil).QuicklispDists(); got != nil {
		t.Errorf("no home: %v", got)
	}
}

// opam's root is OPAMROOT, else %LOCALAPPDATA%\opam on Windows, else ~/.opam;
// Alire's settings are ALIRE_SETTINGS_DIR's (ALR_CONFIG's for Alire 1), else
// %USERPROFILE%\.config\alire on Windows, else $XDG_CONFIG_HOME/alire
// (~/.config/alire).
//
// Verifies: REQ-SUP-064
func TestOpamAndAlireLocations(t *testing.T) {
	home, dir, local, xdg := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	for _, tc := range []struct {
		goos        string
		vars        map[string]string
		opam, alire string
	}{
		{"linux", nil, filepath.Join(home, ".opam"), filepath.Join(home, ".config", "alire")},
		{"darwin", map[string]string{"XDG_CONFIG_HOME": xdg}, filepath.Join(home, ".opam"), filepath.Join(xdg, "alire")},
		{"linux", map[string]string{"OPAMROOT": dir, "ALIRE_SETTINGS_DIR": dir, "ALR_CONFIG": local}, dir, dir},
		{"linux", map[string]string{"ALR_CONFIG": local}, filepath.Join(home, ".opam"), local},
		{"windows", map[string]string{"LOCALAPPDATA": local, "XDG_CONFIG_HOME": xdg}, filepath.Join(local, "opam"), filepath.Join(home, ".config", "alire")},
		{"windows", nil, filepath.Join(home, ".opam"), filepath.Join(home, ".config", "alire")},
	} {
		m := machine(t, home, tc.goos, tc.vars)
		if got := m.OpamRoot(); got != tc.opam {
			t.Errorf("%s %v: opam root %s, want %s", tc.goos, tc.vars, got, tc.opam)
		}
		if got := m.AlireSettingsDir(); got != tc.alire {
			t.Errorf("%s %v: alire settings %s, want %s", tc.goos, tc.vars, got, tc.alire)
		}
	}
	if m := machine(t, "", "linux", nil); m.OpamRoot() != "" || m.AlireSettingsDir() != "" {
		t.Error("no home, no paths")
	}
}

package userconf

import (
	"path/filepath"
	"reflect"
	"testing"
)

// registries.conf is CONTAINERS_REGISTRIES_CONF's file, else the user's when it
// exists, else /etc's; the drop-ins are the system's registries.conf.d and then the
// user's, each by name, or only the user's beside the user's registries.conf.
//
// Verifies: REQ-SUP-068, REQ-SUP-064
func TestRegistriesConf(t *testing.T) {
	home, xdg := t.TempDir(), t.TempDir()
	m := machine(t, home, "linux", nil)
	sys := filepath.Join(SystemRoot, "etc", "containers")
	touch(t, filepath.Join(sys, "registries.conf.d", "20-b.conf"))
	touch(t, filepath.Join(sys, "registries.conf.d", "10-a.conf"))
	touch(t, filepath.Join(sys, "registries.conf.d", "notes.txt"))
	touch(t, filepath.Join(home, ".config", "containers", "registries.conf.d", "00-user.conf"))
	main, dropIns := m.RegistriesConf()
	want := []string{
		filepath.Join(sys, "registries.conf.d", "10-a.conf"),
		filepath.Join(sys, "registries.conf.d", "20-b.conf"),
		filepath.Join(home, ".config", "containers", "registries.conf.d", "00-user.conf"),
	}
	if main != filepath.Join(sys, "registries.conf") || !reflect.DeepEqual(dropIns, want) {
		t.Errorf("system: %s %v", main, dropIns)
	}
	m.Env = func(k string) string {
		return map[string]string{"CONTAINERS_REGISTRIES_CONF": "/ci/registries.conf"}[k]
	}
	if main, dropIns = m.RegistriesConf(); main != "/ci/registries.conf" || !reflect.DeepEqual(dropIns, want) {
		t.Errorf("CONTAINERS_REGISTRIES_CONF: %s %v", main, dropIns)
	}
	m.Env = func(k string) string { return map[string]string{"XDG_CONFIG_HOME": xdg}[k] }
	touch(t, filepath.Join(xdg, "containers", "registries.conf"))
	touch(t, filepath.Join(xdg, "containers", "registries.conf.d", "x.conf"))
	if main, dropIns = m.RegistriesConf(); main != filepath.Join(xdg, "containers", "registries.conf") ||
		!reflect.DeepEqual(dropIns, []string{filepath.Join(xdg, "containers", "registries.conf.d", "x.conf")}) {
		t.Errorf("user file: %s %v", main, dropIns)
	}
}

// The Docker daemon's configuration: the rootless daemon's over /etc's on Linux,
// Docker Desktop's ~/.docker/daemon.json elsewhere, and the Windows engine's under
// %ProgramData%.
//
// Verifies: REQ-SUP-068
func TestDockerDaemonConfig(t *testing.T) {
	home, data := t.TempDir(), t.TempDir()
	m := machine(t, home, "linux", nil)
	if got := m.DockerDaemonConfig(); got != "" {
		t.Errorf("none: %q", got)
	}
	etc := filepath.Join(SystemRoot, "etc", "docker", "daemon.json")
	touch(t, etc)
	if got := m.DockerDaemonConfig(); got != etc {
		t.Errorf("rootful: %q", got)
	}
	rootless := filepath.Join(home, ".config", "docker", "daemon.json")
	touch(t, rootless)
	if got := m.DockerDaemonConfig(); got != rootless {
		t.Errorf("rootless: %q", got)
	}
	win := machine(t, home, "windows", map[string]string{"ProgramData": data})
	engine := filepath.Join(data, "docker", "config", "daemon.json")
	touch(t, engine)
	if got := win.DockerDaemonConfig(); got != engine {
		t.Errorf("windows engine: %q", got)
	}
	desktop := filepath.Join(home, ".docker", "daemon.json")
	touch(t, desktop)
	if got := win.DockerDaemonConfig(); got != desktop {
		t.Errorf("windows desktop: %q", got)
	}
	if got := machine(t, home, "darwin", nil).DockerDaemonConfig(); got != desktop {
		t.Errorf("darwin: %q", got)
	}
}

// GOAUTH sends the netrc when unset or when its list names netrc, and not when it is
// off or lists only forms that run a program.
//
// Verifies: REQ-AUTH-029
func TestGoAuthNetrc(t *testing.T) {
	for value, want := range map[string]bool{
		"": true, "netrc": true, "off": false, "git /src": false, "cmd arg": false,
		"git /src; netrc": true, "netrc;off": false, "off;netrc": false, " ; ": false,
	} {
		m := machine(t, t.TempDir(), "linux", map[string]string{"GOAUTH": value})
		if got := m.GoAuthNetrc(); got != want {
			t.Errorf("GOAUTH=%q: %v", value, got)
		}
	}
}

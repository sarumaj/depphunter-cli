package userconf

import (
	"path/filepath"
	"sort"
	"strings"
)

// RegistriesConf is where the containers tools (Podman, Buildah, Skopeo, CRI-O)
// read registries.conf(5), as containers/image finds it: main is the file
// CONTAINERS_REGISTRIES_CONF names; else the user's
// ($XDG_CONFIG_HOME/containers/registries.conf, ~/.config/containers/registries.conf)
// when it exists; else /etc/containers/registries.conf. dropIns are the *.conf files
// of the registries.conf.d directories, by name, each read over main and the ones
// before it: the system's and then the user's, or only the user's when main is the
// user's file.
//
// Implements: REQ-SUP-068
func (m Machine) RegistriesConf() (main string, dropIns []string) {
	user := join(m.xdgConfigHome(), "containers", "registries.conf")
	userDir := join(m.xdgConfigHome(), "containers", "registries.conf.d")
	dirs := []string{system("etc", "containers", "registries.conf.d"), userDir}
	switch env := m.Env("CONTAINERS_REGISTRIES_CONF"); {
	case env != "":
		main = env
	case user != "" && isFile(user):
		main, dirs = user, []string{userDir}
	default:
		main = system("etc", "containers", "registries.conf")
	}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		files, _ := filepath.Glob(filepath.Join(dir, "*.conf"))
		sort.Strings(files)
		for _, f := range files {
			if isFile(f) {
				dropIns = append(dropIns, f)
			}
		}
	}
	return main, dropIns
}

// DockerDaemonConfig is the daemon.json of the Docker Engine this machine's docker
// talks to, or "" when there is none: the rootless daemon's
// ($XDG_CONFIG_HOME/docker/daemon.json, ~/.config/docker/daemon.json) when it
// exists, else /etc/docker/daemon.json on Linux; Docker Desktop's
// ~/.docker/daemon.json on macOS and Windows, and on Windows the engine's
// %ProgramData%\docker\config\daemon.json.
//
// Implements: REQ-SUP-068
func (m Machine) DockerDaemonConfig() string {
	var candidates []string
	switch m.GOOS {
	case "windows":
		candidates = []string{join(m.Home, ".docker", "daemon.json"), join(m.Env("ProgramData"), "docker", "config", "daemon.json")}
	case "darwin", "ios":
		candidates = []string{join(m.Home, ".docker", "daemon.json")}
	default:
		candidates = []string{join(m.xdgConfigHome(), "docker", "daemon.json"), system("etc", "docker", "daemon.json")}
	}
	for _, f := range candidates {
		if f != "" && isFile(f) {
			return f
		}
	}
	return ""
}

// GoAuthNetrc reports whether the go command sends the netrc's credentials to a
// module proxy: GOAUTH (from the environment or the go env file) unset, or a
// ";"-separated list naming "netrc" and not "off". The list's other forms - "git
// <dir>" and a command - make the go command run a program for credentials, which
// depphunter does not: they contribute nothing.
//
// Implements: REQ-AUTH-029
func (m Machine) GoAuthNetrc() bool {
	value := strings.TrimSpace(m.GoEnv("GOAUTH"))
	if value == "" {
		return true
	}
	netrc := false
	for _, entry := range strings.Split(value, ";") {
		switch strings.TrimSpace(entry) {
		case "off":
			return false
		case "netrc":
			netrc = true
		}
	}
	return netrc
}

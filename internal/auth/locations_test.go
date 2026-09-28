package auth

import (
	"encoding/base64"
	"path/filepath"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// onMachine reads the credentials of a machine with home, the environment vars (listed
// and answered) and the platform goos.
func onMachine(t *testing.T, home, goos string, vars map[string]string) *Store {
	t.Helper()
	return readMachine(userconf.Machine{Home: home, GOOS: goos, Env: bundlerEnv(t, vars), Environ: environ})
}

// CARGO_HOME moves Cargo's config and credentials; a registry the environment defines
// (CARGO_REGISTRIES_<NAME>_INDEX) takes its token from CARGO_REGISTRIES_<NAME>_TOKEN,
// and overrides config.toml's index for the same name.
//
// Verifies: REQ-AUTH-008, REQ-AUTH-020
func TestCargoHomeAndEnvironmentRegistries(t *testing.T) {
	home, cargo := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(home, ".cargo", "credentials.toml"), "[registries.corp]\ntoken = \"home-token\"\n")
	writeFile(t, filepath.Join(home, ".cargo", "config.toml"), "[registries]\ncorp = { index = \"sparse+https://home.corp/index/\" }\n")
	writeFile(t, filepath.Join(cargo, "config.toml"), "[registries]\ncorp = { index = \"sparse+https://crates.corp/index/\" }\nmoved = { index = \"sparse+https://old.corp/index/\" }\n")
	writeFile(t, filepath.Join(cargo, "credentials.toml"), "[registries.corp]\ntoken = \"corp-token\"\n[registries.moved]\ntoken = \"moved-token\"\n")
	c := Read(home, bundlerEnv(t, map[string]string{
		"CARGO_HOME":                    cargo,
		"CARGO_REGISTRIES_MY_REG_INDEX": "sparse+https://env.corp/index/",
		"CARGO_REGISTRIES_MY_REG_TOKEN": "env-token",
		"CARGO_REGISTRIES_MOVED_INDEX":  "https://new.corp/index",
	}))
	for host, want := range map[string]string{
		"crates.corp": "corp-token", "env.corp": "env-token", "new.corp": "moved-token",
		"home.corp": "", "old.corp": "",
	} {
		if got := authorization(t, c, "https://"+host+"/index/1/a"); got != want {
			t.Errorf("%s: %q, want %q", host, got, want)
		}
	}
}

// npm's credentials come from the global npmrc, then the user's (NPM_CONFIG_USERCONFIG
// replacing ~/.npmrc), then npm_config_//host/:field variables, key over key.
//
// Verifies: REQ-AUTH-001, REQ-AUTH-020
func TestNpmCredentialLocations(t *testing.T) {
	home, dir := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(home, ".npmrc"), "//home.corp/:_authToken=home\n")
	global, user := filepath.Join(dir, "global-npmrc"), filepath.Join(dir, "ci-npmrc")
	writeFile(t, global, "//a.corp/:_authToken=global-a\n//b.corp/:_authToken=global-b\n//g.corp/:_authToken=global-g\n")
	writeFile(t, user, "//b.corp/:_authToken=user-b\n//a.corp/:_authToken=user-a\n")
	c := Read(home, bundlerEnv(t, map[string]string{
		"npm_config_globalconfig":             global,
		"NPM_CONFIG_USERCONFIG":               user,
		"npm_config_//a.corp/:_authToken":     "env-a",
		"NPM_CONFIG_//c.corp/:_authToken":     "env-c",
		"npm_config_//d.corp/:_auth":          base64.StdEncoding.EncodeToString([]byte("u:p")),
		"npm_config_//e.corp/:_authToken":     "${NPM_TOKEN_UNSET_IN_TEST}",
		"NOT_npm_config_//f.corp/:_authToken": "x",
	}))
	for host, want := range map[string]string{
		"a.corp": "Bearer env-a", "b.corp": "Bearer user-b", "g.corp": "Bearer global-g", "c.corp": "Bearer env-c",
		"d.corp": basicHeader("u:p"), "e.corp": "", "f.corp": "", "home.corp": "",
	} {
		if got := authorization(t, c, "https://"+host+"/pkg"); got != want {
			t.Errorf("%s: %q, want %q", host, got, want)
		}
	}
}

// NETRC names the netrc file, and ~/.netrc is then not read; on Windows ~/_netrc is
// read in preference to ~/.netrc, elsewhere ~/.netrc only.
//
// Verifies: REQ-AUTH-002, REQ-AUTH-020
func TestNetrcVariableAndWindowsName(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".netrc"), "machine dot.corp login u password p\n")
	writeFile(t, filepath.Join(home, "_netrc"), "machine underscore.corp login u password p\n")
	other := filepath.Join(t.TempDir(), "netrc")
	writeFile(t, other, "machine env.corp login u password p\n")
	for _, tc := range []struct {
		goos string
		vars map[string]string
		want string
	}{
		{"linux", map[string]string{"NETRC": other}, "env.corp"},
		{"windows", map[string]string{"NETRC": other}, "env.corp"},
		{"linux", nil, "dot.corp"},
		{"windows", nil, "underscore.corp"},
	} {
		c := onMachine(t, home, tc.goos, tc.vars)
		if len(c.basic) != 1 || c.basic[tc.want] == "" {
			t.Errorf("%s %v: %v, want %s only", tc.goos, tc.vars, c.basic, tc.want)
		}
	}
}

// Container registry credentials: REGISTRY_AUTH_FILE alone when set; else the first of
// $XDG_RUNTIME_DIR/containers/auth.json, ~/.config/containers/auth.json and Docker's
// config.json ($DOCKER_CONFIG replacing ~/.docker) that holds a registry wins.
//
// Verifies: REQ-AUTH-005, REQ-AUTH-020
func TestContainerAuthFilePrecedence(t *testing.T) {
	home, runtime, docker := t.TempDir(), t.TempDir(), t.TempDir()
	entry := func(pairs ...string) string {
		s := `{"auths":{`
		for i := 0; i < len(pairs); i += 2 {
			if i > 0 {
				s += ","
			}
			s += `"` + pairs[i] + `":{"auth":"` + base64.StdEncoding.EncodeToString([]byte(pairs[i+1])) + `"}`
		}
		return s + "}}"
	}
	writeFile(t, filepath.Join(runtime, "containers", "auth.json"), entry("shared.corp", "podman:run"))
	writeFile(t, filepath.Join(home, ".config", "containers", "auth.json"), entry("shared.corp", "podman:config", "xdg.corp", "x:y"))
	writeFile(t, filepath.Join(docker, "config.json"), entry("shared.corp", "docker:env", "docker.corp", "d:e"))
	writeFile(t, filepath.Join(home, ".docker", "config.json"), entry("home.corp", "h:d"))
	authFile := filepath.Join(t.TempDir(), "auth.json")
	writeFile(t, authFile, entry("only.corp", "o:k"))

	c := onMachine(t, home, "linux", map[string]string{"XDG_RUNTIME_DIR": runtime, "DOCKER_CONFIG": docker})
	for host, want := range map[string]string{
		"shared.corp": "podman:run", "xdg.corp": "x:y", "docker.corp": "d:e", "home.corp": "",
	} {
		if got := c.basic[host]; got != want {
			t.Errorf("%s: %q, want %q", host, got, want)
		}
	}
	if !c.Registry("docker.corp") || c.Registry("home.corp") {
		t.Errorf("registries: %v", c.registries)
	}

	c = onMachine(t, home, "linux", map[string]string{"REGISTRY_AUTH_FILE": authFile, "XDG_RUNTIME_DIR": runtime, "DOCKER_CONFIG": docker})
	if len(c.basic) != 1 || c.basic["only.corp"] != "o:k" {
		t.Errorf("REGISTRY_AUTH_FILE: %v, want only.corp only", c.basic)
	}

	// Elsewhere than Linux Podman keeps its file in ~/.config/containers.
	c = onMachine(t, home, "darwin", map[string]string{"XDG_RUNTIME_DIR": runtime})
	if got := c.basic["shared.corp"]; got != "podman:config" {
		t.Errorf("macOS: shared.corp %q", got)
	}
}

// On Windows NuGet's user configuration is %APPDATA%\NuGet\NuGet.Config.
//
// Verifies: REQ-AUTH-004, REQ-AUTH-020
func TestNuGetCredentialsOnWindows(t *testing.T) {
	home, appdata := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(appdata, "NuGet", "NuGet.Config"), `<configuration>
  <packageSources><add key="corp" value="https://nuget.corp/v3/index.json" /></packageSources>
  <packageSourceCredentials><corp>
    <add key="Username" value="build" /><add key="ClearTextPassword" value="secret" />
  </corp></packageSourceCredentials>
</configuration>`)
	c := onMachine(t, home, "windows", map[string]string{"APPDATA": appdata})
	if got := c.basic["nuget.corp"]; got != "build:secret" {
		t.Errorf("nuget.corp: %q", got)
	}
}

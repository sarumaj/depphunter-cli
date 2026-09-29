package auth

import (
	"path/filepath"
	"testing"
)

// cSpell: ignore conancenter

// Conan finds a remote's login in the home's credentials.json by the remote's
// name (rendered with the environment variables it names), else in
// CONAN_LOGIN_USERNAME_<NAME> and CONAN_PASSWORD_<NAME> ("-" as "_", upper
// case), else in CONAN_LOGIN_USERNAME and CONAN_PASSWORD. A password without a
// user is none. The login is filed by the remote's URL and is no Authorization
// header of its own; the auth_remote.py plugin is found and not run. A home
// without remotes.json has ConanCenter as "conancenter".
//
// Verifies: REQ-AUTH-035
func TestConanLogins(t *testing.T) {
	home := t.TempDir()
	conan := filepath.Join(home, ".conan2")
	writeFile(t, filepath.Join(conan, "remotes.json"), `{"remotes": [
  {"name": "acme", "url": "https://conan.acme.test/api/conan/", "verify_ssl": true},
  {"name": "my-remote", "url": "https://my.test", "verify_ssl": true},
  {"name": "templated", "url": "https://templated.test", "verify_ssl": true},
  {"name": "platform", "url": "https://platform.test", "verify_ssl": true},
  {"name": "generic", "url": "https://generic.test", "verify_ssl": true}]}`)
	writeFile(t, filepath.Join(conan, "credentials.json"), `{"credentials": [
  {"remote": "acme", "user": "first", "password": "old"},
  {"remote": "acme", "user": "mona", "password": "secret"},
  {"remote": "templated", "user": "ci", "password": "{{ os.getenv('CI_TOKEN') }}"},
  {"remote": "platform", "user": "ci", "password": "{{ platform.node() }}"}]}`)
	writeFile(t, filepath.Join(conan, "extensions", "plugins", "auth_remote.py"), "def auth_remote_plugin(remote, user=None):\n    pass\n")
	c := onMachine(t, home, "linux", map[string]string{
		"CI_TOKEN":                       `to"ken`,
		"CONAN_LOGIN_USERNAME_MY_REMOTE": "hubot",
		"CONAN_PASSWORD":                 "everywhere",
	})
	for remote, want := range map[string][2]string{
		"https://conan.acme.test/api/conan": {"mona", "secret"},
		"https://my.test/":                  {"hubot", "everywhere"},
		"https://templated.test":            {"ci", `to"ken`},
		"https://platform.test":             {},
		"https://generic.test":              {},
		"https://center2.conan.io":          {},
	} {
		user, password, ok := c.ConanLogin(remote)
		if got := [2]string{user, password}; got != want || ok != (want[0] != "") {
			t.Errorf("%s: %v %v, want %v", remote, got, ok, want)
		}
	}
	if got := authorization(t, c, "https://conan.acme.test/api/conan/v2/conans/search"); got != "" {
		t.Errorf("the login went as %q", got)
	}
	if got := c.ConanAuthPlugin(); got != filepath.Join(conan, "extensions", "plugins", "auth_remote.py") {
		t.Errorf("plugin %q", got)
	}
	other := t.TempDir()
	c = onMachine(t, home, "linux", map[string]string{"CONAN_HOME": other,
		"CONAN_LOGIN_USERNAME": "mona", "CONAN_PASSWORD_CONANCENTER": "center"})
	if user, password, ok := c.ConanLogin("https://center2.conan.io"); !ok || user != "mona" || password != "center" {
		t.Errorf("CONAN_HOME without remotes.json: %s %s %v", user, password, ok)
	}
	if _, _, ok := c.ConanLogin("https://conan.acme.test/api/conan"); ok || c.ConanAuthPlugin() != "" {
		t.Error("CONAN_HOME moves the home")
	}
	// A remotes.json without ConanCenter leaves it without a login.
	c = onMachine(t, home, "linux", map[string]string{"CONAN_LOGIN_USERNAME": "mona", "CONAN_PASSWORD": "everywhere"})
	if _, _, ok := c.ConanLogin("https://center2.conan.io"); ok {
		t.Error("ConanCenter is no remote of remotes.json")
	}
	if user, _, ok := c.ConanLogin("https://generic.test"); !ok || user != "mona" {
		t.Errorf("generic.test: %s %v", user, ok)
	}
}

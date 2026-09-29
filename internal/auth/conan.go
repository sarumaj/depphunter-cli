package auth

import (
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// conanCenter is the remote Conan configures when its home has no remotes.json.
const conanCenter = "https://center2.conan.io"

// readConan files the login this machine's Conan holds for each remote of the
// home's remotes.json (userconf.ConanHome; ConanCenter as "conancenter" when
// there is none), by the remote's URL, found as Conan finds it: the home's
// credentials.json entry for the remote's name, else CONAN_LOGIN_USERNAME_<NAME>
// and CONAN_PASSWORD_<NAME> (the name upper case, "-" as "_"), else
// CONAN_LOGIN_USERNAME and CONAN_PASSWORD. A password without a user is no
// login, as Conan refuses it. The login is a user and password, which are sent
// to nothing but the remote's authenticate endpoint, for a token (internal/index);
// it is never an Authorization header of its own. The home's auth_remote.py
// plugin, which Conan asks first, is a program and is not run (ConanAuthPlugin).
//
// Implements: REQ-AUTH-035
func (c *Store) readConan(m userconf.Machine) {
	home := m.ConanHome()
	if home == "" {
		return
	}
	if plugin := filepath.Join(home, "extensions", "plugins", "auth_remote.py"); isFile(plugin) {
		c.conanPlugin = plugin
	}
	remotes := map[string]string{"conancenter": conanCenter}
	if data, err := os.ReadFile(filepath.Join(home, "remotes.json")); err == nil {
		var doc struct {
			Remotes []struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			} `json:"remotes"`
		}
		if json.Unmarshal(data, &doc) != nil {
			return
		}
		remotes = map[string]string{}
		for _, r := range doc.Remotes {
			remotes[r.Name] = r.URL
		}
	}
	logins := conanCredentials(m, filepath.Join(home, "credentials.json"))
	for name, address := range remotes {
		login, ok := logins[name]
		if !ok {
			variable := strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
			login.user = cmp.Or(m.Environment("CONAN_LOGIN_USERNAME_"+variable), m.Environment("CONAN_LOGIN_USERNAME"))
			login.password = cmp.Or(m.Environment("CONAN_PASSWORD_"+variable), m.Environment("CONAN_PASSWORD"))
		}
		user, password := login.user, login.password
		address = conanRemoteKey(address)
		if user == "" || password == "" || address == "" {
			continue
		}
		if c.conan == nil {
			c.conan = map[string]conanLogin{}
		}
		c.conan[address] = conanLogin{user, password}
	}
}

// conanLogin is a user and password for a Conan remote.
type conanLogin struct{ user, password string }

// conanTemplateVariable is the part of Jinja2 that credentials.json is rendered
// with that is read here: a variable of the environment, {{ os.getenv("NAME") }}
// or {{ os.environ["NAME"] }}.
var conanTemplateVariable = regexp.MustCompile(`\{\{\s*os\.(?:getenv\(\s*|environ\[\s*|environ\.get\(\s*)["']([A-Za-z_][A-Za-z0-9_]*)["']\s*[\])]\s*\}\}`)

// conanCredentials reads a Conan home's credentials.json, `{"credentials":
// [{"remote", "user", "password"}]}`, by remote. Conan renders the file as a
// Jinja2 template first; the environment variables it names are substituted
// here, and an entry that still holds template syntax is left out.
func conanCredentials(m userconf.Machine, name string) map[string]conanLogin {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil
	}
	rendered := conanTemplateVariable.ReplaceAllStringFunc(string(data), func(s string) string {
		value, _ := json.Marshal(m.Environment(conanTemplateVariable.FindStringSubmatch(s)[1]))
		return strings.Trim(string(value), `"`)
	})
	var doc struct {
		Credentials []struct {
			Remote   string `json:"remote"`
			User     string `json:"user"`
			Password string `json:"password"`
		} `json:"credentials"`
	}
	if json.Unmarshal([]byte(rendered), &doc) != nil {
		return nil
	}
	out := map[string]conanLogin{}
	for _, entry := range doc.Credentials {
		if strings.Contains(entry.User+entry.Password, "{{") || strings.Contains(entry.User+entry.Password, "{%") {
			continue
		}
		out[entry.Remote] = conanLogin{entry.User, entry.Password} // the last entry for a remote, as Conan reads them
	}
	return out
}

// conanRemoteKey is how a remote's URL is compared: without surrounding space
// and trailing slashes, as internal/index records it.
func conanRemoteKey(address string) string {
	return strings.TrimRight(strings.TrimSpace(address), "/")
}

// ConanLogin is the user and password this machine's Conan configuration holds
// for a remote, by its URL (see readConan). They are for the remote's
// authenticate endpoint alone.
//
// Implements: REQ-AUTH-035
func (c *Store) ConanLogin(remote string) (user, password string, ok bool) {
	if c == nil {
		return "", "", false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	login, ok := c.conan[conanRemoteKey(remote)]
	return login.user, login.password, ok
}

// ConanAuthPlugin is the auth_remote.py plugin of this machine's Conan home, ""
// when it has none: Conan asks it for a remote's login before anything else, and
// it is a program, which is not run.
//
// Implements: REQ-AUTH-035
func (c *Store) ConanAuthPlugin() string {
	if c == nil {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conanPlugin
}

// isFile reports whether name is a file.
func isFile(name string) bool {
	info, err := os.Stat(name)
	return err == nil && !info.IsDir()
}

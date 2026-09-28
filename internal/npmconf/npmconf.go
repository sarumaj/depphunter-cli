// Package npmconf reads the registry settings of the npm-family package managers
// that do not use an npmrc: Yarn Berry's .yarnrc.yml, Yarn 1's .yarnrc and Bun's
// bunfig.toml. It says what a file names, as written; whether a value may be
// trusted, and which environment fills its ${VAR} references, is for the caller
// (internal/index for the registries, internal/auth for the credentials) to decide.
package npmconf

import (
	"encoding/base64"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// YarnDefault is the registry Yarn Berry asks when nothing names another.
const YarnDefault = "https://registry.yarnpkg.com"

// BunDefault is the registry Bun asks when nothing names another.
const BunDefault = "https://registry.npmjs.org"

// Entry is a registry and the credential a configuration keeps for it, each value
// as written (still holding its ${VAR} references).
type Entry struct {
	URL string
	// Token is sent as a Bearer token (Yarn's npmAuthToken, Bun's token).
	Token string
	// Ident is Yarn's npmAuthIdent: "user:password", or that pair base64-encoded.
	Ident string
	// Username and Password are Bun's, sent as Basic credentials.
	Username, Password string
}

// Settings are what one file says: the default registry, the registry of each
// scope (by "@scope"), and - Yarn's npmRegistries - credentials by registry URL.
type Settings struct {
	Registry Entry
	Scopes   map[string]Entry
	Hosts    map[string]Entry
}

// ParseYarnrc reads a Yarn Berry .yarnrc.yml: npmRegistryServer, npmAuthToken and
// npmAuthIdent at the top, per scope under npmScopes, and per registry under
// npmRegistries (a key written "//host/path" meaning https).
func ParseYarnrc(data []byte) (Settings, bool) {
	var doc map[string]any
	if yaml.Unmarshal(data, &doc) != nil || doc == nil {
		return Settings{}, false
	}
	s := Settings{Registry: yarnEntry(doc)}
	if scopes, ok := doc["npmScopes"].(map[string]any); ok {
		s.Scopes = map[string]Entry{}
		for name, v := range scopes {
			if m, ok := v.(map[string]any); ok && strings.TrimPrefix(name, "@") != "" {
				s.Scopes["@"+strings.TrimPrefix(name, "@")] = yarnEntry(m)
			}
		}
	}
	if hosts, ok := doc["npmRegistries"].(map[string]any); ok {
		s.Hosts = map[string]Entry{}
		for key, v := range hosts {
			m, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if strings.HasPrefix(key, "//") {
				key = "https:" + key
			}
			e := yarnEntry(m)
			e.URL = key
			s.Hosts[key] = e
		}
	}
	return s, true
}

func yarnEntry(m map[string]any) Entry {
	str := func(k string) string { v, _ := m[k].(string); return strings.TrimSpace(v) }
	return Entry{URL: str("npmRegistryServer"), Token: str("npmAuthToken"), Ident: str("npmAuthIdent")}
}

// ParseYarnClassic reads the registries of a Yarn 1 .yarnrc: `registry "url"` and
// `"@scope:registry" "url"`. Yarn 1 takes its credentials from the npmrc.
func ParseYarnClassic(data []byte) Settings {
	s := Settings{Scopes: map[string]Entry{}}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value := classicField(line)
		switch {
		case key == "registry":
			s.Registry.URL = value
		case strings.HasPrefix(key, "@") && strings.HasSuffix(key, ":registry"):
			s.Scopes[strings.TrimSuffix(key, ":registry")] = Entry{URL: value}
		}
	}
	return s
}

// classicField splits a .yarnrc line - a key and a value, either quoted - in two.
func classicField(line string) (key, value string) {
	if strings.HasPrefix(line, `"`) {
		if end := strings.Index(line[1:], `"`); end >= 0 {
			key, value = line[1:end+1], line[end+2:]
		}
	} else {
		key, value, _ = strings.Cut(line, " ")
	}
	return key, strings.Trim(strings.TrimSpace(value), `"`)
}

// ParseBunfig reads the registries of a bunfig.toml: [install] registry and each
// [install.scopes] entry, either a URL (which may carry user:password) or a table
// of url, token, username and password.
func ParseBunfig(data []byte) (Settings, bool) {
	var doc struct {
		Install struct {
			Registry any            `toml:"registry"`
			Scopes   map[string]any `toml:"scopes"`
		} `toml:"install"`
	}
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return Settings{}, false
	}
	s := Settings{Registry: bunEntry(doc.Install.Registry), Scopes: map[string]Entry{}}
	for name, v := range doc.Install.Scopes {
		if name = strings.TrimPrefix(name, "@"); name != "" {
			if e := bunEntry(v); e.URL != "" {
				s.Scopes["@"+name] = e
			}
		}
	}
	return s, true
}

// bunEntry is a Bun registry value. A user:password in the URL is moved into the
// entry's Username and Password; a password with no user name is a token, as Bun
// takes https://:token@host.
func bunEntry(v any) Entry {
	var e Entry
	switch v := v.(type) {
	case string:
		e.URL = strings.TrimSpace(v)
	case map[string]any:
		str := func(k string) string { s, _ := v[k].(string); return strings.TrimSpace(s) }
		e = Entry{URL: str("url"), Token: str("token"), Username: str("username"), Password: str("password")}
	}
	scheme, rest, ok := strings.Cut(e.URL, "://")
	host, _, _ := strings.Cut(rest, "/")
	at := strings.LastIndex(host, "@")
	if !ok || at < 0 {
		return e
	}
	e.URL = scheme + "://" + rest[at+1:]
	user, pass, _ := strings.Cut(host[:at], ":")
	switch {
	case e.Token != "" || e.Password != "":
	case user == "" && pass != "":
		e.Token = pass
	default:
		e.Username, e.Password = user, pass
	}
	return e
}

// Credentials lists the credentials a file keeps, each with the registry it is
// sent to: the top-level one to the default registry (the file's, else def), those
// of npmRegistries to their key, and a scope's to the scope's registry (else the
// default). The order is the top level, then the registries, then the scopes, each
// by name.
func (s Settings) Credentials(def string) []Entry {
	top := s.Registry.URL
	if top == "" {
		top = def
	}
	var out []Entry
	add := func(e Entry, u string) {
		if e.Token != "" || e.Ident != "" || e.Password != "" {
			e.URL = u
			out = append(out, e)
		}
	}
	add(s.Registry, top)
	for _, k := range slices.Sorted(maps.Keys(s.Hosts)) {
		add(s.Hosts[k], k)
	}
	for _, k := range slices.Sorted(maps.Keys(s.Scopes)) {
		u := s.Scopes[k].URL
		if u == "" {
			u = top
		}
		add(s.Scopes[k], u)
	}
	return out
}

// yarnVar is a Yarn environment reference: ${NAME}, ${NAME-fallback} or
// ${NAME:-fallback}.
var yarnVar = regexp.MustCompile(`\$\{(\w+)(:?-([^}]*))?\}`)

// Interpolate resolves the ${...} references of a Yarn value from env as Yarn
// does. A lookup cannot tell an empty variable from an unset one, so an empty
// one takes the fallback, and without a fallback ok is false: Yarn refuses a
// configuration naming a variable that is not set.
func Interpolate(v string, env func(string) string) (string, bool) {
	ok := true
	out := yarnVar.ReplaceAllStringFunc(v, func(ref string) string {
		m := yarnVar.FindStringSubmatch(ref)
		if value := env(m[1]); value != "" {
			return value
		}
		if m[2] != "" {
			return m[3]
		}
		ok = false
		return ""
	})
	return out, ok
}

// Reference is the variable a value consists of and nothing else - Yarn's ${NAME}
// (fallback reports a ${NAME:-x} or ${NAME-x} form), or Bun's $NAME and ${NAME} -
// and "" for any other value.
func Reference(v string) (name string, fallback bool) {
	v = strings.TrimSpace(v)
	if m := yarnVar.FindStringSubmatch(v); m != nil && m[0] == v {
		return m[1], m[2] != ""
	}
	if n, ok := strings.CutPrefix(v, "$"); ok && n != "" && strings.Trim(n, "_abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") == "" {
		return n, false
	}
	return "", false
}

// Basic is the "user:password" of an entry's Ident or Username and Password, when
// it has one.
func (e Entry) Basic() (string, bool) {
	if e.Ident != "" {
		if strings.Contains(e.Ident, ":") {
			return e.Ident, true
		}
		if pair, err := base64.StdEncoding.DecodeString(e.Ident); err == nil && strings.Contains(string(pair), ":") {
			return string(pair), true
		}
		return "", false
	}
	if e.Username != "" && e.Password != "" {
		return e.Username + ":" + e.Password, true
	}
	return "", false
}

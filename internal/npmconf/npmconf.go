// Package npmconf reads the registry settings of the npm-family package managers
// that do not use an npmrc: Yarn Berry's .yarnrc.yml, Yarn 1's .yarnrc and Bun's
// bunfig.toml. It says what a file names, as written; whether a value may be
// trusted, and which environment fills its ${VAR} references, is for the caller
// (internal/index for the registries, internal/auth for the credentials) to decide.
package npmconf

import (
	"encoding/base64"
	"fmt"
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
	// AlwaysAuth is Yarn's npmAlwaysAuth as written: "true" or "1" has the
	// credential sent with every request, not only with those for a scoped
	// package.
	AlwaysAuth string
	// Packages limits the requests a credential is sent with (see
	// YarnCredentials): "" every one, "@" those for a scoped package, "@scope"
	// those for that scope's packages.
	Packages string
}

// Settings are what one file says: the default registry, the registry of each
// scope (by "@scope"), and - Yarn's npmRegistries - credentials by registry URL.
type Settings struct {
	Registry Entry
	Scopes   map[string]Entry
	Hosts    map[string]Entry
}

// ParseYarnrc reads a Yarn Berry .yarnrc.yml: npmRegistryServer, npmAuthToken,
// npmAuthIdent and npmAlwaysAuth at the top, per scope under npmScopes, and per
// registry under npmRegistries (a key written "//host/path" meaning https).
func ParseYarnrc(data []byte) (Settings, bool) { return MergeYarnrc([][]byte{data}) }

// MergeYarnrc reads several Yarn Berry files, closest first, merged the way Yarn
// merges the files it finds: key by key at every depth, a closer file's value
// winning, so a closer npmScopes entry that sets only the token keeps a farther
// one's registry. A file that is not YAML is skipped (Yarn refuses to run); ok
// says whether any file was read. Yarn's onConflict directives are not followed.
func MergeYarnrc(files [][]byte) (Settings, bool) {
	var merged map[string]any
	for i := len(files) - 1; i >= 0; i-- {
		var doc map[string]any
		if yaml.Unmarshal(files[i], &doc) != nil || doc == nil {
			continue
		}
		merged = mergeYAML(merged, doc)
	}
	if merged == nil {
		return Settings{}, false
	}
	return yarnSettings(merged), true
}

// mergeYAML lays closer over farther: a mapping both hold is merged key by key, any
// other value of closer replaces farther's.
func mergeYAML(farther, closer map[string]any) map[string]any {
	out := maps.Clone(farther)
	if out == nil {
		out = map[string]any{}
	}
	for key, value := range closer {
		near, nearMap := value.(map[string]any)
		far, farMap := out[key].(map[string]any)
		if nearMap && farMap {
			out[key] = mergeYAML(far, near)
		} else {
			out[key] = value
		}
	}
	return out
}

// yarnSettings are the registry settings of a Yarn Berry document.
func yarnSettings(doc map[string]any) Settings {
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
	return s
}

func yarnEntry(m map[string]any) Entry {
	stringField := func(k string) string { v, _ := m[k].(string); return strings.TrimSpace(v) }
	e := Entry{URL: stringField("npmRegistryServer"), Token: stringField("npmAuthToken"), Ident: stringField("npmAuthIdent")}
	switch v := m["npmAlwaysAuth"].(type) {
	case string:
		e.AlwaysAuth = strings.TrimSpace(v)
	case bool, int:
		e.AlwaysAuth = fmt.Sprint(v)
	}
	return e
}

// YarnEnvironment lays the YARN_NPM_* variables Yarn reads over the top level of
// its files: YARN_NPM_REGISTRY_SERVER, YARN_NPM_AUTH_TOKEN, YARN_NPM_AUTH_IDENT and
// YARN_NPM_ALWAYS_AUTH. YARN_NPM_SCOPES and YARN_NPM_REGISTRIES are not read: Yarn
// takes a variable's value as a string, and refuses to run when a setting that is
// a map gets one.
func (s *Settings) YarnEnvironment(environment func(string) string) {
	for name, field := range map[string]*string{
		"YARN_NPM_REGISTRY_SERVER": &s.Registry.URL,
		"YARN_NPM_AUTH_TOKEN":      &s.Registry.Token,
		"YARN_NPM_AUTH_IDENT":      &s.Registry.Ident,
		"YARN_NPM_ALWAYS_AUTH":     &s.Registry.AlwaysAuth,
	} {
		if value := environment(name); value != "" {
			*field = value
		}
	}
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
		stringField := func(k string) string { s, _ := v[k].(string); return strings.TrimSpace(s) }
		e = Entry{URL: stringField("url"), Token: stringField("token"), Username: stringField("username"), Password: stringField("password")}
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
// sent to: the top-level one to the default registry (the file's, else defaultRegistry), those
// of npmRegistries to their key, and a scope's to the scope's registry (else the
// default). The order is the top level, then the registries, then the scopes, each
// by name.
func (s Settings) Credentials(defaultRegistry string) []Entry {
	top := s.Registry.URL
	if top == "" {
		top = defaultRegistry
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

// YarnCredentials lists the credentials of a Yarn Berry configuration as
// Credentials does for the default registry YarnDefault, each limited to the
// requests Yarn sends it with when it fetches a package's metadata (Packages). A
// scope's credential goes only with the scope's packages. The top level's, and an
// npmRegistries entry's, go with every package when its npmAlwaysAuth is true
// (resolve fills the value's variables; nil leaves them, and a value with any
// counts as false), else only with scoped packages, which Yarn always
// authenticates. The top level's credential is left out when npmRegistries has an
// entry for the default registry: Yarn then takes that entry's settings instead.
func (s Settings) YarnCredentials(resolve func(string) string) []Entry {
	top := s.Registry.URL
	if top == "" {
		top = YarnDefault
	}
	always := func(e Entry) string {
		value := e.AlwaysAuth
		if resolve != nil {
			value = resolve(value)
		}
		if value == "true" || value == "1" {
			return ""
		}
		return "@"
	}
	var out []Entry
	add := func(e Entry, u, packages string) {
		if e.Token != "" || e.Ident != "" {
			e.URL, e.Packages = u, packages
			out = append(out, e)
		}
	}
	own := false
	for key := range s.Hosts {
		own = own || sameRegistry(key, top)
	}
	if !own {
		add(s.Registry, top, always(s.Registry))
	}
	for _, k := range slices.Sorted(maps.Keys(s.Hosts)) {
		add(s.Hosts[k], k, always(s.Hosts[k]))
	}
	for _, k := range slices.Sorted(maps.Keys(s.Scopes)) {
		u := s.Scopes[k].URL
		if u == "" {
			u = top
		}
		add(s.Scopes[k], u, k)
	}
	return out
}

// sameRegistry reports whether two registry URLs name one registry as Yarn matches
// an npmRegistries key: without a trailing slash, and with or without the scheme.
func sameRegistry(a, b string) bool {
	normalize := func(u string) string {
		u = strings.TrimRight(strings.TrimSpace(u), "/")
		if _, rest, ok := strings.Cut(u, "://"); ok {
			return rest
		}
		return u
	}
	return normalize(a) == normalize(b)
}

// yarnVariable is a Yarn environment reference: ${NAME}, ${NAME-fallback} or
// ${NAME:-fallback}.
var yarnVariable = regexp.MustCompile(`\$\{(\w+)(:?-([^}]*))?\}`)

// Interpolate resolves the ${...} references of a Yarn value from env as Yarn
// does. A lookup cannot tell an empty variable from an unset one, so an empty
// one takes the fallback, and without a fallback ok is false: Yarn refuses a
// configuration naming a variable that is not set.
func Interpolate(v string, environment func(string) string) (string, bool) {
	ok := true
	out := yarnVariable.ReplaceAllStringFunc(v, func(reference string) string {
		m := yarnVariable.FindStringSubmatch(reference)
		if value := environment(m[1]); value != "" {
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
	if m := yarnVariable.FindStringSubmatch(v); m != nil && m[0] == v {
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

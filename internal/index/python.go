package index

import (
	"maps"
	"net/url"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/npmconf"
	"github.com/sarumaj/depphunter-cli/internal/pyconf"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// pythonMachine is what this machine's uv, Poetry and PDM configuration says
// beyond the sources it adds: the indexes a repository may refer to by name, the
// credentials kept by index name, and the hosts of Poetry's repositories.
type pythonMachine struct {
	uv  []pyconf.Index
	pdm pyconf.Settings
	// poetry holds Poetry's http-basic credentials by pyconf.EnvName.
	poetry map[string]pyconf.Credential
	// hosts are the hosts of this machine's Poetry repositories and explicit uv
	// indexes: neither is a source by itself (Poetry installs only from the
	// sources a pyproject.toml declares), but the machine vouches for their hosts.
	hosts map[string]bool
}

// machinePython reads the indexes of this machine's uv and PDM configuration, after
// pip's (whose index-url, when it names one, stays the replacement asked): uv's
// variables and then its user's and system's uv.toml (pyconf.UVMachine), a default
// index replacing PyPI and any other asked beside it, an explicit one serving only
// what a repository pins to it by name; PDM's global config.toml with PDM_PYPI_URL
// over it, its [pypi] url replacing PyPI and each [pypi.<name>] asked beside it.
// Poetry's config.toml repositories and POETRY_REPOSITORIES_<NAME>_URL are publish
// targets, not sources: they are kept for their hosts (see vouched) and their
// credentials.
//
// Implements: REQ-SUP-066, REQ-SUP-064
func (c *Config) machinePython(m userconf.Machine, k sink) {
	c.py = pythonMachine{hosts: map[string]bool{}}
	c.py.uv = pyconf.UVMachine(m)
	for _, i := range c.py.uv {
		if addPython(i, false, k) != "" && i.Kind == pyconf.Explicit {
			c.py.vouch(i.URL)
		}
	}
	c.py.pdm = pyconf.PDMMachine(m)
	for _, i := range c.py.pdm.Indexes {
		addPython(i, false, k)
	}
	var repos map[string]string
	repos, c.py.poetry = pyconf.PoetryMachine(m)
	for _, r := range repos {
		c.py.vouch(r)
	}
}

// vouch records the host of an index this machine names without it being a source.
func (p *pythonMachine) vouch(index string) {
	if u, err := url.Parse(strings.TrimSpace(index)); err == nil && u.Host != "" {
		p.hosts[strings.ToLower(u.Host)] = true
	}
}

// addPython records one index as the source its kind makes it: Default replaces
// PyPI (or, listed, joins Poetry's ordered primary sources), Primary is one of
// those, Extra is asked beside PyPI and, for each PDM include_packages pattern,
// serves the matching packages alone. An explicit index serves only what is
// pinned to it and is not a source by itself. It answers the URL recorded.
func addPython(i pyconf.Index, listed bool, k sink) string {
	u := strings.TrimSpace(i.URL)
	if u == "" {
		return ""
	}
	switch {
	case i.Kind == pyconf.Primary, i.Kind == pyconf.Default && listed:
		k.put(PyPI, Source{URL: u, Kind: Listed})
	case i.Kind == pyconf.Default:
		k.put(PyPI, Source{URL: u})
	case i.Kind == pyconf.Extra:
		for _, p := range i.Include {
			k.put(PyPI, Source{URL: u, Scope: p})
		}
		k.extra(PyPI, u)
	}
	return u
}

// pyTool says how one tool's repository configuration is read.
type pyTool struct {
	// listed makes a default index one of the ordered primary sources (Poetry).
	listed bool
	// credential is this machine's credential for an index of the tool by name.
	credential func(name string) (user, pass string)
	// fallback finds an index a pin names that the file does not define.
	fallback func(name string) (pyconf.Index, bool)
}

// projectPython prepares the reading of what the repository's pyproject.toml,
// uv.toml, Pipfile, Pipfile.lock and pdm.toml files say for uv, Poetry, Pipenv and
// PDM, and answers the reader of one file, which reports whether the file is one of
// those. A directory's uv.toml replaces the [tool.uv] indexes of the pyproject.toml
// beside it, as in uv, unless UV_NO_CONFIG or UV_CONFIG_FILE keeps uv from reading
// project files; the packages [tool.uv.sources] pins are resolved against the
// directory's indexes, then this machine's.
//
// Implements: REQ-SUP-066, REQ-AUTH-023
func (c *Config) projectPython(files []*scan.File) func(f *scan.File, base string, data []byte, k sink) bool {
	env := c.m.Env
	if env == nil {
		env = func(string) string { return "" }
	}
	m := userconf.Machine{Env: env}
	uvToml := map[string]pyconf.Settings{}
	if m.UVProjectConfig() {
		for _, f := range files {
			if strings.ToLower(path.Base(f.Path)) != "uv.toml" {
				continue
			}
			if data, err := os.ReadFile(f.Abs); err == nil {
				if s, ok := pyconf.UV(data, false); ok {
					uvToml[path.Dir(f.Path)] = s
				}
			}
		}
	}
	uv := pyTool{
		credential: func(name string) (string, string) { return pyconf.UVCredential(env, name) },
		fallback: func(name string) (pyconf.Index, bool) {
			return pyconf.Settings{Indexes: c.py.uv}.Named(name)
		},
	}
	poetry := pyTool{listed: true, credential: func(name string) (string, string) {
		cred := pyconf.PoetryCredential(m, c.py.poetry, name)
		return cred.Username, cred.Password
	}}
	pdm := pyTool{credential: func(name string) (string, string) {
		i, _ := c.py.pdm.Named(name)
		return i.Username, i.Password
	}}
	return func(f *scan.File, base string, data []byte, k sink) bool {
		switch base {
		case "uv.toml":
			if s, ok := uvToml[path.Dir(f.Path)]; ok {
				c.pythonSettings(s, uv, k)
			}
		case "pyproject.toml":
			if s, ok := pyconf.Poetry(data); ok {
				c.pythonSettings(s, poetry, k)
			}
			if s, ok := pyconf.PDM(data); ok {
				c.pythonSettings(s, pdm, k)
			}
			s, ok := pyconf.UV(data, true)
			if dir, replaced := uvToml[path.Dir(f.Path)]; ok && replaced {
				s.Indexes = dir.Indexes // recorded with uv.toml; only the pins are read here
				c.pythonPins(s, uv, k)
			} else if ok {
				c.pythonSettings(s, uv, k)
			}
		case "pipfile":
			if s, ok := pyconf.Pipfile(data); ok {
				c.pythonSettings(s, pyTool{}, k)
			}
		case "pipfile.lock":
			if s, ok := pyconf.PipfileLock(data); ok {
				c.pythonSettings(s, pyTool{}, k)
			}
		case "pdm.toml":
			if s, ok := pyconf.PDMConfig(data); ok {
				c.pythonSettings(s, pdm, k)
			}
		default:
			return false
		}
		return true
	}
}

// pythonSettings records the indexes of one repository file, lends the credentials
// this machine holds for them (see lendPython), and records its pins.
func (c *Config) pythonSettings(s pyconf.Settings, tool pyTool, k sink) {
	for i := range s.Indexes {
		idx := &s.Indexes[i]
		if pyconf.PoetryPyPI(*idx) {
			idx.URL = public[PyPI]
		}
		clean, user, pass := splitUserinfo(idx.URL)
		if strings.Contains(clean, "$") {
			idx.URL = "" // a URL made of this machine's variables is not drawn on the map
			continue
		}
		idx.URL = clean
		if addPython(*idx, tool.listed, k) == "" {
			continue
		}
		if u, p, ok := c.referencedCredential(user, pass); ok {
			c.lendPython(clean, u, p)
		} else if u, p, ok := c.referencedCredential(idx.Username, idx.Password); ok {
			c.lendPython(clean, u, p)
		} else if tool.credential != nil && idx.Name != "" {
			if u, p := tool.credential(idx.Name); u != "" || p != "" {
				c.lendPython(clean, u, p)
			}
		}
	}
	c.pythonPins(s, tool, k)
}

// pythonPins records the packages a file pins to an index by name, each served by
// that index alone. A name the file does not define is looked up in this machine's
// configuration (uv); an index this machine names stays trusted.
func (c *Config) pythonPins(s pyconf.Settings, tool pyTool, k sink) {
	for _, pkg := range slices.Sorted(maps.Keys(s.Pins)) {
		name := s.Pins[pkg]
		if i, ok := s.Named(name); ok {
			if pyconf.PoetryPyPI(i) {
				i.URL = public[PyPI]
			}
			if u, _, _ := splitUserinfo(i.URL); u != "" && !strings.Contains(u, "$") {
				k.put(PyPI, Source{URL: u, Scope: pkg})
			}
			continue
		}
		if tool.fallback == nil {
			continue
		}
		if i, ok := tool.fallback(name); ok && i.URL != "" {
			c.Add(PyPI, Source{URL: i.URL, Scope: pkg, Trusted: true, Origin: OriginProject})
		}
	}
}

// splitUserinfo separates the user name and password written into an index URL,
// as written (a ${VAR} reference included, which url.Parse refuses), from the
// rest of it.
func splitUserinfo(raw string) (rest, user, pass string) {
	raw = strings.TrimSpace(raw)
	scheme, after, ok := strings.Cut(raw, "://")
	if !ok {
		return raw, "", ""
	}
	authority, tail := after, ""
	if i := strings.IndexAny(after, "/?#"); i >= 0 {
		authority, tail = after[:i], after[i:]
	}
	at := strings.LastIndex(authority, "@")
	if at < 0 {
		return raw, "", ""
	}
	user, pass, _ = strings.Cut(authority[:at], ":")
	return scheme + "://" + authority[at+1:] + tail, user, pass
}

// referencedCredential is the credential a repository writes as references to this
// machine's environment: a password that is exactly $NAME or ${NAME} (the user name
// written out or a reference too), or, without a password, a user name that is one
// (a token sent as the user name). ok is false for a password written out.
func (c *Config) referencedCredential(user, pass string) (u, p string, ok bool) {
	if pass != "" {
		if p, ok = c.secretOf(pass); !ok {
			return "", "", false
		}
		if name, _ := npmconf.Reference(user); name != "" {
			return c.m.Env(name), p, true
		}
		if strings.Contains(user, "$") {
			return "", "", false
		}
		return user, p, true
	}
	u, ok = c.secretOf(user)
	return u, "", ok
}

// lendPython hands the credential store a credential for a Python index the
// repository names, when this machine vouches for the index: the user did with
// --trust-index, this machine's own configuration names a PyPI index on its host,
// or a Poetry repository of this machine's is there. Anything else would let the
// repository choose where this machine's secret goes.
//
// Implements: REQ-AUTH-023, REQ-AUTH-026
func (c *Config) lendPython(index, user, pass string) {
	if c.credentials == nil || user == "" && pass == "" {
		return
	}
	u, err := url.Parse(strings.TrimSpace(index))
	if err != nil || u.Host == "" {
		return
	}
	if c.vouched(PyPI, index, u) {
		c.credentials.LendIndex(index, user, pass)
	}
}

// pypiName is a distribution name as PyPI compares them (PEP 503): lower case, each
// run of "-", "_" and "." as one "-".
func pypiName(name string) string {
	var b strings.Builder
	sep := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if r == '-' || r == '_' || r == '.' {
			sep = true
			continue
		}
		if sep && b.Len() > 0 {
			b.WriteByte('-')
		}
		sep = false
		b.WriteRune(r)
	}
	return b.String()
}

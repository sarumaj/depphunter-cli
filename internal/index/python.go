package index

import (
	"cmp"
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
	// uvStrategyVariable and uvStrategyFile are uv's index-strategy as
	// UV_INDEX_STRATEGY and this machine's uv.toml set it (see pythonMerges).
	uvStrategyVariable, uvStrategyFile string
	// findLinks says this machine's uv configuration names find-links locations.
	findLinks bool
}

// pythonProject is what the repository's uv and PDM files say about how their
// indexes are searched.
type pythonProject struct {
	// uvStrategy is the first index-strategy a uv file of the repository sets.
	uvStrategy string
	// findLinks says a uv file of the repository names find-links locations.
	findLinks bool
	// pdmMerges says a pyproject.toml declares PDM sources without
	// respect-source-order.
	pdmMerges bool
}

// machinePython reads the indexes of this machine's uv and PDM configuration, after
// pip's (whose index-url, when it names one, stays the replacement asked): uv's
// variables and then its user's and system's uv.toml (pyconf.UVMachine), a default
// index replacing PyPI and any other asked beside it, an explicit one serving only
// what a repository pins to it by name, a flat one (format = "flat", find-links,
// UV_FIND_LINKS) read as a page or a directory of files; PDM's global config.toml
// with PDM_PYPI_URL over it, its [pypi] url replacing PyPI and each [pypi.<name>]
// asked beside it.
// Poetry's config.toml repositories and POETRY_REPOSITORIES_<NAME>_URL are publish
// targets, not sources: they are kept for their hosts (see vouched) and their
// credentials.
//
// Implements: REQ-SUP-066, REQ-SUP-064
func (c *Config) machinePython(m userconf.Machine, k sink) {
	c.py = pythonMachine{hosts: map[string]bool{}}
	c.py.uv = pyconf.UVMachine(m)
	c.py.uvStrategyVariable, c.py.uvStrategyFile = pyconf.UVMachineStrategy(m)
	for _, i := range c.py.uv {
		if addPython(i, false, k) != "" && i.Kind == pyconf.Explicit {
			c.py.vouch(i.URL)
		}
		c.py.findLinks = c.py.findLinks || i.FindLinks
	}
	c.py.pdm = pyconf.PDMMachine(m)
	for _, i := range c.py.pdm.Indexes {
		addPython(i, false, k)
	}
	var repositories map[string]string
	repositories, c.py.poetry = pyconf.PoetryMachine(m)
	for _, r := range repositories {
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
// those, Extra is asked beside PyPI, Supplemental is asked after the primary
// sources, and for each PDM include_packages pattern any of them serves the
// matching packages (with the other sources that include them) alone. An explicit
// index serves only what is pinned to it and is not a source by itself. It answers
// the URL recorded.
func addPython(i pyconf.Index, listed bool, k sink) string {
	u := strings.TrimSpace(i.URL)
	if u == "" {
		return ""
	}
	s := Source{URL: u, flatPage: flatPage(i), exclude: i.Exclude, uvFile: i.File != ""}
	switch {
	case i.Kind == pyconf.Explicit:
		return u
	case i.Kind == pyconf.Primary, i.Kind == pyconf.Default && listed:
		s.Kind = Listed
	case i.Kind == pyconf.Default:
		s.Kind = Replace
	case i.Kind == pyconf.Supplemental:
		s.Kind = Supplemental
	default:
		s.Kind = Additive
	}
	for _, p := range i.Include {
		k.put(PyPI, Source{URL: u, Scope: p, include: true, flatPage: flatPage(i)})
	}
	k.put(PyPI, s)
	return u
}

// included are the sources a package several PDM sources include is asked of:
// every one whose include_packages matches it, in order, as PDM asks all of them.
//
// Implements: REQ-SUP-066
func (c *Config) included(ecosystem, packageName string) []candidate {
	var out []candidate
	seen := map[string]bool{}
	for _, s := range c.sources[ecosystem] {
		if s.include && matches(ecosystem, s.Scope, packageName) && !seen[s.URL] {
			seen[s.URL] = true
			out = append(out, candidate{url: s.URL, primary: true, known: c.fetchable(ecosystem, s)})
		}
	}
	return out
}

// excluded reports whether a source is never asked for a package: one of its PDM
// exclude_packages patterns matches it.
func excluded(ecosystem string, s Source, packageName string) bool {
	return slices.ContainsFunc(s.exclude, func(pattern string) bool { return matches(ecosystem, pattern, packageName) })
}

// uvFirst wraps the repository's sink for what uv's own files name: uv reads a
// project's configuration before the user's and the system's uv.toml (arrays
// concatenated project first, the first default index winning), so a repository's
// uv index goes before the first index of this machine's uv.toml files - after
// what uv's variables and every other tool of this machine name.
//
// Implements: REQ-SUP-066, REQ-SUP-063
func (c *Config) uvFirst(k sink) sink {
	return sink{off: k.off, put: func(ecosystem string, s Source) {
		n := len(c.sources[ecosystem])
		k.put(ecosystem, s)
		sources := c.sources[ecosystem]
		if len(sources) != n+1 {
			return // already named
		}
		if i := slices.IndexFunc(sources, func(s Source) bool { return s.uvFile }); i >= 0 {
			added := sources[n]
			copy(sources[i+1:], sources[i:n])
			sources[i] = added
		}
	}}
}

// flatPage is where a flat index's list of files is, as written: "" for an index
// that is not flat.
func flatPage(i pyconf.Index) string {
	if !i.Flat {
		return ""
	}
	return strings.TrimSpace(i.URL)
}

// pythonFlat is the page (or directory) a flat index lists its files on, "" for an
// index that is not flat, and whether it is a local directory this machine's own
// configuration names, which alone is read from disk.
func (c *Config) pythonFlat(index string) (page string, local bool) {
	for _, s := range c.sources[PyPI] {
		if s.URL == index && s.flatPage != "" {
			page = s.flatPage
			local = local || s.Trusted && fileURLPath(s.flatPage) != ""
		}
	}
	return page, local
}

// pythonMerges says why the Python tools configured here would merge the versions
// several indexes have, which depphunter does not: it asks the indexes in order and
// takes the first that has the package. "" when none would.
//
// Implements: REQ-SUP-066, REQ-TRC-017
func (c *Config) pythonMerges() string {
	var reasons []string
	strategy := cmp.Or(c.py.uvStrategyVariable, c.pythonProject.uvStrategy, c.py.uvStrategyFile)
	if strategy == "unsafe-best-match" {
		reasons = append(reasons, "uv's index-strategy unsafe-best-match picks the best version of all its indexes")
	}
	if c.py.findLinks || c.pythonProject.findLinks {
		reasons = append(reasons, "uv adds the files of its find-links locations to every index's")
	}
	if c.pythonProject.pdmMerges {
		reasons = append(reasons, "PDM merges the versions of all its sources unless respect-source-order is set")
	}
	if len(reasons) == 0 {
		return ""
	}
	return strings.Join(reasons, "; ") + ": depphunter asks the indexes in order and takes the first that has the package"
}

// pyTool says how one tool's repository configuration is read.
type pyTool struct {
	// listed makes a default index one of the ordered primary sources (Poetry).
	listed bool
	// credential is this machine's credential for an index of the tool by name.
	credential func(name string) (user, pass string)
	// named finds an index a pin names before the file's own: uv looks a name up
	// among the indexes its variables (UV_INDEX, UV_DEFAULT_INDEX) name first.
	named func(name string) (pyconf.Index, bool)
	// uv marks uv's files: their indexes go before this machine's uv.toml ones
	// (see uvFirst), and their index-strategy and find-links are noted.
	uv bool
}

// projectPython prepares the reading of what the repository's pyproject.toml,
// uv.toml, Pipfile, Pipfile.lock and pdm.toml files say for uv, Poetry, Pipenv and
// PDM, and answers the reader of one file, which reports whether the file is one of
// those. A directory's uv.toml replaces the [tool.uv] indexes of the pyproject.toml
// beside it, as in uv, unless UV_NO_CONFIG or UV_CONFIG_FILE keeps uv from reading
// project files; the packages [tool.uv.sources] pins are resolved as uv lowers
// them: against the indexes uv's variables name, then the pyproject.toml's own
// [[tool.uv.index]] entries (read for that even beside a uv.toml), never a
// uv.toml's.
//
// Implements: REQ-SUP-066, REQ-AUTH-023
func (c *Config) projectPython(files []*scan.File) func(f *scan.File, base string, data []byte, k sink) bool {
	environment := c.m.Environment
	if environment == nil {
		environment = func(string) string { return "" }
	}
	m := userconf.Machine{Environment: environment}
	uvToml := map[string]pyconf.Settings{}
	if m.UVProjectConfig() {
		for _, f := range files {
			if strings.ToLower(path.Base(f.Path)) != "uv.toml" {
				continue
			}
			if data, err := os.ReadFile(f.AbsolutePath); err == nil {
				if s, ok := pyconf.UV(data, false); ok {
					uvToml[path.Dir(f.Path)] = s
				}
			}
		}
	}
	uv := pyTool{
		uv:         true,
		credential: func(name string) (string, string) { return pyconf.UVCredential(environment, name) },
		named: func(name string) (pyconf.Index, bool) {
			variables := pyconf.Settings{}
			for _, i := range c.py.uv {
				if i.File == "" {
					variables.Indexes = append(variables.Indexes, i)
				}
			}
			return variables.Named(name)
		},
	}
	poetry := pyTool{listed: true, credential: func(name string) (string, string) {
		credential := pyconf.PoetryCredential(m, c.py.poetry, name)
		return credential.Username, credential.Password
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
			if _, replaced := uvToml[path.Dir(f.Path)]; ok && replaced {
				c.pythonPins(s, uv, c.uvFirst(k)) // the indexes are uv.toml's; only the pins are read here
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
// this machine holds for them (see lendPython), and records its pins. A file that
// drops the tool's implicit PyPI switches PyPI off: it is asked only where one of
// the file's indexes names it.
//
// Implements: REQ-SUP-066, REQ-SUP-063
func (c *Config) pythonSettings(s pyconf.Settings, tool pyTool, k sink) {
	if tool.uv {
		k = c.uvFirst(k)
		c.pythonProject.uvStrategy = cmp.Or(c.pythonProject.uvStrategy, s.IndexStrategy)
	}
	if s.PDMSources && !s.RespectSourceOrder {
		c.pythonProject.pdmMerges = true
	}
	if s.NoImplicitPyPI {
		k.off(PyPI)
	}
	for i := range s.Indexes {
		index := &s.Indexes[i]
		if pyconf.PoetryPyPI(*index) {
			index.URL = public[PyPI]
		}
		clean, user, pass := splitUserinfo(index.URL)
		if strings.Contains(clean, "$") {
			index.URL = "" // a URL made of this machine's variables is not drawn on the map
			continue
		}
		index.URL = clean
		if index.Flat && !webURL(clean) {
			continue // a directory the repository names is no address anyone can be asked at
		}
		c.pythonProject.findLinks = c.pythonProject.findLinks || index.FindLinks
		if addPython(*index, tool.listed, k) == "" {
			continue
		}
		if u, p, ok := c.referencedCredential(user, pass); ok {
			c.lendPython(clean, u, p)
		} else if u, p, ok := c.referencedCredential(index.Username, index.Password); ok {
			c.lendPython(clean, u, p)
		} else if tool.credential != nil && index.Name != "" {
			if u, p := tool.credential(index.Name); u != "" || p != "" {
				c.lendPython(clean, u, p)
			}
		}
	}
	c.pythonPins(s, tool, k)
}

// pythonPins records the packages a file pins to an index by name, each served by
// that index alone. uv looks the name up among the indexes its variables name
// first (such an index stays trusted), then the file's own.
func (c *Config) pythonPins(s pyconf.Settings, tool pyTool, k sink) {
	for _, packageName := range slices.Sorted(maps.Keys(s.Pins)) {
		name := s.Pins[packageName]
		if tool.named != nil {
			if i, ok := tool.named(name); ok {
				if i.URL != "" {
					c.Add(PyPI, Source{URL: i.URL, Scope: packageName, Trusted: true, Origin: OriginProject, flatPage: flatPage(i)})
				}
				continue
			}
		}
		if i, ok := s.Named(name); ok {
			if pyconf.PoetryPyPI(i) {
				i.URL = public[PyPI]
			}
			if u, _, _ := splitUserinfo(i.URL); u != "" && !strings.Contains(u, "$") && (!i.Flat || webURL(u)) {
				i.URL = u
				k.put(PyPI, Source{URL: u, Scope: packageName, flatPage: flatPage(i)})
			}
		}
	}
}

// webURL reports whether a location is an http or https URL.
func webURL(location string) bool {
	lower := strings.ToLower(location)
	return strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://")
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
			return c.m.Environment(name), p, true
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
	if (user != "" || pass != "") && c.mayLend(PyPI, index) {
		c.credentials.LendIndex(index, user, pass)
	}
}

// pypiName is a distribution name as PyPI compares them (PEP 503): lower case, each
// run of "-", "_" and "." as one "-".
func pypiName(name string) string {
	var b strings.Builder
	separator := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if r == '-' || r == '_' || r == '.' {
			separator = true
			continue
		}
		if separator && b.Len() > 0 {
			b.WriteByte('-')
		}
		separator = false
		b.WriteRune(r)
	}
	return b.String()
}

package index

import (
	"maps"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/npmconf"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// machineYarnBun reads the registries of this machine's Yarn and Bun
// configuration, after npm's (whose registry, when it names one, stays the
// replacement asked): Yarn Berry's files (userconf.YarnConfigs: those of the
// directories above the analyzed one outside its checkout, then the home
// directory's, merged key by key), with YARN_NPM_REGISTRY_SERVER over their
// npmRegistryServer and ${VAR} references resolved from the environment; Yarn 1's
// ~/.yarnrc; Bun's global bunfig, with $VAR resolved. Each names a default registry
// and the registry of each scope.
//
// Implements: REQ-SUP-015, REQ-SUP-064, REQ-SUP-079
func machineYarnBun(m userconf.Machine, k sink) {
	read := func(name string) []byte {
		if name == "" {
			return nil
		}
		data, _ := os.ReadFile(name)
		return data
	}
	var files [][]byte
	for _, name := range m.YarnConfigs() {
		if data := read(name); data != nil {
			files = append(files, data)
		}
	}
	berry, _ := npmconf.MergeYarnrc(files)
	berry.YarnEnvironment(m.Environment)
	addNpmSettings(berry, k.add, func(v string) string {
		out, ok := npmconf.Interpolate(v, m.Environment)
		if !ok {
			return ""
		}
		return out
	})
	if data := read(m.YarnClassicConfig()); data != nil {
		addNpmSettings(npmconf.ParseYarnClassic(data), k.add, nil)
	}
	if bun, ok := npmconf.ParseBunfig(read(m.BunConfig())); ok {
		addNpmSettings(bun, k.add, func(v string) string { return os.Expand(v, m.Environment) })
	}
}

// addNpmSettings records the default registry and the scope registries of one
// Yarn or Bun file. resolve fills a value's variable references; nil leaves a
// value that has any unused, as the repository's are: its variables are this
// machine's, and would be drawn on the map.
func addNpmSettings(s npmconf.Settings, add func(ecosystem, url, scope string), resolve func(string) string) {
	value := func(v string) string {
		if resolve != nil {
			return resolve(v)
		}
		if strings.Contains(v, "$") {
			return ""
		}
		return v
	}
	add(NPM, value(s.Registry.URL), "")
	for _, scope := range slices.Sorted(maps.Keys(s.Scopes)) {
		add(NPM, value(s.Scopes[scope].URL), scope)
	}
}

// yarnRCFilename is the name of Yarn Berry's configuration files on this machine.
func (c *Config) yarnRCFilename() string {
	if c.m.Environment == nil {
		return ".yarnrc.yml"
	}
	return c.m.YarnRCFilename()
}

// yarnFiles are a repository's Yarn Berry files, by directory ("." for the root):
// those the scan found (.yarnrc.yml, or the name YARN_RC_FILENAME gives them) and
// those of the directories between the analyzed directory and the top of its
// checkout (userconf.Machine.DirectoriesAbove), which are the repository's too.
type yarnFiles struct {
	scanned map[string][]byte
	// above are the files of the checkout's directories above the analyzed one,
	// closest first.
	above [][]byte
}

// readYarnFiles collects a repository's Yarn Berry files.
func (c *Config) readYarnFiles(files []*scan.File) yarnFiles {
	y := yarnFiles{scanned: map[string][]byte{}}
	for _, f := range files {
		if c.isYarnrc(f) {
			if data, err := os.ReadFile(f.AbsolutePath); err == nil {
				y.scanned[path.Dir(f.Path)] = data
			}
		}
	}
	repository, _ := c.m.DirectoriesAbove()
	checkout := checkoutRoot(repository)
	for _, directory := range repository {
		if data, ok := checkout.ReadBounded(filepath.Join(directory, c.yarnRCFilename())); ok {
			y.above = append(y.above, data)
		}
	}
	return y
}

// isYarnrc reports whether a scanned file is a Yarn Berry configuration file.
func (c *Config) isYarnrc(f *scan.File) bool {
	base := strings.ToLower(path.Base(f.Path))
	return base == ".yarnrc.yml" || base == strings.ToLower(c.yarnRCFilename())
}

// chain is the configuration Yarn takes for a package in directory: the files of
// directory and of each directory above it, closest first, up to the checkout's top.
func (y yarnFiles) chain(directory string) [][]byte {
	var out [][]byte
	for d := directory; ; d = path.Dir(d) {
		if data, ok := y.scanned[d]; ok {
			out = append(out, data)
		}
		if d == "." {
			break
		}
	}
	return append(out, y.above...)
}

// projectYarnrc records the registries of the Yarn Berry configuration a
// repository gives the packages of directory - its .yarnrc.yml merged key by key
// with those of the directories above it in the repository (yarnFiles.chain) - and
// lends the credentials it binds to them whose secret is this machine's: an
// npmAuthToken or npmAuthIdent that is exactly ${NAME} (or ${NAME:-fallback} with
// NAME set) - see lendNpm - each with the packages Yarn sends it with (see
// npmconf.Settings.YarnCredentials). A token or identifier written out, or one
// only its fallback fills, is the repository's and is discarded.
//
// Implements: REQ-SUP-015, REQ-AUTH-023, REQ-SUP-079
func (c *Config) projectYarnrc(files [][]byte, add func(ecosystem, url, scope string)) {
	s, ok := npmconf.MergeYarnrc(files)
	if !ok {
		return
	}
	addNpmSettings(s, add, nil)
	for _, e := range s.YarnCredentials(nil) {
		if token, ok := c.secretOf(e.Token); ok {
			c.lendNpm(e.URL, e.Packages, true, token)
		} else if identifier, ok := c.secretOf(e.Ident); ok {
			if pair, ok := (npmconf.Entry{Ident: identifier}).Basic(); ok {
				c.lendNpm(e.URL, e.Packages, false, pair)
			}
		}
	}
}

// projectBunfig records the registries of a repository's bunfig.toml, and lends
// the credentials whose token or password is exactly $NAME or ${NAME} (a user
// name may be written out) - see lendNpm. A token or password written out,
// in a table or in the URL, is discarded.
//
// Implements: REQ-SUP-015, REQ-AUTH-023
func (c *Config) projectBunfig(data []byte, add func(ecosystem, url, scope string)) {
	s, ok := npmconf.ParseBunfig(data)
	if !ok {
		return
	}
	addNpmSettings(s, add, nil)
	for _, e := range s.Credentials(npmconf.BunDefault) {
		if token, ok := c.secretOf(e.Token); ok {
			c.lendNpm(e.URL, "", true, token)
		} else if pass, ok := c.secretOf(e.Password); ok && e.Username != "" {
			user := e.Username
			if name, _ := npmconf.Reference(user); name != "" {
				user = c.m.Environment(name)
			}
			if user != "" {
				c.lendNpm(e.URL, "", false, user+":"+pass)
			}
		}
	}
}

// secretOf is the value of the machine's variable that v refers to and consists
// of, when it is set; ok is false for anything written out, including a
// fallback.
func (c *Config) secretOf(v string) (string, bool) {
	name, _ := npmconf.Reference(v)
	if name == "" || c.m.Environment == nil {
		return "", false
	}
	value := c.m.Environment(name)
	return value, value != ""
}

// lendNpm hands the credential store a credential for a registry the repository
// names, sent with the requests packages allows (see auth.Store.LendRegistry),
// when this machine vouches for it: the user did with --trust-index, or
// this machine's own npm, Yarn or Bun configuration names a registry on its host.
// Anything else would let the repository choose where this machine's secret goes.
//
// Implements: REQ-AUTH-023
func (c *Config) lendNpm(registry, packages string, bearer bool, value string) {
	if value == "" || c.credentials == nil || strings.Contains(registry, "$") {
		return
	}
	u, err := url.Parse(strings.TrimSpace(registry))
	if err != nil || u.Host == "" {
		return
	}
	if c.vouched(NPM, registry, u) {
		c.credentials.LendRegistry(registry, packages, bearer, value)
	}
}

// checkoutRoot reads the files of the checkout's directories above the analyzed
// one (userconf.Machine.DirectoriesAbove, closest first): they are the
// repository's, and read only inside it, through the Root of its top.
func checkoutRoot(repository []string) lang.Root {
	if len(repository) == 0 {
		return lang.Root{}
	}
	return lang.OpenRoot(repository[len(repository)-1])
}

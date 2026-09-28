package puppet

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Well-known modules by their short name, for a module no manifest
// declares: the Forge slug names it (unresolved, since nothing says which
// version).
var knownModule = map[string]string{
	"stdlib": "puppetlabs-stdlib", "concat": "puppetlabs-concat", "apt": "puppetlabs-apt", "apache": "puppetlabs-apache",
	"mysql": "puppetlabs-mysql", "postgresql": "puppetlabs-postgresql", "inifile": "puppetlabs-inifile",
	"firewall": "puppetlabs-firewall", "ntp": "puppetlabs-ntp", "java": "puppetlabs-java", "tomcat": "puppetlabs-tomcat",
	"docker": "puppetlabs-docker", "haproxy": "puppetlabs-haproxy", "motd": "puppetlabs-motd", "registry": "puppetlabs-registry",
	"vcsrepo": "puppetlabs-vcsrepo", "powershell": "puppetlabs-powershell", "reboot": "puppetlabs-reboot",
	"puppetdb": "puppetlabs-puppetdb", "puppet_agent": "puppetlabs-puppet_agent", "java_ks": "puppetlabs-java_ks",
	"translate": "puppetlabs-translate", "facts": "puppetlabs-facts", "archive": "puppet-archive", "nginx": "puppet-nginx",
	"systemd": "puppet-systemd", "epel": "puppet-epel", "selinux": "puppet-selinux", "augeasproviders_core": "puppet-augeasproviders_core",
}

// The custom resource types of well-known modules whose name does not start
// with the module's.
var knownType = map[string]string{
	"file_line": "stdlib", "anchor": "stdlib", "ini_setting": "inifile", "ini_subsetting": "inifile",
	"concat_fragment": "concat", "firewallchain": "firewall", "java_ks": "java_ks",
}

type resolver struct {
	root  string
	files map[string]bool
	// modules maps a module's short name to the directories of the
	// repository's modules of that name.
	modules     map[string][]string
	metadata    map[string]*metadata // module directory -> its metadata.json
	fixtures    map[string][]*dep    // directory -> its .fixtures.yml
	puppetfiles map[string]*puppetfile
	pfDirs      []string
	rubyTypes   map[string]string // custom type -> module directory (lib/puppet/type/x.rb)

	mu        sync.Mutex
	installed map[string]*metadata // absolute metadata.json path -> read (nil: none)
}

// segment names under a module's root that only a module has.
var moduleDirs = map[string]bool{"manifests": true, "functions": true, "types": true, "plans": true}

func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{root: root, files: map[string]bool{}, modules: map[string][]string{}, metadata: map[string]*metadata{},
		fixtures: map[string][]*dep{}, puppetfiles: map[string]*puppetfile{}, rubyTypes: map[string]string{}, installed: map[string]*metadata{}}
	roots := map[string]bool{}
	addRoot := func(name, dir string) {
		if name == "" || roots[name+"\x00"+dir] {
			return
		}
		roots[name+"\x00"+dir] = true
		r.modules[name] = append(r.modules[name], dir)
	}
	for _, f := range all {
		if installed(f.Path, f.Abs) {
			continue
		}
		r.files[f.Path] = true
		segments := strings.Split(f.Path, "/")
		for i := 0; i+1 < len(segments); i++ {
			if moduleDirs[segments[i]] && path.Ext(f.Path) == ".pp" && i > 0 {
				addRoot(segments[i-1], strings.Join(segments[:i], "/"))
				break
			}
			if segments[i] == "lib" && segments[i+1] == "puppet" {
				dir := strings.Join(segments[:i], "/")
				if i > 0 {
					addRoot(segments[i-1], dir)
				}
				if len(segments) == i+4 && segments[i+2] == "type" && path.Ext(f.Path) == ".rb" {
					r.rubyTypes[strings.TrimSuffix(segments[i+3], ".rb")] = dir
				}
				break
			}
		}
		if f.Binary || f.TooLarge || f.Size > lang.MaxParseSize {
			continue
		}
		dir := path.Dir(f.Path)
		switch class(f.Path) {
		case classPuppetfile:
			if src, err := os.ReadFile(f.Abs); err == nil {
				r.puppetfiles[dir] = readPuppetfile(src)
				r.pfDirs = append(r.pfDirs, dir)
			}
		case classMetadata:
			if src, err := os.ReadFile(f.Abs); err == nil {
				if m := readMetadata(src); m != nil {
					r.metadata[dir] = m
					if dir == "." {
						dir = ""
					}
					addRoot(short(m.name), dir)
				}
			}
		case classFixtures:
			if src, err := os.ReadFile(f.Abs); err == nil {
				r.fixtures[dir] = readFixtures(src)
			}
		}
	}
	// A repository .fixtures.yml installs for a module its metadata.json
	// declares from the Forge is that Forge module, installed from git.
	for dir, deps := range r.fixtures {
		if m := r.metadata[dir]; m != nil {
			for _, d := range deps {
				for _, md := range m.deps {
					if !d.forge && md.name == d.name {
						d.pkg = md.pkg
					}
				}
			}
		}
	}
	sort.Strings(r.pfDirs)
	for _, dirs := range r.modules {
		sort.Strings(dirs)
	}
	return r
}

// Implements: REQ-PUPPET-004, REQ-PUPPET-005, REQ-PUPPET-006, REQ-PUPPET-010
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	dir := path.Dir(file)
	switch imp.Name {
	case kindMod:
		if pf := r.puppetfiles[dir]; pf != nil {
			return find(pf.deps, imp.Module)
		}
		return lang.Target{}
	case kindMetadata:
		if m := r.metadata[dir]; m != nil {
			return find(m.deps, imp.Module)
		}
		return lang.Target{}
	case kindFixture:
		return find(r.fixtures[dir], imp.Module)
	}
	mod, rest := imp.Module, ""
	sep := "::"
	if imp.Name == kindTemplate || imp.Name == kindFile {
		sep = "/"
	}
	if i := strings.Index(mod, sep); i >= 0 {
		mod, rest = mod[:i], mod[i+len(sep):]
	}
	mod = strings.ToLower(mod)
	if root, ok := r.local(file, mod); ok {
		return lang.Target{Local: r.localFile(root, mod, rest, imp.Name)}
	}
	if imp.Name == kindDefine && rest == "" {
		// An unqualified custom type: a module's lib/puppet/type/<name>.rb,
		// a well-known module's type, or the declared module its prefix names.
		if root, ok := r.rubyTypes[mod]; ok {
			return lang.Target{Local: path.Join(root, "lib/puppet/type", mod+".rb")}
		}
		if m, ok := knownType[mod]; ok {
			mod = m
		} else if prefix, _, ok := strings.Cut(mod, "_"); ok && (r.declared(file, prefix) != nil || r.modules[prefix] != nil) {
			return r.Resolve(file, lang.RawImport{Module: prefix + "::" + mod, Name: kindDefine})
		} else if r.declared(file, mod) == nil && knownModule[mod] == "" {
			return lang.Target{}
		}
		if root, ok := r.local(file, mod); ok {
			return lang.Target{Local: r.localFile(root, mod, "", kindClass)}
		}
	}
	if d := r.declared(file, mod); d != nil {
		return d.target()
	}
	if t, ok := r.installedModule(mod); ok {
		return t
	}
	if s := knownModule[mod]; s != "" {
		return lang.Target{Ecosystem: ecoForge, Package: s, Unresolved: true}
	}
	return lang.Target{Ecosystem: ecoForge, Package: mod, Unresolved: true}
}

func find(deps []*dep, key string) lang.Target {
	for _, d := range deps {
		if d.key == key {
			return d.target()
		}
	}
	return lang.Target{}
}

// local finds the repository's module of that name nearest the file: one
// the file is in, else the one sharing the longest path with it.
func (r *resolver) local(file, mod string) (string, bool) {
	dirs := r.modules[mod]
	if len(dirs) == 0 {
		return "", false
	}
	best, score := "", -1
	for _, d := range dirs {
		s := common(file, d)
		if d == "" || strings.HasPrefix(file, d+"/") {
			s += 1 << 20 // the module the file is in
		}
		if s > score || s == score && len(d) < len(best) {
			best, score = d, s
		}
	}
	return best, true
}

// common is the number of leading path segments a and b share.
func common(a, b string) int {
	as, bs := strings.Split(a, "/"), strings.Split(b, "/")
	n := 0
	for n < len(as) && n < len(bs) && as[n] == bs[n] {
		n++
	}
	return n
}

// localFile is the file Puppet's autoloader reads for a name in the module
// at root: x::y::z is manifests/y/z.pp (a class or defined type),
// functions/y/z.pp or lib/puppet/functions/x/y/z.rb (a function),
// types/y/z.pp (a type alias); x alone is manifests/init.pp; a template or
// file is under templates/ or files/. A name whose file is missing links to
// the module's init.pp, its metadata.json or its directory.
func (r *resolver) localFile(root, mod, rest, kind string) string {
	sub := strings.ReplaceAll(rest, "::", "/")
	var tries []string
	switch kind {
	case kindTemplate:
		tries = []string{"templates/" + rest}
	case kindFile:
		tries = []string{"files/" + rest}
	case kindFunction:
		tries = []string{"functions/" + sub + ".pp", "lib/puppet/functions/" + mod + "/" + sub + ".rb", "lib/puppet/parser/functions/" + sub + ".rb"}
	case kindType:
		tries = []string{"types/" + sub + ".pp", "manifests/" + sub + ".pp"}
	default:
		if rest == "" {
			tries = []string{"manifests/init.pp"}
		} else {
			tries = []string{"manifests/" + sub + ".pp", "lib/puppet/type/" + mod + "_" + strings.ReplaceAll(rest, "::", "_") + ".rb"}
		}
	}
	tries = append(tries, "manifests/init.pp", "metadata.json")
	for _, t := range tries {
		if p := path.Join(root, t); r.files[p] {
			return p
		}
	}
	if root == "" {
		return "."
	}
	return root
}

// declared is the dependency on a module the manifests governing a file
// declare: the metadata.json and .fixtures.yml of the module it is in, the
// Puppetfiles above it (nearest first), then any other manifest's.
func (r *resolver) declared(file, mod string) *dep {
	var lists [][]*dep
	for d := path.Dir(file); ; d = path.Dir(d) {
		if m := r.metadata[d]; m != nil {
			lists = append(lists, m.deps)
		}
		lists = append(lists, r.fixtures[d])
		if pf := r.puppetfiles[d]; pf != nil {
			lists = append(lists, pf.deps)
		}
		if d == "." || d == "/" {
			break
		}
	}
	for _, d := range r.pfDirs {
		lists = append(lists, r.puppetfiles[d].deps)
	}
	for _, d := range sortedKeys(r.metadata) {
		lists = append(lists, r.metadata[d].deps)
	}
	for _, list := range lists {
		for _, d := range list {
			if d.name == mod {
				return d
			}
		}
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// installedModule is a module r10k installed beside a Puppetfile, named by
// the metadata.json it installed with it.
func (r *resolver) installedModule(mod string) (lang.Target, bool) {
	for _, dir := range r.pfDirs {
		if m := r.installedMetadata(dir, mod); m != nil && short(m.name) == mod {
			return lang.Target{Ecosystem: ecoForge, Package: m.name, Version: m.version}, true
		}
	}
	return lang.Target{}, false
}

// installedMetadata reads the metadata.json of a module installed into the
// module directory of the Puppetfile in dir.
func (r *resolver) installedMetadata(dir, mod string) *metadata {
	pf := r.puppetfiles[dir]
	if pf == nil || mod == "" || strings.ContainsAny(mod, "/\\.:") {
		return nil
	}
	p := filepath.Join(r.root, filepath.FromSlash(dir), filepath.FromSlash(pf.moduledir), mod, "metadata.json")
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.installed[p]; ok {
		return m
	}
	var m *metadata
	if src, err := os.ReadFile(p); err == nil {
		m = readMetadata(src)
	}
	r.installed[p] = m
	return m
}

// installedFor is the installed module a target is, as the Puppetfile that
// installed it names it, or by its Forge slug.
func (r *resolver) installedFor(t lang.Target) *metadata {
	if t.Ecosystem != ecoForge {
		return nil
	}
	for _, dir := range r.pfDirs {
		for _, d := range r.puppetfiles[dir].deps {
			if d.pkg == t.Package {
				if m := r.installedMetadata(dir, d.name); m != nil {
					return m
				}
			}
		}
		if m := r.installedMetadata(dir, short(t.Package)); m != nil && m.name == t.Package {
			return m
		}
	}
	return nil
}

// Dependencies are what an installed module's metadata.json depends on.
//
// Implements: REQ-PUPPET-007
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	m := r.installedFor(t)
	if m == nil {
		return nil
	}
	out := make([]lang.Target, 0, len(m.deps))
	for _, d := range m.deps {
		out = append(out, d.target())
	}
	return out
}

// Implements: REQ-PUPPET-007
func (r *resolver) Installed(t lang.Target) bool { return r.installedFor(t) != nil }

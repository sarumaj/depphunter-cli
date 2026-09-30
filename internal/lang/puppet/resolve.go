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
	modules       map[string][]string
	metadata      map[string]*metadata     // module directory -> its metadata.json
	fixtures      map[string][]*dependency // directory -> its .fixtures.yml
	puppetfiles   map[string]*puppetfile
	pfDirectories []string
	rubyTypes     map[string]string // custom type -> module directory (lib/puppet/type/x.rb)

	mu        sync.Mutex
	installed map[string]*metadata // absolute metadata.json path -> read (nil: none)
}

// segment names under a module's root that only a module has.
var moduleDirectories = map[string]bool{"manifests": true, "functions": true, "types": true, "plans": true}

func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{root: root, files: map[string]bool{}, modules: map[string][]string{}, metadata: map[string]*metadata{},
		fixtures: map[string][]*dependency{}, puppetfiles: map[string]*puppetfile{}, rubyTypes: map[string]string{}, installed: map[string]*metadata{}}
	roots := map[string]bool{}
	addRoot := func(name, directory string) {
		if name == "" || roots[name+"\x00"+directory] {
			return
		}
		roots[name+"\x00"+directory] = true
		r.modules[name] = append(r.modules[name], directory)
	}
	for _, f := range all {
		if installed(f.Path, f.AbsolutePath) {
			continue
		}
		r.files[f.Path] = true
		segments := strings.Split(f.Path, "/")
		for i := 0; i+1 < len(segments); i++ {
			if moduleDirectories[segments[i]] && path.Ext(f.Path) == ".pp" && i > 0 {
				addRoot(segments[i-1], strings.Join(segments[:i], "/"))
				break
			}
			if segments[i] == "lib" && segments[i+1] == "puppet" {
				directory := strings.Join(segments[:i], "/")
				if i > 0 {
					addRoot(segments[i-1], directory)
				}
				if len(segments) == i+4 && segments[i+2] == "type" && path.Ext(f.Path) == ".rb" {
					r.rubyTypes[strings.TrimSuffix(segments[i+3], ".rb")] = directory
				}
				break
			}
		}
		if f.Binary || f.TooLarge || f.Size > lang.MaxParseSize {
			continue
		}
		directory := path.Dir(f.Path)
		switch class(f.Path) {
		case classPuppetfile:
			if source, err := os.ReadFile(f.AbsolutePath); err == nil {
				r.puppetfiles[directory] = readPuppetfile(source)
				r.pfDirectories = append(r.pfDirectories, directory)
			}
		case classMetadata:
			if source, err := os.ReadFile(f.AbsolutePath); err == nil {
				if m := readMetadata(source); m != nil {
					r.metadata[directory] = m
					if directory == "." {
						directory = ""
					}
					addRoot(short(m.name), directory)
				}
			}
		case classFixtures:
			if source, err := os.ReadFile(f.AbsolutePath); err == nil {
				r.fixtures[directory] = readFixtures(source)
			}
		}
	}
	// A repository .fixtures.yml installs for a module its metadata.json
	// declares from the Forge is that Forge module, installed from git.
	for directory, dependencies := range r.fixtures {
		if m := r.metadata[directory]; m != nil {
			for _, d := range dependencies {
				for _, md := range m.dependencies {
					if !d.forge && md.name == d.name {
						d.packageName = md.packageName
					}
				}
			}
		}
	}
	sort.Strings(r.pfDirectories)
	for _, directories := range r.modules {
		sort.Strings(directories)
	}
	return r
}

// Implements: REQ-PUPPET-004, REQ-PUPPET-005, REQ-PUPPET-006, REQ-PUPPET-010
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	directory := path.Dir(file)
	switch rawImport.Name {
	case kindModule:
		if puppetfile := r.puppetfiles[directory]; puppetfile != nil {
			return find(puppetfile.dependencies, rawImport.Module)
		}
		return lang.Target{}
	case kindMetadata:
		if m := r.metadata[directory]; m != nil {
			return find(m.dependencies, rawImport.Module)
		}
		return lang.Target{}
	case kindFixture:
		return find(r.fixtures[directory], rawImport.Module)
	}
	module, rest := rawImport.Module, ""
	separator := "::"
	if rawImport.Name == kindTemplate || rawImport.Name == kindFile {
		separator = "/"
	}
	if i := strings.Index(module, separator); i >= 0 {
		module, rest = module[:i], module[i+len(separator):]
	}
	module = strings.ToLower(module)
	if root, ok := r.local(file, module); ok {
		return lang.Target{Local: r.localFile(root, module, rest, rawImport.Name)}
	}
	if rawImport.Name == kindDefine && rest == "" {
		// An unqualified custom type: a module's lib/puppet/type/<name>.rb,
		// a well-known module's type, or the declared module its prefix names.
		if root, ok := r.rubyTypes[module]; ok {
			return lang.Target{Local: path.Join(root, "lib/puppet/type", module+".rb")}
		}
		if m, ok := knownType[module]; ok {
			module = m
		} else if prefix, _, ok := strings.Cut(module, "_"); ok && (r.declared(file, prefix) != nil || r.modules[prefix] != nil) {
			return r.Resolve(file, lang.RawImport{Module: prefix + "::" + module, Name: kindDefine})
		} else if r.declared(file, module) == nil && knownModule[module] == "" {
			return lang.Target{}
		}
		if root, ok := r.local(file, module); ok {
			return lang.Target{Local: r.localFile(root, module, "", kindClass)}
		}
	}
	if d := r.declared(file, module); d != nil {
		return d.target()
	}
	if t, ok := r.installedModule(module); ok {
		return t
	}
	if s := knownModule[module]; s != "" {
		return lang.Target{Ecosystem: ecosystemForge, Package: s, Unresolved: true}
	}
	return lang.Target{Ecosystem: ecosystemForge, Package: module, Unresolved: true}
}

func find(dependencies []*dependency, key string) lang.Target {
	for _, d := range dependencies {
		if d.key == key {
			return d.target()
		}
	}
	return lang.Target{}
}

// local finds the repository's module of that name nearest the file: one
// the file is in, else the one sharing the longest path with it.
func (r *resolver) local(file, module string) (string, bool) {
	directories := r.modules[module]
	if len(directories) == 0 {
		return "", false
	}
	best, score := "", -1
	for _, d := range directories {
		s := lang.CommonSegments(file, d)
		if d == "" || strings.HasPrefix(file, d+"/") {
			s += 1 << 20 // the module the file is in
		}
		if s > score || s == score && len(d) < len(best) {
			best, score = d, s
		}
	}
	return best, true
}

// localFile is the file Puppet's autoloader reads for a name in the module
// at root: x::y::z is manifests/y/z.pp (a class or defined type),
// functions/y/z.pp or lib/puppet/functions/x/y/z.rb (a function),
// types/y/z.pp (a type alias); x alone is manifests/init.pp; a template or
// file is under templates/ or files/. A name whose file is missing links to
// the module's init.pp, its metadata.json or its directory.
func (r *resolver) localFile(root, module, rest, kind string) string {
	subpath := strings.ReplaceAll(rest, "::", "/")
	var tries []string
	switch kind {
	case kindTemplate:
		tries = []string{"templates/" + rest}
	case kindFile:
		tries = []string{"files/" + rest}
	case kindFunction:
		tries = []string{"functions/" + subpath + ".pp", "lib/puppet/functions/" + module + "/" + subpath + ".rb", "lib/puppet/parser/functions/" + subpath + ".rb"}
	case kindType:
		tries = []string{"types/" + subpath + ".pp", "manifests/" + subpath + ".pp"}
	default:
		if rest == "" {
			tries = []string{"manifests/init.pp"}
		} else {
			tries = []string{"manifests/" + subpath + ".pp", "lib/puppet/type/" + module + "_" + strings.ReplaceAll(rest, "::", "_") + ".rb"}
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
func (r *resolver) declared(file, module string) *dependency {
	var lists [][]*dependency
	for d := range lang.Ancestors(file) {
		if m := r.metadata[d]; m != nil {
			lists = append(lists, m.dependencies)
		}
		lists = append(lists, r.fixtures[d])
		if puppetfile := r.puppetfiles[d]; puppetfile != nil {
			lists = append(lists, puppetfile.dependencies)
		}
	}
	for _, d := range r.pfDirectories {
		lists = append(lists, r.puppetfiles[d].dependencies)
	}
	for _, d := range lang.SortedKeys(r.metadata) {
		lists = append(lists, r.metadata[d].dependencies)
	}
	for _, list := range lists {
		for _, d := range list {
			if d.name == module {
				return d
			}
		}
	}
	return nil
}

// installedModule is a module r10k installed beside a Puppetfile, named by
// the metadata.json it installed with it.
func (r *resolver) installedModule(module string) (lang.Target, bool) {
	for _, directory := range r.pfDirectories {
		if m := r.installedMetadata(directory, module); m != nil && short(m.name) == module {
			return lang.Target{Ecosystem: ecosystemForge, Package: m.name, Version: m.version}, true
		}
	}
	return lang.Target{}, false
}

// installedMetadata reads the metadata.json of a module installed into the
// module directory of the Puppetfile in directory.
func (r *resolver) installedMetadata(directory, module string) *metadata {
	puppetfile := r.puppetfiles[directory]
	if puppetfile == nil || module == "" || strings.ContainsAny(module, "/\\.:") {
		return nil
	}
	p := filepath.Join(r.root, filepath.FromSlash(directory), filepath.FromSlash(puppetfile.moduleDirectory), module, "metadata.json")
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.installed[p]; ok {
		return m
	}
	var m *metadata
	if source, err := os.ReadFile(p); err == nil {
		m = readMetadata(source)
	}
	r.installed[p] = m
	return m
}

// installedFor is the installed module a target is, as the Puppetfile that
// installed it names it, or by its Forge slug.
func (r *resolver) installedFor(t lang.Target) *metadata {
	if t.Ecosystem != ecosystemForge {
		return nil
	}
	for _, directory := range r.pfDirectories {
		for _, d := range r.puppetfiles[directory].dependencies {
			if d.packageName == t.Package {
				if m := r.installedMetadata(directory, d.name); m != nil {
					return m
				}
			}
		}
		if m := r.installedMetadata(directory, short(t.Package)); m != nil && m.name == t.Package {
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
	out := make([]lang.Target, 0, len(m.dependencies))
	for _, d := range m.dependencies {
		out = append(out, d.target())
	}
	return out
}

// Implements: REQ-PUPPET-007
func (r *resolver) Installed(t lang.Target) bool { return r.installedFor(t) != nil }

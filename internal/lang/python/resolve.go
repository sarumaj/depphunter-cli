package python

import (
	"encoding/json"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Implements: REQ-PY-005
var stdlib = map[string]bool{"__future__": true, "_thread": true}

func init() {
	// cSpell: disable
	for _, m := range strings.Fields(`abc aifc argparse array ast asyncio atexit audioop base64 bdb
		binascii bisect builtins bz2 cProfile calendar cgi cgitb chunk cmath cmd code codecs codeop
		collections colorsys compileall concurrent configparser contextlib contextvars copy copyreg crypt
		csv ctypes curses dataclasses datetime dbm decimal difflib dis doctest email encodings ensurepip
		enum errno faulthandler fcntl filecmp fileinput fnmatch fractions ftplib functools gc genericpath
		getopt getpass gettext glob graphlib grp gzip hashlib heapq hmac html http idlelib imaplib imghdr
		importlib inspect io ipaddress itertools json keyword lib2to3 linecache locale logging lzma
		mailbox mailcap marshal math mimetypes mmap modulefinder msilib msvcrt multiprocessing netrc nis
		nntplib nt ntpath nturl2path numbers opcode operator optparse os ossaudiodev pathlib pdb pickle
		pickletools pipes pkgutil platform plistlib poplib posix posixpath pprint profile pstats pty pwd
		py_compile pyclbr pydoc pydoc_data pyexpat queue quopri random re readline reprlib resource
		rlcompleter runpy sched secrets select selectors shelve shlex shutil signal site smtplib sndhdr
		socket socketserver spwd sqlite3 sre_compile sre_constants sre_parse ssl stat statistics string
		stringprep struct subprocess sunau symtable sys sysconfig syslog tabnanny tarfile telnetlib
		tempfile termios textwrap threading time timeit tkinter token tokenize tomllib trace traceback
		tracemalloc tty turtle types typing unicodedata unittest urllib uu uuid venv warnings wave
		weakref webbrowser winreg winsound wsgiref xdrlib xml xmlrpc zipapp zipfile zipimport zlib zoneinfo`) {
		stdlib[m] = true
	}
	// cSpell: enable
}

// importAliases maps import names to the distribution that provides them, for the
// well-known cases where the two differ.
//
// Implements: REQ-PY-010
var importAliases = map[string]string{
	// cSpell: disable
	"yaml": "PyYAML", "PIL": "Pillow", "sklearn": "scikit-learn", "skimage": "scikit-image",
	"bs4": "beautifulsoup4", "cv2": "opencv-python", "dateutil": "python-dateutil",
	"dotenv": "python-dotenv", "jwt": "PyJWT", "attr": "attrs", "MySQLdb": "mysqlclient",
	"OpenSSL": "pyOpenSSL", "Crypto": "pycryptodome", "magic": "python-magic", "docx": "python-docx",
	"pptx": "python-pptx", "serial": "pyserial", "usb": "pyusb", "zmq": "pyzmq", "git": "GitPython",
	"jose": "python-jose", "multipart": "python-multipart", "slugify": "python-slugify",
	"websocket": "websocket-client", "win32api": "pywin32", "gi": "PyGObject", "wx": "wxPython",
	"fitz": "PyMuPDF", "google.protobuf": "protobuf", "telegram": "python-telegram-bot",
	"psycopg2": "psycopg2-binary", "Levenshtein": "python-Levenshtein", "faiss": "faiss-cpu",
	// cSpell: enable
}

// dist is a distribution declared in a manifest or pinned in a lockfile. Lockfile-only
// entries (transitive dependencies) are installed too, so importing them resolves.
type dist struct {
	name      string
	version   string
	requested string // the manifest's specifier once a lock file replaced it
	pinned    bool
	git       string // "<repository URL>#<commit>" of a direct reference to a commit
}

type resolver struct {
	lang.NoteList
	files         map[string]bool
	pyDirectories map[string]bool // directories containing Python files at any depth
	roots         []importRoot    // import roots, most specific first, "." last
	distMap       map[string]*dist
	// tree maps a normalized distribution name to what a lock file says it needs.
	tree map[string][]string
	// environment is the interpreter's installed distributions, or nil (findEnvironment).
	environment *environment
}

// newResolver reads the project files in all (claimed are the Python files) from
// root, the analyzed repository. getenv, when not nil, gives the PYTHONPATH of the
// process running depphunter.
//
// Implements: REQ-PY-003, REQ-PY-006, REQ-PY-009, REQ-PY-016
func newResolver(root string, all, claimed []*scan.File, getenv func(string) string) *resolver {
	r := &resolver{files: map[string]bool{}, pyDirectories: map[string]bool{}, distMap: map[string]*dist{}, tree: map[string][]string{}}
	for _, f := range claimed {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); d != "." && !r.pyDirectories[d]; d = path.Dir(d) {
			r.pyDirectories[d] = true
		}
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		absolute = root
	}
	finder := &rootFinder{resolver: r, absolute: absolute, environmentFiles: map[string]bool{}}
	finder.environment(getenv)

	projects := map[string]bool{".": true}
	var locks []*scan.File
	for _, f := range all {
		directory, base := path.Dir(f.Path), path.Base(f.Path)
		switch {
		case base == "pyproject.toml":
			projects[directory] = true
			r.readPyproject(f.AbsolutePath)
			finder.pyproject(f)
		case base == "setup.cfg":
			projects[directory] = true
			r.readSetupConfig(f.AbsolutePath)
			finder.iniFile(f)
		case base == "setup.py":
			projects[directory] = true
			r.readSetupPy(f.AbsolutePath)
			finder.setupPy(f)
		case base == "pytest.ini" || base == ".pytest.ini" || base == "tox.ini" || base == "mypy.ini" || base == ".mypy.ini":
			finder.iniFile(f)
		case base == "pyrightconfig.json" || base == "basedpyrightconfig.json":
			finder.pyrightConfig(f)
		case base == ".env":
			finder.dotenv(f.Path, "PYTHONPATH in "+f.Path)
		case base == "Pipfile":
			r.readPipfile(f.AbsolutePath)
		case base == "poetry.lock" || base == "uv.lock" || base == "pdm.lock" || base == "Pipfile.lock":
			locks = append(locks, f)
		case strings.HasSuffix(base, ".txt") && (strings.HasPrefix(base, "requirements") || path.Base(directory) == "requirements"):
			r.readRequirements(f.AbsolutePath)
		}
	}
	for _, f := range locks { // after manifests, so pinned versions override ranges
		r.readLock(f)
	}
	// The scan skips .vscode and leaves out a git-ignored .env, which is where a .env
	// usually is: both are read from disk in the repository root and in each project
	// directory, the folders an editor opens.
	for _, directory := range slices.Sorted(maps.Keys(projects)) {
		finder.dotenv(path.Join(directory, ".env"), "PYTHONPATH in "+path.Join(directory, ".env"))
		finder.vscode(directory)
	}

	// A project's own roots - the directories of its manifests, their src/ and the
	// source directories its packaging tools name - deepest first, so a sub-project's
	// modules shadow same-named top-level ones. Then the roots the environment and
	// the tools' settings add, in the order they give them, the process's PYTHONPATH
	// first and deeper files before shallower: they stand for paths a run or an
	// editor puts ahead of the working directory, but a sub-project's own modules
	// are more specific than a path configured for the whole repository. The
	// repository root, the least specific, comes last.
	var own []string
	for d := range projects {
		if d != "." {
			own = append(own, d)
		}
		if source := path.Join(d, "src"); r.pyDirectories[source] {
			own = append(own, source)
		}
	}
	for _, packaging := range finder.packaging {
		own = append(own, packaging.directory)
	}
	depth := func(d string) int { return strings.Count(d, "/") }
	sort.Slice(own, func(i, j int) bool {
		if depthI, depthJ := depth(own[i]), depth(own[j]); depthI != depthJ {
			return depthI > depthJ
		}
		return own[i] < own[j]
	})
	seen := map[importRoot]bool{}
	add := func(root importRoot) {
		if !seen[root] && !seen[importRoot{directory: root.directory}] {
			seen[root] = true
			r.roots = append(r.roots, root)
		}
	}
	for _, d := range own {
		add(importRoot{directory: d})
	}
	for _, root := range ordered(finder.configured) {
		add(root)
	}
	add(importRoot{directory: "."})
	return r
}

// Resolve handles both "import a.b" (Name empty) and "from m import n".
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	return r.resolveFrom(rawImport.Module, rawImport.Name, file)
}

// resolve handles "import a.b.c".
//
// Implements: REQ-PY-003, REQ-PY-004, REQ-PY-005
func (r *resolver) resolve(dotted, file string) lang.Target {
	parts := strings.Split(dotted, ".")
	for _, root := range r.roots {
		if !root.serves(file) {
			continue
		}
		if t, ok := r.longest(root.directory, parts); ok {
			return t
		}
	}
	if stdlib[parts[0]] {
		return lang.Target{Ecosystem: ecosystemStd, Package: parts[0]}
	}
	// A script's own directory is on sys.path when it runs.
	if t, ok := r.probe(path.Dir(file), parts); ok {
		return t
	}
	return r.distribution(parts)
}

// resolveFrom handles "from module import name"; name may be a sub-module.
//
// Implements: REQ-PY-002
func (r *resolver) resolveFrom(module, name, file string) lang.Target {
	if !strings.HasPrefix(module, ".") {
		if name != "" {
			return r.resolve(module+"."+name, file)
		}
		return r.resolve(module, file)
	}
	rest := strings.TrimLeft(module, ".")
	base := path.Dir(file)
	for range len(module) - len(rest) - 1 {
		if base == "." {
			return lang.Target{} // climbs out of the project
		}
		base = path.Dir(base)
	}
	var parts []string
	if rest != "" {
		parts = strings.Split(rest, ".")
	}
	if name != "" {
		if t, ok := r.probe(base, append(parts, name)); ok {
			return t
		}
	}
	if len(parts) == 0 {
		if base == "." {
			return lang.Target{}
		}
		return lang.Target{Local: base}
	}
	t, _ := r.probe(base, parts)
	return t
}

func (r *resolver) longest(root string, parts []string) (lang.Target, bool) {
	for n := len(parts); n > 0; n-- {
		if t, ok := r.probe(root, parts[:n]); ok {
			return t, true
		}
	}
	return lang.Target{}, false
}

// probe maps a module path under root to a module file or a package directory.
func (r *resolver) probe(root string, parts []string) (lang.Target, bool) {
	p := path.Join(append([]string{root}, parts...)...)
	for _, candidate := range []string{p + ".py", p + ".pyi"} {
		if r.files[candidate] {
			return lang.Target{Local: candidate}, true
		}
	}
	if r.pyDirectories[p] {
		return lang.Target{Local: p}, true
	}
	return lang.Target{}, false
}

// Implements: REQ-PY-010
func (r *resolver) distribution(parts []string) lang.Target {
	top := parts[0]
	candidates := []string{top, "python-" + top, "py" + top, top + "-python"}
	if len(parts) > 1 {
		if a, ok := importAliases[parts[0]+"."+parts[1]]; ok {
			candidates = append([]string{a}, candidates...)
		}
	}
	if a, ok := importAliases[top]; ok {
		candidates = append([]string{a}, candidates...)
	}
	for _, c := range candidates {
		if d := r.distMap[normalize(c)]; d != nil {
			return r.declared(d)
		}
	}
	// What the environment says provides the import settles what the names could not:
	// a distribution declared under a name its modules do not share, or one no index
	// has that is installed all the same. One an index has and nothing declares is
	// still undeclared, under its own name now rather than a guess.
	// Implements: REQ-PY-015
	if in := r.environment.provider(parts); in != nil {
		if d := r.distMap[normalize(in.name)]; d != nil {
			return r.declared(d)
		}
		if in.origin != "" {
			return lang.Target{Ecosystem: lang.EcosystemPyPI, Package: in.name, Version: in.version, Origin: in.origin}
		}
		return lang.Target{Ecosystem: lang.EcosystemPyPI, Package: in.name, Unresolved: true}
	}
	return lang.Target{Ecosystem: lang.EcosystemPyPI, Package: candidates[0], Unresolved: true}
}

// declared is the target of a distribution the project declares. Installed from
// outside any index, it says where from, and takes the installed version when the
// project names none.
func (r *resolver) declared(d *dist) lang.Target {
	t := lang.Target{Ecosystem: lang.EcosystemPyPI, Package: d.name, Version: d.version, Requested: d.requested, Pinned: d.pinned, Git: d.git}
	if in := r.environment.get(d.name); in != nil && in.origin != "" {
		t.Origin = in.origin
		if t.Version == "" {
			t.Version = in.version
		}
	}
	return t
}

var separators = regexp.MustCompile(`[-_.]+`)

// normalize applies PEP 503 name normalization.
func normalize(name string) string {
	return strings.ToLower(separators.ReplaceAllString(name, "-"))
}

var requirementNamePattern = regexp.MustCompile(`^\s*([A-Za-z0-9][A-Za-z0-9._-]*)\s*(\[[^\]]*\])?\s*(.*)$`)

// addRequirement records a PEP 508 requirement such as "requests[socks]>=2.0; python_version>'3'".
func (r *resolver) addRequirement(requirement string) {
	requirement, _, _ = strings.Cut(requirement, "#")
	requirement, _, _ = strings.Cut(requirement, ";")
	m := requirementNamePattern.FindStringSubmatch(requirement)
	if m == nil {
		return
	}
	r.addDist(m[1], strings.TrimSpace(m[3]), false)
}

// addDist records a distribution and what its specifier says about the version:
// "==1.2.3" and a lock file name one, ">=2.0" and "*" do not. locked marks entries
// read from a lock file, which are read after the manifests and win.
//
// Implements: REQ-SUP-003, REQ-PY-009, REQ-PY-013
func (r *resolver) addDist(name, spec string, locked bool) {
	if name == "" || strings.EqualFold(name, "python") {
		return
	}
	key := normalize(name)
	d := r.distMap[key]
	if d == nil {
		d = &dist{name: name}
		r.distMap[key] = d
	}
	spec = strings.TrimSpace(spec)
	if spec == "" || spec == "*" {
		return
	}
	version := strings.TrimSpace(strings.TrimPrefix(spec, "=="))
	switch {
	case locked:
		if d.version != "" && d.version != version {
			d.requested = d.version // what the manifest asked for before the lock
		}
		d.version, d.pinned = version, true
	case !d.pinned: // a range must not loosen what a lock already fixed
		d.version, d.pinned = version, lang.Pinned(spec)
		if g := directCommit(spec); g != "" {
			d.pinned, d.git = true, g
		}
	}
}

// directCommit is the checkout a PEP 508 direct reference to a git commit names
// ("@ git+https://github.com/o/r@<sha>", a "#egg=" fragment allowed), as
// "<repository URL>#<commit>"; "" for anything else. Such a reference installs
// that commit and nothing else, so it pins.
//
// Implements: REQ-PY-013, REQ-FND-026
func directCommit(spec string) string {
	reference, ok := strings.CutPrefix(strings.TrimSpace(spec), "@")
	if !ok {
		return ""
	}
	reference, _, _ = strings.Cut(strings.TrimSpace(reference), "#")
	reference, _, _ = strings.Cut(reference, ";") // an environment marker
	u, ok := strings.CutPrefix(strings.TrimSpace(reference), "git+")
	if !ok {
		return ""
	}
	i := strings.LastIndex(u, "@")
	if i < 0 || !lang.Commit(u[i+1:]) || !strings.Contains(u[:i], "/") {
		return ""
	}
	return u[:i] + "#" + u[i+1:]
}

// Implements: REQ-PY-006
func (r *resolver) readRequirements(absolute string) {
	data, err := os.ReadFile(absolute)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "#") {
			r.addRequirement(line)
		}
	}
}

// Implements: REQ-PY-007
func (r *resolver) readPyproject(absolute string) {
	var doc struct {
		Project struct {
			Dependencies         []string
			OptionalDependencies map[string][]string `toml:"optional-dependencies"`
		}
		DependencyGroups map[string][]any `toml:"dependency-groups"`
		Tool             struct {
			Poetry struct {
				Dependencies    map[string]any
				DevDependencies map[string]any `toml:"dev-dependencies"`
				Group           map[string]struct{ Dependencies map[string]any }
			}
		}
	}
	if _, err := toml.DecodeFile(absolute, &doc); err != nil {
		return
	}
	for _, d := range doc.Project.Dependencies {
		r.addRequirement(d)
	}
	for _, group := range doc.Project.OptionalDependencies {
		for _, d := range group {
			r.addRequirement(d)
		}
	}
	for _, group := range doc.DependencyGroups {
		for _, d := range group {
			if s, ok := d.(string); ok { // other entries are {include-group = ...}
				r.addRequirement(s)
			}
		}
	}
	r.addTable(doc.Tool.Poetry.Dependencies)
	r.addTable(doc.Tool.Poetry.DevDependencies)
	for _, g := range doc.Tool.Poetry.Group {
		r.addTable(g.Dependencies)
	}
}

// readSetupConfig reads [options] install_requires and [options.extras_require] from a
// setuptools setup.cfg (INI with indented continuation lines).
//
// Implements: REQ-PY-011
func (r *resolver) readSetupConfig(absolute string) {
	data, err := os.ReadFile(absolute)
	if err != nil {
		return
	}
	section, key := "", ""
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";"):
			continue
		case strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"):
			section, key = strings.ToLower(strings.Trim(trimmed, "[]")), ""
			continue
		case line[0] != ' ' && line[0] != '\t': // "key = value" starts a new key
			k, v, _ := strings.Cut(trimmed, "=")
			key, trimmed = strings.TrimSpace(k), strings.TrimSpace(v)
		}
		if (section == "options" && key == "install_requires") || section == "options.extras_require" {
			if trimmed != "" {
				r.addRequirement(trimmed)
			}
		}
	}
}

var (
	setupList   = regexp.MustCompile(`(?s)install_requires\s*=\s*\[(.*?)\]`)
	setupExtras = regexp.MustCompile(`(?s)extras_require\s*=\s*\{(.*?)\}`)
	extrasList  = regexp.MustCompile(`(?s)\[(.*?)\]`)
	pyString    = regexp.MustCompile(`["']([^"']+)["']`)
)

// readSetupPy reads literal install_requires / extras_require lists from setup.py;
// requirements computed at run time cannot be seen without executing it.
//
// Implements: REQ-PY-012
func (r *resolver) readSetupPy(absolute string) {
	data, err := os.ReadFile(absolute)
	if err != nil {
		return
	}
	var lists []string
	if m := setupList.FindSubmatch(data); m != nil {
		lists = append(lists, string(m[1]))
	}
	if m := setupExtras.FindSubmatch(data); m != nil {
		for _, l := range extrasList.FindAllSubmatch(m[1], -1) { // values only, not the dict's keys
			lists = append(lists, string(l[1]))
		}
	}
	for _, l := range lists {
		for _, s := range pyString.FindAllStringSubmatch(l, -1) {
			r.addRequirement(s[1])
		}
	}
}

// Implements: REQ-PY-008
func (r *resolver) readPipfile(absolute string) {
	var doc struct {
		Packages    map[string]any
		DevPackages map[string]any `toml:"dev-packages"`
	}
	if _, err := toml.DecodeFile(absolute, &doc); err == nil {
		r.addTable(doc.Packages)
		r.addTable(doc.DevPackages)
	}
}

// addTable records Poetry/Pipfile style tables: name = "^1.0" or name = {version = "^1.0", ...}.
func (r *resolver) addTable(t map[string]any) {
	for name, v := range t {
		version := ""
		switch v := v.(type) {
		case string:
			version = v
		case map[string]any:
			version, _ = v["version"].(string)
		}
		r.addDist(name, version, false)
	}
}

// Implements: REQ-PY-009
func (r *resolver) readLock(f *scan.File) {
	if path.Base(f.Path) == "Pipfile.lock" {
		var doc map[string]json.RawMessage
		data, err := os.ReadFile(f.AbsolutePath)
		if err != nil || json.Unmarshal(data, &doc) != nil {
			return
		}
		for _, section := range []string{"default", "develop"} {
			var packages map[string]struct{ Version string }
			if json.Unmarshal(doc[section], &packages) == nil {
				for name, p := range packages {
					r.addDist(name, p.Version, true)
				}
			}
		}
		return
	}
	var doc struct {
		Package []struct {
			Name, Version string
			// What the distribution itself needs, written differently by every tool:
			// a table of name -> constraint (poetry), a list of {name = …} tables
			// (uv), or a list of requirement strings (pdm).
			Dependencies any
		}
	}
	if _, err := toml.DecodeFile(f.AbsolutePath, &doc); err == nil {
		for _, p := range doc.Package {
			r.addDist(p.Name, p.Version, true)
			for _, dependency := range lockDependencies(p.Dependencies) {
				if dependency != "" && !strings.EqualFold(dependency, p.Name) {
					key := normalize(p.Name)
					r.tree[key] = append(r.tree[key], dependency)
				}
			}
		}
	}
}

// lockDependencies reads the names out of whichever shape a lock file wrote.
func lockDependencies(v any) []string {
	var out []string
	switch v := v.(type) {
	case map[string]any: // poetry: certifi = ">=2017.4.17"
		for name := range v {
			out = append(out, name)
		}
	case []any:
		for _, item := range v {
			switch item := item.(type) {
			case map[string]any: // uv: { name = "certifi" }
				if name, _ := item["name"].(string); name != "" {
					out = append(out, name)
				}
			case string: // pdm: "certifi>=2017.4.17; python_version >= '3'"
				out = append(out, requirementName(item))
			}
		}
	}
	return out
}

// requirementName takes the distribution name off the front of a requirement string.
func requirementName(requirement string) string {
	if i := strings.IndexAny(requirement, " <>=!~[;("); i >= 0 {
		requirement = requirement[:i]
	}
	return strings.TrimSpace(requirement)
}

// Dependencies implements lang.Transitive from the lock files the project carries.
//
// Implements: REQ-SUP-009
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != lang.EcosystemPyPI {
		return nil
	}
	if _, locked := r.tree[normalize(t.Package)]; !locked {
		return r.installedDependencies(t)
	}
	var out []lang.Target
	for _, dependency := range r.tree[normalize(t.Package)] {
		name, version, pinned := dependency, "", false
		if d := r.distMap[normalize(dependency)]; d != nil {
			name, version, pinned = d.name, d.version, d.pinned
		}
		out = append(out, lang.Target{Ecosystem: lang.EcosystemPyPI, Package: name, Version: version, Pinned: pinned})
	}
	return out
}

// Installed implements lang.Installed: whether Dependencies answers for t from the
// environment rather than from a lock file.
func (r *resolver) Installed(t lang.Target) bool {
	_, locked := r.tree[normalize(t.Package)]
	in := r.environment.get(t.Package)
	return !locked && in != nil && in.origin != ""
}

// installedDependencies answers for a distribution no lock file covers and no index
// can be asked about, because it was installed from somewhere else: what its own
// metadata requires, as the environment has it installed.
//
// Implements: REQ-PY-015
func (r *resolver) installedDependencies(t lang.Target) []lang.Target {
	in := r.environment.get(t.Package)
	if in == nil || in.origin == "" {
		return nil
	}
	var out []lang.Target
	for _, requirement := range in.requires {
		switch dependency := r.environment.get(requirement); {
		case r.distMap[normalize(requirement)] != nil:
			out = append(out, r.declared(r.distMap[normalize(requirement)]))
		case dependency != nil:
			out = append(out, lang.Target{Ecosystem: lang.EcosystemPyPI, Package: dependency.name, Version: dependency.version, Origin: dependency.origin})
		default:
			out = append(out, lang.Target{Ecosystem: lang.EcosystemPyPI, Package: requirement})
		}
	}
	return out
}

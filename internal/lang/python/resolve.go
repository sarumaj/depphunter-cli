package python

import (
	"encoding/json"
	"os"
	"path"
	"regexp"
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
}

type resolver struct {
	files   map[string]bool
	pyDirs  map[string]bool // directories containing Python files at any depth
	roots   []string        // import roots, most specific first, "." last
	distMap map[string]*dist
	// tree maps a normalized distribution name to what a lock file says it needs.
	tree map[string][]string
	// env is the interpreter's installed distributions, or nil (findEnvironment).
	env *environment
}

// Implements: REQ-PY-003, REQ-PY-006, REQ-PY-009
func newResolver(all, claimed []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, pyDirs: map[string]bool{}, distMap: map[string]*dist{}, tree: map[string][]string{}}
	for _, f := range claimed {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); d != "." && !r.pyDirs[d]; d = path.Dir(d) {
			r.pyDirs[d] = true
		}
	}

	roots := map[string]bool{".": true}
	var locks []*scan.File
	for _, f := range all {
		dir, base := path.Dir(f.Path), path.Base(f.Path)
		switch {
		case base == "pyproject.toml":
			roots[dir] = true
			r.readPyproject(f.Abs)
		case base == "setup.cfg":
			roots[dir] = true
			r.readSetupCfg(f.Abs)
		case base == "setup.py":
			roots[dir] = true
			r.readSetupPy(f.Abs)
		case base == "Pipfile":
			r.readPipfile(f.Abs)
		case base == "poetry.lock" || base == "uv.lock" || base == "pdm.lock" || base == "Pipfile.lock":
			locks = append(locks, f)
		case strings.HasSuffix(base, ".txt") && (strings.HasPrefix(base, "requirements") || path.Base(dir) == "requirements"):
			r.readRequirements(f.Abs)
		}
	}
	for _, f := range locks { // after manifests, so pinned versions override ranges
		r.readLock(f)
	}
	for d := range roots {
		r.roots = append(r.roots, d)
		if src := path.Join(d, "src"); r.pyDirs[src] {
			r.roots = append(r.roots, src)
		}
	}
	// Deepest first, so a sub-project's modules shadow same-named top-level ones; "." last.
	depth := func(d string) int {
		if d == "." {
			return -1
		}
		return strings.Count(d, "/")
	}
	sort.Slice(r.roots, func(i, j int) bool {
		if di, dj := depth(r.roots[i]), depth(r.roots[j]); di != dj {
			return di > dj
		}
		return r.roots[i] < r.roots[j]
	})
	return r
}

// Resolve handles both "import a.b" (Name empty) and "from m import n".
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	return r.resolveFrom(imp.Module, imp.Name, file)
}

// resolve handles "import a.b.c".
//
// Implements: REQ-PY-003, REQ-PY-004, REQ-PY-005
func (r *resolver) resolve(dotted, file string) lang.Target {
	parts := strings.Split(dotted, ".")
	for _, root := range r.roots {
		if t, ok := r.longest(root, parts); ok {
			return t
		}
	}
	if stdlib[parts[0]] {
		return lang.Target{Ecosystem: ecoStd, Package: parts[0]}
	}
	// A script's own directory is on sys.path when it runs.
	if t, ok := r.probe(path.Dir(file), parts); ok {
		return t
	}
	return r.distribution(parts)
}

// resolveFrom handles "from mod import name"; name may be a sub-module.
//
// Implements: REQ-PY-002
func (r *resolver) resolveFrom(mod, name, file string) lang.Target {
	if !strings.HasPrefix(mod, ".") {
		if name != "" {
			return r.resolve(mod+"."+name, file)
		}
		return r.resolve(mod, file)
	}
	rest := strings.TrimLeft(mod, ".")
	base := path.Dir(file)
	for range len(mod) - len(rest) - 1 {
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
	if r.pyDirs[p] {
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
	if in := r.env.provider(parts); in != nil {
		if d := r.distMap[normalize(in.name)]; d != nil {
			return r.declared(d)
		}
		if in.origin != "" {
			return lang.Target{Ecosystem: ecoPyPI, Package: in.name, Version: in.version, Origin: in.origin}
		}
		return lang.Target{Ecosystem: ecoPyPI, Package: in.name, Unresolved: true}
	}
	return lang.Target{Ecosystem: ecoPyPI, Package: candidates[0], Unresolved: true}
}

// declared is the target of a distribution the project declares. Installed from
// outside any index, it says where from, and takes the installed version when the
// project names none.
func (r *resolver) declared(d *dist) lang.Target {
	t := lang.Target{Ecosystem: ecoPyPI, Package: d.name, Version: d.version, Requested: d.requested, Pinned: d.pinned}
	if in := r.env.get(d.name); in != nil && in.origin != "" {
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

var reqName = regexp.MustCompile(`^\s*([A-Za-z0-9][A-Za-z0-9._-]*)\s*(\[[^\]]*\])?\s*(.*)$`)

// addRequirement records a PEP 508 requirement such as "requests[socks]>=2.0; python_version>'3'".
func (r *resolver) addRequirement(req string) {
	req, _, _ = strings.Cut(req, "#")
	req, _, _ = strings.Cut(req, ";")
	m := reqName.FindStringSubmatch(req)
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
	}
}

// Implements: REQ-PY-006
func (r *resolver) readRequirements(abs string) {
	data, err := os.ReadFile(abs)
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
func (r *resolver) readPyproject(abs string) {
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
	if _, err := toml.DecodeFile(abs, &doc); err != nil {
		return
	}
	for _, d := range doc.Project.Dependencies {
		r.addRequirement(d)
	}
	for _, ds := range doc.Project.OptionalDependencies {
		for _, d := range ds {
			r.addRequirement(d)
		}
	}
	for _, ds := range doc.DependencyGroups {
		for _, d := range ds {
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

// readSetupCfg reads [options] install_requires and [options.extras_require] from a
// setuptools setup.cfg (INI with indented continuation lines).
//
// Implements: REQ-PY-011
func (r *resolver) readSetupCfg(abs string) {
	data, err := os.ReadFile(abs)
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
func (r *resolver) readSetupPy(abs string) {
	data, err := os.ReadFile(abs)
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
func (r *resolver) readPipfile(abs string) {
	var doc struct {
		Packages    map[string]any
		DevPackages map[string]any `toml:"dev-packages"`
	}
	if _, err := toml.DecodeFile(abs, &doc); err == nil {
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
		data, err := os.ReadFile(f.Abs)
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
	if _, err := toml.DecodeFile(f.Abs, &doc); err == nil {
		for _, p := range doc.Package {
			r.addDist(p.Name, p.Version, true)
			for _, dep := range lockDependencies(p.Dependencies) {
				if dep != "" && !strings.EqualFold(dep, p.Name) {
					key := normalize(p.Name)
					r.tree[key] = append(r.tree[key], dep)
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
func requirementName(req string) string {
	if i := strings.IndexAny(req, " <>=!~[;("); i >= 0 {
		req = req[:i]
	}
	return strings.TrimSpace(req)
}

// Dependencies implements lang.Transitive from the lock files the project carries.
//
// Implements: REQ-SUP-009
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoPyPI {
		return nil
	}
	if _, locked := r.tree[normalize(t.Package)]; !locked {
		return r.installedDependencies(t)
	}
	var out []lang.Target
	for _, dep := range r.tree[normalize(t.Package)] {
		name, version, pinned := dep, "", false
		if d := r.distMap[normalize(dep)]; d != nil {
			name, version, pinned = d.name, d.version, d.pinned
		}
		out = append(out, lang.Target{Ecosystem: ecoPyPI, Package: name, Version: version, Pinned: pinned})
	}
	return out
}

// Installed implements lang.Installed: whether Dependencies answers for t from the
// environment rather than from a lock file.
func (r *resolver) Installed(t lang.Target) bool {
	_, locked := r.tree[normalize(t.Package)]
	in := r.env.get(t.Package)
	return !locked && in != nil && in.origin != ""
}

// installedDependencies answers for a distribution no lock file covers and no index
// can be asked about, because it was installed from somewhere else: what its own
// metadata requires, as the environment has it installed.
//
// Implements: REQ-PY-015
func (r *resolver) installedDependencies(t lang.Target) []lang.Target {
	in := r.env.get(t.Package)
	if in == nil || in.origin == "" {
		return nil
	}
	var out []lang.Target
	for _, req := range in.requires {
		switch dep := r.env.get(req); {
		case r.distMap[normalize(req)] != nil:
			out = append(out, r.declared(r.distMap[normalize(req)]))
		case dep != nil:
			out = append(out, lang.Target{Ecosystem: ecoPyPI, Package: dep.name, Version: dep.version, Origin: dep.origin})
		default:
			out = append(out, lang.Target{Ecosystem: ecoPyPI, Package: req})
		}
	}
	return out
}

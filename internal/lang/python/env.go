package python

import (
	"bufio"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// An environment is what one Python interpreter has installed: the distributions in
// its site-packages directories, read off the disk. Only one kind of them is used to
// resolve imports - the kind no package index has (installed), which a repository
// cannot declare in a way anything else could look up.
//
// The interpreter is never run. It may be the repository's own .venv, and running a
// program a repository ships is running the repository; everything needed is in the
// files an installer leaves behind (the .dist-info directories).
type environment struct {
	// sites are the site-packages directories read, most specific first.
	sites []string
	// byName holds every distribution, by normalized name.
	byName map[string]*installed
	// modules maps a dotted module path an installed distribution provides ("acme",
	// "google.cloud.storage") to it; nil where two distributions provide it, as the
	// namespace packages do at their top.
	modules map[string]*installed
}

// installed is one distribution in an environment.
type installed struct {
	name     string
	version  string
	requires []string // Requires-Dist names, without those only an extra asks for
	// origin is where it was installed from when that was not an index - a local
	// directory (editable or not), an archive or a VCS URL - from its direct_url.json
	// (PEP 610). Empty for anything an installer fetched from an index.
	origin string
}

// Environment settings: the interpreter to read, as a path; the variable an activated
// virtual environment sets; and the directories a project keeps its own in.
const envVirtual = "VIRTUAL_ENV"

var projectEnvs = []string{".venv", "venv"}

// findEnvironment picks the interpreter whose site-packages to read, in this order: the
// one configured (--python), the activated virtual environment, the project's own .venv
// or venv. Nothing is read when there is none of them: an interpreter somebody merely
// has on PATH was not given, and what it happens to have installed would make the map
// depend on the machine it was drawn on.
//
// Implements: REQ-PY-015
func findEnvironment(root, interpreter string, getenv func(string) string) *environment {
	var prefix string
	switch {
	case interpreter != "":
		prefix = prefixOf(interpreter)
	case getenv != nil && getenv(envVirtual) != "":
		prefix = getenv(envVirtual)
	default:
		for _, d := range projectEnvs {
			if isVenv(filepath.Join(root, d)) {
				prefix = filepath.Join(root, d)
				break
			}
		}
	}
	if prefix == "" {
		return nil
	}
	return readEnvironment(sitesOf(prefix, versionOf(interpreter)))
}

// prefixOf is the installation an interpreter belongs to: the virtual environment it
// is in (bin/python beside a pyvenv.cfg one level up), or else the prefix of the
// program it is a link to (/usr for /usr/bin/python3.12).
func prefixOf(interpreter string) string {
	abs, err := filepath.Abs(interpreter)
	if err != nil {
		return ""
	}
	if venv := filepath.Dir(filepath.Dir(abs)); isVenv(venv) {
		return venv
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	return filepath.Dir(filepath.Dir(abs))
}

func isVenv(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "pyvenv.cfg"))
	return err == nil && !st.IsDir()
}

var pythonVersion = regexp.MustCompile(`python(3\.\d+)`)

// versionOf reads the version an interpreter's name gives ("python3.12"), which picks
// its own lib/python3.12 from a prefix that has several. "" matches all of them.
func versionOf(interpreter string) string {
	if interpreter == "" {
		return ""
	}
	name := filepath.Base(interpreter)
	if real, err := filepath.EvalSymlinks(interpreter); err == nil {
		name = filepath.Base(real)
	}
	if m := pythonVersion.FindStringSubmatch(name); m != nil {
		return m[1]
	}
	return ""
}

// sitesOf lists the site-packages directories of an installation prefix: POSIX
// (lib/pythonX.Y), Debian's dist-packages, and Windows (Lib). A virtual environment
// that includes the system site-packages (pyvenv.cfg) adds its base interpreter's.
func sitesOf(prefix, version string) []string {
	ver := "3.*"
	if version != "" {
		ver = version
	}
	var sites []string
	for _, pattern := range []string{
		"lib/python" + ver + "/site-packages", "lib64/python" + ver + "/site-packages",
		"lib/python3/dist-packages", "local/lib/python" + ver + "/dist-packages",
		"Lib/site-packages",
	} {
		matches, _ := filepath.Glob(filepath.Join(prefix, filepath.FromSlash(pattern)))
		sort.Sort(sort.Reverse(sort.StringSlice(matches))) // the newest Python first
		sites = append(sites, matches...)
	}
	if cfg := readPyvenvCfg(filepath.Join(prefix, "pyvenv.cfg")); cfg["include-system-site-packages"] == "true" && cfg["home"] != "" {
		// home is the directory the base interpreter is in (/usr/bin).
		base := filepath.Dir(cfg["home"])
		if version == "" {
			version = majorMinor(cfg["version"])
		}
		sites = append(sites, sitesOf(base, version)...)
	}
	return sites
}

func readPyvenvCfg(file string) map[string]string {
	out := map[string]string{}
	data, err := os.ReadFile(file)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if ok {
			out[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
		}
	}
	// Written as version or, by newer versions of venv, as version_info.
	if out["version"] == "" {
		out["version"] = out["version_info"]
	}
	return out
}

func majorMinor(v string) string {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "." + parts[1]
}

// readEnvironment reads the .dist-info directories of the given sites. A distribution
// found in an earlier site shadows one of the same name in a later one, as it would on
// sys.path.
func readEnvironment(sites []string) *environment {
	env := &environment{sites: sites, byName: map[string]*installed{}, modules: map[string]*installed{}}
	claimed := map[string]map[string]bool{} // module -> distributions providing it
	for _, site := range sites {
		entries, err := os.ReadDir(site)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || !strings.HasSuffix(e.Name(), ".dist-info") {
				continue
			}
			info := filepath.Join(site, e.Name())
			d := readDistInfo(info)
			if d == nil || env.byName[normalize(d.name)] != nil {
				continue
			}
			env.byName[normalize(d.name)] = d
			for _, m := range providedModules(site, info) {
				if claimed[m] == nil {
					claimed[m] = map[string]bool{}
				}
				claimed[m][normalize(d.name)] = true
			}
		}
	}
	for m, by := range claimed {
		if len(by) != 1 {
			env.modules[m] = nil
			continue
		}
		for name := range by {
			env.modules[m] = env.byName[name]
		}
	}
	return env
}

// readDistInfo reads a distribution's name, version and requirements from METADATA,
// and where it came from from direct_url.json.
func readDistInfo(info string) *installed {
	f, err := os.Open(filepath.Join(info, "METADATA"))
	if err != nil {
		return nil
	}
	defer f.Close()
	d := &installed{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			break // the headers end here, and the description begins
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.ToLower(k) {
		case "name":
			d.name = v
		case "version":
			d.version = v
		case "requires-dist":
			// A requirement only an extra asks for is not installed by asking for
			// the distribution itself.
			if _, marker, ok := strings.Cut(v, ";"); ok && strings.Contains(marker, "extra") {
				continue
			}
			if name := requirementName(v); name != "" {
				d.requires = append(d.requires, name)
			}
		}
	}
	if d.name == "" {
		return nil
	}
	if data, err := os.ReadFile(filepath.Join(info, "direct_url.json")); err == nil {
		var u struct {
			URL string `json:"url"`
		}
		if json.Unmarshal(data, &u) == nil {
			d.origin = u.URL
		}
	}
	return d
}

// editableMapping is the table setuptools' editable finder keeps of the packages it
// makes importable: MAPPING = {'acme': '/home/me/acme/src/acme'}.
var editableMapping = regexp.MustCompile(`(?s)MAPPING\s*(?::[^=]*)?=\s*\{(.*?)\}`)
var mappingKey = regexp.MustCompile(`['"]([A-Za-z_][\w.]*)['"]\s*:`)

// providedModules lists the dotted module paths a distribution makes importable: from
// the files it installed (RECORD), every package and module and the packages above
// them; from top_level.txt when there is no RECORD; and, for an editable install, from
// the path file or finder that points back at its source.
func providedModules(site, info string) []string {
	seen := map[string]bool{}
	add := func(parts []string) {
		for n := 1; n <= len(parts); n++ {
			seen[strings.Join(parts[:n], ".")] = true
		}
	}
	record, err := os.ReadFile(filepath.Join(info, "RECORD"))
	if err == nil {
		for _, line := range strings.Split(string(record), "\n") {
			file, _, _ := strings.Cut(line, ",")
			file = strings.Trim(file, `"`)
			switch {
			case file == "" || strings.HasPrefix(file, "..") || strings.Contains(file, ".dist-info/") ||
				strings.Contains(file, "__pycache__"):
			case strings.HasSuffix(file, ".pth"):
				for _, dir := range pthDirs(filepath.Join(site, file)) {
					for _, m := range modulesIn(dir) {
						add([]string{m})
					}
				}
			case strings.HasPrefix(path.Base(file), "__editable__") && strings.HasSuffix(file, "_finder.py"):
				if src, err := os.ReadFile(filepath.Join(site, file)); err == nil {
					if m := editableMapping.FindSubmatch(src); m != nil {
						for _, k := range mappingKey.FindAllSubmatch(m[1], -1) {
							add(strings.Split(string(k[1]), "."))
						}
					}
				}
			case strings.HasSuffix(file, ".py") || strings.HasSuffix(file, ".pyi") || strings.HasSuffix(file, ".so") || strings.HasSuffix(file, ".pyd"):
				parts := strings.Split(file, "/")
				last := parts[len(parts)-1]
				if last == "__init__.py" || last == "__init__.pyi" {
					parts = parts[:len(parts)-1]
				} else {
					parts[len(parts)-1], _, _ = strings.Cut(last, ".")
				}
				if len(parts) > 0 && !strings.HasPrefix(parts[0], "__editable__") {
					add(parts)
				}
			}
		}
	}
	if len(seen) == 0 {
		if top, err := os.ReadFile(filepath.Join(info, "top_level.txt")); err == nil {
			for _, m := range strings.Fields(string(top)) {
				add(strings.Split(m, "/"))
			}
		}
	}
	out := make([]string, 0, len(seen))
	for m := range seen {
		out = append(out, m)
	}
	return out
}

// pthDirs reads the directories a .pth file adds to sys.path; its import lines are
// code, and are not.
func pthDirs(file string) []string {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "import ") {
			continue
		}
		if st, err := os.Stat(line); err == nil && st.IsDir() {
			out = append(out, line)
		}
	}
	return out
}

// modulesIn lists the top-level packages and modules in a directory on sys.path.
func modulesIn(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		switch {
		case e.IsDir() && isIdentifier(name):
			if _, err := os.Stat(filepath.Join(dir, name, "__init__.py")); err == nil {
				out = append(out, name)
			}
		case strings.HasSuffix(name, ".py") && isIdentifier(strings.TrimSuffix(name, ".py")):
			out = append(out, strings.TrimSuffix(name, ".py"))
		}
	}
	return out
}

var identifier = regexp.MustCompile(`^[A-Za-z_]\w*$`)

func isIdentifier(s string) bool { return identifier.MatchString(s) && s != "setup" }

// get is the installed distribution of that name, or nil.
func (e *environment) get(name string) *installed {
	if e == nil {
		return nil
	}
	return e.byName[normalize(name)]
}

// provider is the distribution that provides the longest leading part of a dotted
// import, when exactly one does.
func (e *environment) provider(parts []string) *installed {
	if e == nil {
		return nil
	}
	for n := len(parts); n > 0; n-- {
		if d, ok := e.modules[strings.Join(parts[:n], ".")]; ok {
			return d // nil when several share it: then there is no telling
		}
	}
	return nil
}

package ruby

import (
	"cmp"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// A Bundler project is a directory with a Gemfile (or gems.rb) and the gemspecs it
// takes in with `gemspec`: what they declare, with the requirements they write, and
// what Gemfile.lock (gems.locked) resolved it all to - every gem installed, direct or
// not, with its exact version and what it depends on. A gem library usually commits
// no lock; its gemspec's dependencies are then all there is.

// declaration is a gem a Gemfile or gemspec names.
type declaration struct {
	name        string
	requirement string // "~> 7.0, >= 7.0.4"; "" for any version
	origin      string // git: or path: source
	commit      string // the full SHA a git origin is pinned to (ref:), else ""
}

// spec is a gem Gemfile.lock records.
type spec struct {
	name, version string
	dependencies  map[string]string // name -> requirement
	origin        string            // remote of a GIT or PATH section; "" for a gem server
	commit        string            // a GIT section's revision, when it is a full SHA
	path          bool              // PATH section: the gem's code is on disk
}

type project struct {
	directory string
	declared  map[string]*declaration // lower-case name
	locked    map[string]*spec        // lower-case name
	loadPath  []string                // lib directories of gems whose code is in the repository
	own       map[string]string       // gems this repository builds -> their gemspec or directory
}

var (
	gemLine     = regexp.MustCompile(`^\s*gem[\s(]`)
	gemspecLine = regexp.MustCompile(`^\s*gemspec\b(.*)$`)
	blockOpen   = regexp.MustCompile(`^(\s*)(source|path|git|github)\b(.*)\bdo\s*(\|.*\|)?\s*$`)
	blockEnd    = regexp.MustCompile(`^(\s*)end\b`)
	evalGemfile = regexp.MustCompile(`^\s*eval_gemfile\s*\(?\s*(.+?)\s*\)?\s*$`)
)

// readGemfile reads the gems a Gemfile declares, and the gemspec directories it takes
// in, line by line: `gem` lines (a trailing comma continues one), `gemspec`, and the
// `path`, `git` and `github` blocks whose gems come from there. Nothing is run.
//
// Implements: REQ-RUBY-007
func readGemfile(source string, read func(relative string) (string, bool)) (declarations []*declaration, gemspecDirectories []string) {
	type block struct {
		indent string
		kind   string
		value  string
	}
	var blocks []block
	lines := strings.Split(source, "\n")
	for i := 0; i < len(lines); i++ {
		line := stripComment(lines[i])
		for strings.HasSuffix(strings.TrimSpace(line), ",") && i+1 < len(lines) {
			i++
			line += " " + stripComment(lines[i])
		}
		if m := blockOpen.FindStringSubmatch(line); m != nil {
			v, _ := evalPath(strings.TrimSpace(strings.SplitN(m[3], ",", 2)[0]))
			blocks = append(blocks, block{m[1], m[2], v})
			continue
		}
		if m := blockEnd.FindStringSubmatch(line); m != nil && len(blocks) > 0 && blocks[len(blocks)-1].indent == m[1] {
			blocks = blocks[:len(blocks)-1]
			continue
		}
		if m := gemspecLine.FindStringSubmatch(line); m != nil {
			p, _ := option(splitArguments(m[1]), "path")
			gemspecDirectories = append(gemspecDirectories, p)
			continue
		}
		if m := evalGemfile.FindStringSubmatch(line); m != nil && read != nil {
			if p, ok := evalPath(m[1]); ok && !strings.HasPrefix(p, "__DIR__/_") {
				if data, ok := read(strings.TrimPrefix(strings.TrimPrefix(p, "__DIR__"), "/")); ok {
					more, directories := readGemfile(data, nil)
					declarations = append(declarations, more...)
					gemspecDirectories = append(gemspecDirectories, directories...)
				}
			}
			continue
		}
		if !gemLine.MatchString(line) {
			continue
		}
		arguments := splitArguments(strings.TrimSpace(line)[3:])
		if len(arguments) == 0 {
			continue
		}
		name, ok := literal(arguments[0])
		if !ok || name == "" {
			continue
		}
		d := &declaration{name: name}
		var requirements []string
		for _, a := range arguments[1:] {
			if v, ok := literal(a); ok {
				requirements = append(requirements, strings.TrimSpace(v))
			}
		}
		d.requirement = strings.Join(requirements, ", ")
		parsedOptions := options(arguments[1:])
		for _, b := range blocks {
			if b.kind != "source" && parsedOptions[b.kind] == "" {
				parsedOptions[b.kind] = b.value
			}
		}
		switch {
		case parsedOptions["path"] != "":
			d.origin = "path:" + parsedOptions["path"]
		case parsedOptions["git"] != "":
			d.origin = parsedOptions["git"]
		case parsedOptions["github"] != "":
			d.origin = "https://github.com/" + parsedOptions["github"] + ".git"
		}
		if d.origin != "" && lang.Commit(parsedOptions["ref"]) {
			d.commit = parsedOptions["ref"]
		}
		declarations = append(declarations, d)
	}
	return declarations, gemspecDirectories
}

// stripComment takes a `#` comment off a line, outside strings.
func stripComment(line string) string {
	var quote byte
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case quote != 0:
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '#':
			return line[:i]
		}
	}
	return line
}

var (
	specName       = regexp.MustCompile(`\.name\s*=\s*["']([^"']+)["']`)
	specDependency = regexp.MustCompile(`\.add_(?:runtime_|development_)?dependency\s*\(?\s*(.*)$`)
	requirePaths   = regexp.MustCompile(`\.require_paths?\s*=\s*(.*)$`)
)

// readGemspec reads a gemspec's name, its dependencies and the directories it puts on
// the load path (lib unless require_paths says otherwise), from its text.
//
// Implements: REQ-RUBY-007
func readGemspec(source string) (name string, declarations []*declaration, paths []string) {
	for _, line := range strings.Split(source, "\n") {
		line = stripComment(line)
		if m := specName.FindStringSubmatch(line); m != nil && name == "" {
			name = m[1]
		}
		if m := requirePaths.FindStringSubmatch(line); m != nil {
			for _, a := range splitArguments(strings.Trim(strings.TrimSpace(m[1]), "[]")) {
				if p, ok := literal(a); ok {
					paths = append(paths, p)
				}
			}
			if s := strings.TrimSpace(m[1]); strings.HasPrefix(s, "%w") && len(s) > 4 {
				paths = append(paths, strings.Fields(s[3:len(s)-1])...)
			}
		}
		m := specDependency.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		arguments := splitArguments(strings.TrimSuffix(strings.TrimSpace(m[1]), ")"))
		if len(arguments) == 0 {
			continue
		}
		dependency, ok := literal(arguments[0])
		if !ok || dependency == "" {
			continue
		}
		var requirements []string
		for _, a := range arguments[1:] {
			for _, item := range splitArguments(strings.Trim(strings.TrimSpace(a), "[]")) {
				if v, ok := literal(item); ok {
					requirements = append(requirements, strings.TrimSpace(v))
				}
			}
		}
		declarations = append(declarations, &declaration{name: dependency, requirement: strings.Join(requirements, ", ")})
	}
	if len(paths) == 0 {
		paths = []string{"lib"}
	}
	return name, declarations, paths
}

var (
	lockSpec       = regexp.MustCompile(`^    ([^\s(]+) \(([^)]+)\)$`)
	lockDependency = regexp.MustCompile(`^      ([^\s(]+)(?: \(([^)]+)\))?$`)
	lockTop        = regexp.MustCompile(`^  ([^\s(!]+)!?(?: \(([^)]+)\))?$`)
)

// readLock reads Gemfile.lock: the specs of its GEM, GIT and PATH sections with what
// each depends on, and the DEPENDENCIES the Gemfile asked for. A gem locked for
// several platforms (nokogiri 1.16.0-x86_64-linux and -arm64-darwin) is one gem at
// one version, the platform left out.
//
// Implements: REQ-RUBY-008, REQ-RUBY-011
func readLock(source string) (map[string]*spec, map[string]string) {
	specs := map[string]*spec{}
	direct := map[string]string{}
	section, remote, revision := "", "", ""
	var current *spec
	for _, line := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		if line != "" && line[0] != ' ' {
			section, remote, revision, current = strings.TrimSpace(line), "", "", nil
			continue
		}
		switch section {
		case "GEM", "GIT", "PATH", "PLUGIN SOURCE":
			trimmed := strings.TrimSpace(line)
			if v, ok := strings.CutPrefix(trimmed, "remote: "); ok && strings.HasPrefix(line, "  r") {
				remote = v
				continue
			}
			if v, ok := strings.CutPrefix(trimmed, "revision: "); ok {
				revision = v
				continue
			}
			if m := lockSpec.FindStringSubmatch(line); m != nil {
				key := strings.ToLower(m[1])
				if old := specs[key]; old != nil {
					current = old // another platform of the same gem
					continue
				}
				current = &spec{name: m[1], version: platformless(m[2]), dependencies: map[string]string{}}
				switch section {
				case "GIT":
					current.origin = remote
					if lang.Commit(revision) {
						current.commit = revision
					}
				case "PATH":
					current.origin, current.path = remote, true
				}
				specs[key] = current
				continue
			}
			if m := lockDependency.FindStringSubmatch(line); m != nil && current != nil {
				current.dependencies[m[1]] = m[2]
			}
		case "DEPENDENCIES":
			if m := lockTop.FindStringSubmatch(line); m != nil {
				direct[strings.ToLower(m[1])] = m[2]
			}
		}
	}
	return specs, direct
}

// platformless takes the platform off a locked version: RubyGems versions separate
// pre-releases with a dot (7.1.0.rc1), so a dash starts the platform.
func platformless(v string) string {
	if i := strings.IndexByte(v, '-'); i > 0 {
		return v[:i]
	}
	return v
}

// target is a gem as a project resolves it. The lock's version pins it, a git
// source by its revision; without a lock a requirement pins only when it names one
// version exactly ("1.2.3" or "= 1.2.3"), and a gem declared without any version
// floats.
//
// Implements: REQ-RUBY-009, REQ-FND-026
func (p *project) target(name string) lang.Target {
	key := strings.ToLower(name)
	d := p.declared[key]
	if s := p.locked[key]; s != nil {
		t := lang.Target{Ecosystem: ecosystemGems, Package: s.name, Version: s.version, Pinned: true}
		switch {
		case s.path:
			t.Origin, t.Pinned = "path:"+s.origin, false
		case s.origin != "":
			t.Origin, t.Pinned = s.origin, s.commit != ""
			if s.commit != "" {
				t.Git = s.origin + "#" + s.commit
			}
		}
		if d != nil && d.requirement != "" && d.requirement != s.version && d.requirement != "= "+s.version {
			t.Requested = d.requirement
		}
		return t
	}
	if d == nil {
		return lang.Target{Ecosystem: ecosystemGems, Package: name, Unresolved: true}
	}
	t := lang.Target{Ecosystem: ecosystemGems, Package: d.name, Version: d.requirement, Origin: d.origin}
	switch {
	case d.origin != "":
		t.Pinned, t.Floating = d.commit != "", d.commit == ""
		if d.commit != "" {
			t.Git = d.origin + "#" + d.commit
		}
	default:
		t.Version, t.Pinned = exactVersion(d.requirement)
		t.Floating = d.requirement == ""
	}
	return t
}

// exactVersion is the version a requirement names, bare ("= 1.2.3" is 1.2.3) when it
// pins one, and whether it does.
func exactVersion(requirement string) (string, bool) {
	if !pinned(requirement) {
		return requirement, false
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(requirement), "=")), true
}

// pinned is RubyGems' reading of a requirement: a bare version and "= 1.2.3" name
// one version; "~>", ">=", "<", "!=" and several requirements together float.
//
// Implements: REQ-RUBY-009
func pinned(requirement string) bool {
	r := strings.TrimSpace(requirement)
	if strings.Contains(r, ",") {
		return false
	}
	if rest, ok := strings.CutPrefix(r, "="); ok {
		r = strings.TrimSpace(rest)
	}
	return lang.Pinned(r) || gemVersion.MatchString(r)
}

// gemVersion is a RubyGems version with a pre-release part after a dot: 7.1.0.rc1.
var gemVersion = regexp.MustCompile(`^\d+(\.\d+)*\.(?i:alpha|beta|pre|rc|dev)\d*(\.\d+)*$`)

// readProject reads a Bundler project at directory: its Gemfile (if any), every gemspec in
// directory and in the directories its gemspec directives name, and its lock.
func readProject(directory, gemfile string, gemspecs map[string][]string, read func(relative string) (string, bool)) *project {
	p := &project{directory: directory, declared: map[string]*declaration{}, locked: map[string]*spec{}, own: map[string]string{}}
	declare := func(d *declaration) {
		key := strings.ToLower(d.name)
		if old, ok := p.declared[key]; !ok || old.requirement == "" && d.requirement != "" {
			p.declared[key] = d
		}
	}
	specDirectories := []string{directory}
	lockName := ""
	if gemfile != "" {
		source, _ := read(path.Join(directory, gemfile))
		declarations, directories := readGemfile(source, func(relative string) (string, bool) { return read(path.Join(directory, relative)) })
		for _, d := range declarations {
			declare(d)
			if relative, ok := strings.CutPrefix(d.origin, "path:"); ok {
				if gDirectory := path.Join(directory, relative); lang.Inside(gDirectory) {
					p.loadPath = append(p.loadPath, path.Join(gDirectory, "lib"))
					p.own[strings.ToLower(d.name)] = gDirectory
				}
			}
		}
		for _, d := range directories {
			if d = path.Join(directory, d); d != directory {
				specDirectories = append(specDirectories, d)
			}
		}
		lockName = "Gemfile.lock"
		if gemfile == "gems.rb" {
			lockName = "gems.locked"
		}
	}
	for _, d := range specDirectories {
		for _, f := range gemspecs[d] {
			source, _ := read(f)
			name, declarations, paths := readGemspec(source)
			name = cmp.Or(name, strings.TrimSuffix(path.Base(f), ".gemspec"))
			p.own[strings.ToLower(name)] = f
			for _, rp := range paths {
				p.loadPath = append(p.loadPath, path.Join(d, rp))
			}
			for _, dd := range declarations {
				declare(dd)
			}
		}
	}
	if lockName != "" {
		if source, ok := read(path.Join(directory, lockName)); ok {
			specs, direct := readLock(source)
			p.locked = specs
			for name, s := range specs {
				if s.path {
					if gDirectory := path.Join(directory, s.origin); lang.Inside(gDirectory) {
						p.loadPath = append(p.loadPath, path.Join(gDirectory, "lib"))
						if _, ok := p.own[name]; !ok {
							p.own[name] = gDirectory
						}
					}
				}
			}
			for name, requirement := range direct {
				if _, ok := p.declared[name]; !ok {
					p.declared[name] = &declaration{name: name, requirement: requirement}
				}
			}
		}
	}
	sort.Strings(p.loadPath)
	return p
}

// has reports whether the project declares or locks a gem.
func (p *project) has(name string) bool {
	key := strings.ToLower(name)
	return p.declared[key] != nil || p.locked[key] != nil
}

// gemNames lists every gem the project declares or locks, sorted.
func (p *project) gemNames() []string {
	seen := map[string]bool{}
	var out []string
	for key, d := range p.declared {
		seen[key] = true
		out = append(out, d.name)
	}
	for key, s := range p.locked {
		if !seen[key] {
			out = append(out, s.name)
		}
	}
	sort.Strings(out)
	return out
}

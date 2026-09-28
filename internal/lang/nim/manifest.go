package nim

import (
	"bytes"
	"encoding/json"
	"path"
	"regexp"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// dependency is one requirement of a .nimble file: `jester >= 0.5`, `pkg#abc123`,
// `https://github.com/x/y.git#head`, `gh:user/repo`, `pkg[feature] == 1.2.3`.
type dependency struct {
	text    string // as written
	name    string // the package name, or the repository URL
	url     string // for a URL requirement, the URL without its #ref
	version string // the version requirement: "", ">= 0.5", "== 1.2.3", "#head", "1.2.3"
	task    string // taskRequires' task
	line    int
}

// packageName is the node name of a requirement: a URL requirement is named by its
// repository like other git sources (github.com/x/y), a name by itself.
func (d dependency) packageName() string {
	if p, ok := d.file(); ok {
		return path.Base(p)
	}
	if d.url != "" {
		return lang.RepositoryName(d.url)
	}
	return d.name
}

// forges are nimble's forge aliases: gh:user/repo is https://github.com/user/repo.
var forges = map[string]string{
	"github": "github.com", "gh": "github.com", "gitlab": "gitlab.com", "gl": "gitlab.com",
	"sourcehut": "git.sr.ht", "srht": "git.sr.ht", "shart": "git.sr.ht", "codeberg": "codeberg.org",
	"cb": "codeberg.org", "cberg": "codeberg.org",
}

var features = regexp.MustCompile(`\[[^\]]*\]`)

// parseDependency reads a requirement as nimble's parseRequires does: a name and a
// version range after the first blank, else a name and a #ref, else a name;
// features in brackets are dropped and forge aliases expanded.
//
// Implements: REQ-NIM-005
func parseDependency(text string, line int, task string) (dependency, bool) {
	d := dependency{text: text, line: line, task: task}
	s := strings.TrimSpace(features.ReplaceAllString(text, ""))
	if s == "" {
		return d, false
	}
	if strings.HasPrefix(s, "file://") {
		d.name, d.url = s, s
		return d, true
	}
	switch {
	case strings.ContainsAny(s, " \t"):
		i := strings.IndexAny(s, " \t")
		d.name, d.version = s[:i], strings.TrimSpace(s[i:])
	case strings.Contains(s, "#"):
		i := strings.Index(s, "#")
		d.name, d.version = s[:i], s[i:]
	default:
		d.name = s
	}
	if at, ok := strings.CutSuffix(d.name, "@"); ok { // pkg@#head, the command line's form
		d.name = at
	}
	if kind, rest, ok := strings.Cut(d.name, ":"); ok && !strings.Contains(d.name, "://") && forges[strings.ToLower(kind)] != "" && strings.Count(rest, "/") == 1 {
		d.url = "https://" + forges[strings.ToLower(kind)] + "/" + rest
	} else if strings.Contains(d.name, "://") || strings.HasPrefix(d.name, "git@") {
		d.url = d.name
	}
	return d, d.name != ""
}

// file is the directory a file:// requirement names.
func (d dependency) file() (string, bool) {
	p, ok := strings.CutPrefix(d.url, "file://")
	if !ok || p == "" {
		return "", false
	}
	return path.Clean(p), true
}

// compiler reports whether a requirement names the compiler (`nim >= 2.0`),
// which is the language's runtime, not a package.
func (d dependency) compiler() bool { return d.url == "" && strings.EqualFold(d.name, "nim") }

// pinRule is nimble's pinning of a requirement without a lock: `== 1.2.3`,
// a bare `1.2.3` (an exact version in nimble) and a #<commit> pin; a #tag is
// shown, neither pinned nor floating (a tag can be moved); #head, a branch,
// a range (>=, ^=, ~=, &) and no version float.
//
// Implements: REQ-NIM-006
func pinRule(t *lang.Target, version string) {
	v := strings.TrimSpace(version)
	switch {
	case v == "" || v == "any" || v == "*":
		t.Floating = true
	case strings.HasPrefix(v, "#"):
		reference := strings.TrimPrefix(v, "#")
		switch {
		case commit(reference):
			t.Version, t.Pinned = reference, true
		case lang.Pinned(reference):
			t.Version = reference
		default:
			t.Version, t.Floating = v, true
		}
	case strings.HasPrefix(v, "=="):
		if x := strings.TrimSpace(strings.TrimPrefix(v, "==")); lang.Pinned(x) {
			t.Version, t.Pinned = x, true
		} else {
			t.Version, t.Floating = v, true
		}
	case lang.Pinned(v):
		t.Version, t.Pinned = v, true
	default:
		t.Version, t.Floating = v, true
	}
}

// commit reports whether a #ref is a commit: nimble takes abbreviated hashes,
// so six to 64 hexadecimal digits with at least one letter (an all-digit reference
// is more likely a version).
func commit(reference string) bool {
	if len(reference) < 6 || len(reference) > 64 {
		return false
	}
	letter := false
	for _, r := range reference {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
			letter = true
		default:
			return false
		}
	}
	return letter
}

// nimbleFile is what a .nimble file declares.
type nimbleFile struct {
	name            string // packageName, else the file's name
	version         string
	sourceDirectory string
	bins            []string
	binLine         int
	dependencies    []dependency
}

// readNimble reads a .nimble file's package settings and requirements; base is
// the file's name without .nimble.
//
// Implements: REQ-NIM-005
func readNimble(source []byte, base string) *nimbleFile {
	s := scanSource(source)
	n := &nimbleFile{name: base}
	first := func(key string) (string, int) {
		for k, a := range s.assigns {
			if strings.EqualFold(k, key) && len(a.values) > 0 {
				return a.values[0], a.line
			}
		}
		return "", 0
	}
	if v, _ := first("packageName"); v != "" {
		n.name = v
	}
	n.version, _ = first("version")
	n.sourceDirectory, _ = first("srcDir")
	for k, a := range s.assigns {
		if k == "bin" {
			n.bins, n.binLine = a.values, a.line
		}
	}
	for _, r := range s.requirements() {
		for _, part := range strings.Split(r.text, ",") {
			if d, ok := parseDependency(part, r.line, r.task); ok {
				n.dependencies = append(n.dependencies, d)
			}
		}
	}
	return n
}

// readRequiresFile reads the plain `requires` file nimble reads beside a
// .nimble: one requirement per line, `#` comments.
func readRequiresFile(source []byte) []dependency {
	var out []dependency
	for i, line := range strings.Split(string(source), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if d, ok := parseDependency(line, i+1, ""); ok {
			out = append(out, d)
		}
	}
	return out
}

// locked is a package of nimble.lock or atlas.lock.
type locked struct {
	name         string
	version      string // the package's version at the locked revision
	revision     string // the commit
	url          string
	dependencies []string
	line         int
}

// shown is the version a locked package is shown with: its version, else
// (a special version like #head, or none) its revision.
func (l *locked) shown() string {
	if l.version != "" && !strings.HasPrefix(l.version, "#") {
		return l.version
	}
	return l.revision
}

// readNimbleLock reads nimble.lock (JSON: version 1 or 2): its packages, and
// the packages each task locks.
//
// Implements: REQ-NIM-006
func readNimbleLock(source []byte) []*locked {
	var out []*locked
	type entry struct {
		Version      string   `json:"version"`
		VcsRevision  string   `json:"vcsRevision"`
		URL          string   `json:"url"`
		Dependencies []string `json:"dependencies"`
	}
	seen := map[string]bool{}
	add := func(name string, e entry, line int) {
		if seen[name] || name == "" {
			return
		}
		seen[name] = true
		out = append(out, &locked{name: name, version: e.Version, revision: e.VcsRevision, url: e.URL, dependencies: e.Dependencies, line: line})
	}
	walkObject(source, func(decoder *json.Decoder, key string, line func() int) {
		switch key {
		case "packages":
			eachMember(decoder, line, func(name string, raw json.RawMessage, line int) {
				var e entry
				if json.Unmarshal(raw, &e) == nil {
					add(name, e, line)
				}
			})
		case "tasks":
			eachMember(decoder, line, func(_ string, raw json.RawMessage, _ int) {
				var task map[string]entry
				if json.Unmarshal(raw, &task) != nil {
					return
				}
				for _, name := range sortedKeys(task) {
					add(name, task[name], lineOf(source, `"`+name+`"`))
				}
			})
		default:
			var skip json.RawMessage
			decoder.Decode(&skip)
		}
	})
	return out
}

// readAtlasLock reads atlas.lock (JSON): `items`, each with the url, commit and
// version Atlas pinned, keyed by the repository's name.
//
// Implements: REQ-NIM-006
func readAtlasLock(source []byte) []*locked {
	var out []*locked
	walkObject(source, func(decoder *json.Decoder, key string, line func() int) {
		if key != "items" {
			var skip json.RawMessage
			decoder.Decode(&skip)
			return
		}
		eachMember(decoder, line, func(name string, raw json.RawMessage, line int) {
			var e struct {
				URL     string `json:"url"`
				Commit  string `json:"commit"`
				Version string `json:"version"`
			}
			if json.Unmarshal(raw, &e) == nil && name != "" {
				out = append(out, &locked{name: name, version: e.Version, revision: e.Commit, url: e.URL, line: line})
			}
		})
	})
	return out
}

// walkObject calls function for each member of the top-level JSON object, with the
// decoder positioned at the member's value.
func walkObject(source []byte, function func(decoder *json.Decoder, key string, line func() int)) {
	decoder := json.NewDecoder(bytes.NewReader(source))
	lines := newLiner(source)
	line := func() int { return lines.at(int(decoder.InputOffset())) }
	if t, err := decoder.Token(); err != nil || t != json.Delim('{') {
		return
	}
	for decoder.More() {
		t, err := decoder.Token()
		if err != nil {
			return
		}
		key, _ := t.(string)
		function(decoder, key, line)
	}
}

// eachMember calls function for each member of the JSON object the decoder is at, in
// file order, with the line of its key.
func eachMember(decoder *json.Decoder, line func() int, function func(name string, raw json.RawMessage, line int)) {
	if t, err := decoder.Token(); err != nil || t != json.Delim('{') {
		return
	}
	for decoder.More() {
		t, err := decoder.Token()
		if err != nil {
			return
		}
		name, _ := t.(string)
		lineNumber := line()
		var raw json.RawMessage
		if decoder.Decode(&raw) != nil {
			return
		}
		function(name, raw, lineNumber)
	}
	decoder.Token()
}

// liner maps offsets to line numbers, for offsets that only grow.
type liner struct {
	source         []byte
	position, line int
}

func newLiner(source []byte) *liner { return &liner{source: source, line: 1} }

func (l *liner) at(off int) int {
	if off > len(l.source) {
		off = len(l.source)
	}
	if off > l.position {
		l.line += bytes.Count(l.source[l.position:off], []byte{'\n'})
		l.position = off
	}
	return l.line
}

func lineOf(source []byte, needle string) int {
	if i := bytes.Index(source, []byte(needle)); i >= 0 {
		return bytes.Count(source[:i], []byte{'\n'}) + 1
	}
	return 1
}

var configPath = regexp.MustCompile(`(?i)^-{0,2}(path|p)\s*[:=]\s*(.+)$`)

// readConfig reads the search paths of a nim.cfg (or nimble.paths): `--path:"x"`,
// `--path:x`, `path = "x"`, `-p:x`, in every `@if` branch.
//
// Implements: REQ-NIM-004
func readConfig(source []byte) []pathSwitch {
	var out []pathSwitch
	for i, line := range strings.Split(string(source), "\n") {
		line = strings.TrimSpace(stripConfigComment(line))
		m := configPath.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		v := strings.TrimSpace(m[2])
		v = strings.TrimPrefix(strings.TrimPrefix(v, "r\""), "R\"")
		v = strings.Trim(v, "\"")
		if v != "" {
			out = append(out, pathSwitch{value: v, line: i + 1})
		}
		if len(out) >= 1024 {
			break
		}
	}
	return out
}

func stripConfigComment(line string) string {
	quoted := false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"':
			quoted = !quoted
		case '#':
			if !quoted {
				return line[:i]
			}
		}
	}
	return line
}

// expandPath turns a configured search path into a directory relative to the
// scan root: $projectDir (and $projectPath, $configDir) and a relative path
// are the configuration's directory; $nim and $lib are the repository's own
// Nim and its library when it carries them (lib, "" when not); an absolute
// path is returned as is with absolute true; $home, ~ and other variables are not
// known ("").
func expandPath(directory, value, library string) (p string, absolute bool) {
	v := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	for _, prerelease := range []string{"$projectDir", "$projectdir", "$projectPath", "$projectpath", "$configDir", "$configdir", "$config"} {
		if rest, ok := strings.CutPrefix(v, prerelease); ok {
			v = "." + rest
			break
		}
	}
	if library != "" {
		for prerelease, to := range map[string]string{"$nim": path.Dir(library), "$lib": library} {
			if rest, ok := strings.CutPrefix(v, prerelease); ok && (rest == "" || rest[0] == '/') {
				return path.Join(to, rest), false
			}
		}
	}
	switch {
	case v == "":
		return "", false
	case strings.HasPrefix(v, "/"):
		return path.Clean(v), true
	case strings.ContainsAny(v, "$~%"):
		return "", false
	}
	return path.Join(directory, v), false
}

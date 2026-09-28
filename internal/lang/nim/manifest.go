package nim

import (
	"bytes"
	"encoding/json"
	"path"
	"regexp"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// dep is one requirement of a .nimble file: `jester >= 0.5`, `pkg#abc123`,
// `https://github.com/x/y.git#head`, `gh:user/repo`, `pkg[feature] == 1.2.3`.
type dep struct {
	text string // as written
	name string // the package name, or the repository URL
	url  string // for a URL requirement, the URL without its #ref
	ver  string // the version requirement: "", ">= 0.5", "== 1.2.3", "#head", "1.2.3"
	task string // taskRequires' task
	line int
}

// pkg is the node name of a requirement: a URL requirement is named by its
// repository like other git sources (github.com/x/y), a name by itself.
func (d dep) pkg() string {
	if p, ok := d.file(); ok {
		return path.Base(p)
	}
	if d.url != "" {
		return lang.RepoName(d.url)
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

// parseDep reads a requirement as nimble's parseRequires does: a name and a
// version range after the first blank, else a name and a #ref, else a name;
// features in brackets are dropped and forge aliases expanded.
//
// Implements: REQ-NIM-005
func parseDep(text string, line int, task string) (dep, bool) {
	d := dep{text: text, line: line, task: task}
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
		d.name, d.ver = s[:i], strings.TrimSpace(s[i:])
	case strings.Contains(s, "#"):
		i := strings.Index(s, "#")
		d.name, d.ver = s[:i], s[i:]
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
func (d dep) file() (string, bool) {
	p, ok := strings.CutPrefix(d.url, "file://")
	if !ok || p == "" {
		return "", false
	}
	return path.Clean(p), true
}

// compiler reports whether a requirement names the compiler (`nim >= 2.0`),
// which is the language's runtime, not a package.
func (d dep) compiler() bool { return d.url == "" && strings.EqualFold(d.name, "nim") }

// pinRule is nimble's pinning of a requirement without a lock: `== 1.2.3`,
// a bare `1.2.3` (an exact version in nimble) and a #<commit> pin; a #tag is
// shown, neither pinned nor floating (a tag can be moved); #head, a branch,
// a range (>=, ^=, ~=, &) and no version float.
//
// Implements: REQ-NIM-006
func pinRule(t *lang.Target, ver string) {
	v := strings.TrimSpace(ver)
	switch {
	case v == "" || v == "any" || v == "*":
		t.Floating = true
	case strings.HasPrefix(v, "#"):
		ref := strings.TrimPrefix(v, "#")
		switch {
		case commit(ref):
			t.Version, t.Pinned = ref, true
		case lang.Pinned(ref):
			t.Version = ref
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
// so six to 64 hexadecimal digits with at least one letter (an all-digit ref
// is more likely a version).
func commit(ref string) bool {
	if len(ref) < 6 || len(ref) > 64 {
		return false
	}
	letter := false
	for _, r := range ref {
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
	name    string // packageName, else the file's name
	version string
	srcDir  string
	bins    []string
	binLine int
	deps    []dep
}

// readNimble reads a .nimble file's package settings and requirements; base is
// the file's name without .nimble.
//
// Implements: REQ-NIM-005
func readNimble(src []byte, base string) *nimbleFile {
	s := scanSource(src)
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
	n.srcDir, _ = first("srcDir")
	for k, a := range s.assigns {
		if k == "bin" {
			n.bins, n.binLine = a.values, a.line
		}
	}
	for _, r := range s.requirements() {
		for _, part := range strings.Split(r.text, ",") {
			if d, ok := parseDep(part, r.line, r.task); ok {
				n.deps = append(n.deps, d)
			}
		}
	}
	return n
}

// readRequiresFile reads the plain `requires` file nimble reads beside a
// .nimble: one requirement per line, `#` comments.
func readRequiresFile(src []byte) []dep {
	var out []dep
	for i, ln := range strings.Split(string(src), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		if d, ok := parseDep(ln, i+1, ""); ok {
			out = append(out, d)
		}
	}
	return out
}

// locked is a package of nimble.lock or atlas.lock.
type locked struct {
	name     string
	version  string // the package's version at the locked revision
	revision string // the commit
	url      string
	deps     []string
	line     int
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
func readNimbleLock(src []byte) []*locked {
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
		out = append(out, &locked{name: name, version: e.Version, revision: e.VcsRevision, url: e.URL, deps: e.Dependencies, line: line})
	}
	walkObject(src, func(dec *json.Decoder, key string, line func() int) {
		switch key {
		case "packages":
			eachMember(dec, line, func(name string, raw json.RawMessage, ln int) {
				var e entry
				if json.Unmarshal(raw, &e) == nil {
					add(name, e, ln)
				}
			})
		case "tasks":
			eachMember(dec, line, func(_ string, raw json.RawMessage, _ int) {
				var task map[string]entry
				if json.Unmarshal(raw, &task) != nil {
					return
				}
				for _, name := range sortedKeys(task) {
					add(name, task[name], lineOf(src, `"`+name+`"`))
				}
			})
		default:
			var skip json.RawMessage
			dec.Decode(&skip)
		}
	})
	return out
}

// readAtlasLock reads atlas.lock (JSON): `items`, each with the url, commit and
// version Atlas pinned, keyed by the repository's name.
//
// Implements: REQ-NIM-006
func readAtlasLock(src []byte) []*locked {
	var out []*locked
	walkObject(src, func(dec *json.Decoder, key string, line func() int) {
		if key != "items" {
			var skip json.RawMessage
			dec.Decode(&skip)
			return
		}
		eachMember(dec, line, func(name string, raw json.RawMessage, ln int) {
			var e struct {
				URL     string `json:"url"`
				Commit  string `json:"commit"`
				Version string `json:"version"`
			}
			if json.Unmarshal(raw, &e) == nil && name != "" {
				out = append(out, &locked{name: name, version: e.Version, revision: e.Commit, url: e.URL, line: ln})
			}
		})
	})
	return out
}

// walkObject calls fn for each member of the top-level JSON object, with the
// decoder positioned at the member's value.
func walkObject(src []byte, fn func(dec *json.Decoder, key string, line func() int)) {
	dec := json.NewDecoder(bytes.NewReader(src))
	lines := newLiner(src)
	line := func() int { return lines.at(int(dec.InputOffset())) }
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return
	}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return
		}
		key, _ := t.(string)
		fn(dec, key, line)
	}
}

// eachMember calls fn for each member of the JSON object the decoder is at, in
// file order, with the line of its key.
func eachMember(dec *json.Decoder, line func() int, fn func(name string, raw json.RawMessage, line int)) {
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return
	}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return
		}
		name, _ := t.(string)
		ln := line()
		var raw json.RawMessage
		if dec.Decode(&raw) != nil {
			return
		}
		fn(name, raw, ln)
	}
	dec.Token()
}

// liner maps offsets to line numbers, for offsets that only grow.
type liner struct {
	src       []byte
	pos, line int
}

func newLiner(src []byte) *liner { return &liner{src: src, line: 1} }

func (l *liner) at(off int) int {
	if off > len(l.src) {
		off = len(l.src)
	}
	if off > l.pos {
		l.line += bytes.Count(l.src[l.pos:off], []byte{'\n'})
		l.pos = off
	}
	return l.line
}

func lineOf(src []byte, needle string) int {
	if i := bytes.Index(src, []byte(needle)); i >= 0 {
		return bytes.Count(src[:i], []byte{'\n'}) + 1
	}
	return 1
}

var cfgPath = regexp.MustCompile(`(?i)^-{0,2}(path|p)\s*[:=]\s*(.+)$`)

// readCfg reads the search paths of a nim.cfg (or nimble.paths): `--path:"x"`,
// `--path:x`, `path = "x"`, `-p:x`, in every `@if` branch.
//
// Implements: REQ-NIM-004
func readCfg(src []byte) []pathSwitch {
	var out []pathSwitch
	for i, ln := range strings.Split(string(src), "\n") {
		ln = strings.TrimSpace(stripCfgComment(ln))
		m := cfgPath.FindStringSubmatch(ln)
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

func stripCfgComment(ln string) string {
	quoted := false
	for i := 0; i < len(ln); i++ {
		switch ln[i] {
		case '"':
			quoted = !quoted
		case '#':
			if !quoted {
				return ln[:i]
			}
		}
	}
	return ln
}

// expandPath turns a configured search path into a directory relative to the
// scan root: $projectDir (and $projectPath, $configDir) and a relative path
// are the configuration's directory; $nim and $lib are the repository's own
// Nim and its library when it carries them (lib, "" when not); an absolute
// path is returned as is with abs true; $home, ~ and other variables are not
// known ("").
func expandPath(dir, value, lib string) (p string, abs bool) {
	v := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	for _, pre := range []string{"$projectDir", "$projectdir", "$projectPath", "$projectpath", "$configDir", "$configdir", "$config"} {
		if rest, ok := strings.CutPrefix(v, pre); ok {
			v = "." + rest
			break
		}
	}
	if lib != "" {
		for pre, to := range map[string]string{"$nim": path.Dir(lib), "$lib": lib} {
			if rest, ok := strings.CutPrefix(v, pre); ok && (rest == "" || rest[0] == '/') {
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
	return path.Join(dir, v), false
}

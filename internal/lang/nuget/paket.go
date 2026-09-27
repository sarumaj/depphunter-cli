package nuget

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// MainGroup is the group of a Paket file's lines before any `group` line.
const MainGroup = "Main"

// Dependency is one line of paket.dependencies that names something to install.
type Dependency struct {
	Group string
	Kind  string // nuget, clitool, github, gist, git, http
	// Name is the package id (nuget, clitool), owner/repo (github, gist) or URL
	// (git, http).
	Name string
	// Constraint is the version constraint of a nuget line ("~> 1.2", "= 1.2.3", ""),
	// or the ref of a remote one (a branch, tag, commit or tag range).
	Constraint string
	File       string // the file a github/gist/http line takes, if it names one
	Line       int
	Text       string // the line without its comment
}

// Source is a `source` line of paket.dependencies: a NuGet feed (or a directory).
type Source struct {
	Group, URL string
}

// ParseDependencies reads paket.dependencies: nuget, clitool, github, gist, git and
// http lines per group, and the sources.
//
// Implements: REQ-FSHARP-006
func ParseDependencies(src []byte) (deps []Dependency, sources []Source) {
	group := MainGroup
	for i, raw := range strings.Split(string(src), "\n") {
		line := stripComment(raw)
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		text := strings.TrimSpace(line)
		switch kw := strings.ToLower(f[0]); kw {
		case "group":
			if len(f) > 1 {
				group = f[1]
			}
		case "source":
			if len(f) > 1 {
				sources = append(sources, Source{Group: group, URL: f[1]})
			}
		case "nuget", "clitool":
			if len(f) < 2 {
				continue
			}
			deps = append(deps, Dependency{Group: group, Kind: "nuget", Name: f[1], Constraint: constraint(f[2:]), Line: i + 1, Text: text})
		case "github", "gist":
			if len(f) < 2 {
				continue
			}
			name, ref, _ := strings.Cut(f[1], ":")
			d := Dependency{Group: group, Kind: kw, Name: name, Constraint: ref, Line: i + 1, Text: text}
			if len(f) > 2 && !strings.Contains(f[2], ":") {
				d.File = f[2]
			}
			deps = append(deps, d)
		case "git":
			if len(f) < 2 {
				continue
			}
			deps = append(deps, Dependency{Group: group, Kind: kw, Name: f[1], Constraint: constraint(f[2:]), Line: i + 1, Text: text})
		case "http":
			if len(f) < 2 {
				continue
			}
			d := Dependency{Group: group, Kind: kw, Name: f[1], Line: i + 1, Text: text}
			if len(f) > 2 && !strings.Contains(f[2], ":") {
				d.File = f[2]
			}
			deps = append(deps, d)
		}
	}
	return deps, sources
}

// stripComment drops a `//` or `#` comment. A `//` right after a `:` is part of a
// URL (https://...), not a comment.
func stripComment(line string) string {
	line = strings.TrimPrefix(line, "\xef\xbb\xbf")
	if t := strings.TrimSpace(line); strings.HasPrefix(t, "#") || strings.HasPrefix(t, "//") {
		return ""
	}
	for i := 0; i+1 < len(line); i++ {
		if line[i] == '/' && line[i+1] == '/' && (i == 0 || line[i-1] != ':') {
			return line[:i]
		}
		if line[i] == '#' && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t') {
			return line[:i]
		}
	}
	return line
}

// constraint joins the version words of a line, up to the first option (a word with
// a colon: `redirects: force`, `framework: net6.0`). A leading @ or ! (resolution
// strategy) is not part of the version.
func constraint(words []string) string {
	var out []string
	for _, w := range words {
		if strings.Contains(w, ":") || strings.HasPrefix(w, "\"") {
			break
		}
		out = append(out, w)
	}
	s := strings.Join(out, " ")
	return strings.TrimLeft(s, "@!")
}

// PaketVersion applies Paket's pin rule to a nuget constraint: `= 1.2.3`,
// `== 1.2.3` and a bare `1.2.3` name one version and pin it (shown without the
// operator); `~>`, `>=`, `<` and ranges float and are shown as written; a
// pre-release word (prerelease, alpha, beta, rc) is not part of the version.
//
// Implements: REQ-FSHARP-008
func PaketVersion(c string) (version string, pinned bool) {
	f := strings.Fields(c)
	for len(f) > 0 {
		switch strings.ToLower(f[len(f)-1]) {
		case "prerelease", "alpha", "beta", "rc", "preview":
			f = f[:len(f)-1]
			continue
		}
		break
	}
	switch {
	case len(f) == 0:
		return "", false
	case len(f) == 1 && lang.Pinned(f[0]) && versionStart(f[0]):
		return f[0], true
	case len(f) == 2 && (f[0] == "=" || f[0] == "==") && versionStart(f[1]):
		return f[1], lang.Pinned(f[1])
	}
	return strings.Join(f, " "), false
}

func versionStart(v string) bool { return v != "" && v[0] >= '0' && v[0] <= '9' }

// LockDep is a dependency line under a locked package: `FSharp.Core (>= 4.3.2)`.
type LockDep struct{ Name, Constraint string }

// Locked is one resolved entry of paket.lock.
type Locked struct {
	Group string
	Kind  string // nuget, github, gist, git, http
	// Remote is the feed of a nuget entry, owner/repo of a github or gist entry, and
	// the URL of a git or http entry.
	Remote  string
	Name    string // package id (nuget), file path (github, gist, http), "" (git)
	Version string // resolved version (nuget) or commit (github, git)
	Line    int
	Deps    []LockDep
	indent  int
}

// ParseLock reads paket.lock: per group (GROUP lines) its NUGET, GITHUB, GIST, GIT
// and HTTP sections, each `remote:` and the entries under it with their dependency
// lines.
//
// Implements: REQ-FSHARP-007
func ParseLock(src []byte) []Locked {
	var out []Locked
	group, section, remote := MainGroup, "", ""
	for i, raw := range strings.Split(string(src), "\n") {
		raw = strings.TrimRight(strings.TrimPrefix(raw, "\xef\xbb\xbf"), "\r")
		text := strings.TrimSpace(raw)
		if text == "" {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
		if indent == 0 {
			switch {
			case strings.HasPrefix(text, "GROUP "):
				group, section, remote = strings.TrimSpace(text[6:]), "", ""
			case text == "NUGET" || text == "GITHUB" || text == "GIST" || text == "GIT" || text == "HTTP":
				section, remote = strings.ToLower(text), ""
			default:
				section = "" // STORAGE:, RESTRICTION:, ...
			}
			continue
		}
		if section == "" {
			continue
		}
		if r, ok := strings.CutPrefix(text, "remote:"); ok {
			remote = strings.TrimSpace(r)
			continue
		}
		if remote == "" || strings.HasSuffix(text, ":") { // specs: (Paket's first lock format)
			continue
		}
		name, version := lockEntry(text)
		// Entries sit one level below remote:, their dependencies one more. The
		// levels are two spaces apart, but GIT sections are written with other
		// indentation, so the level is judged against the entry above.
		if n := len(out); n > 0 && out[n-1].Group == group && out[n-1].Kind == section && out[n-1].Remote == remote && indent > out[n-1].indentHint() && name != "" {
			if strings.Contains(text, ":") && !strings.Contains(text, "(") && section != "nuget" {
				continue // build: "..." options of a git entry
			}
			out[n-1].Deps = append(out[n-1].Deps, LockDep{Name: name, Constraint: version})
			continue
		}
		if name == "" && version == "" {
			continue
		}
		out = append(out, Locked{Group: group, Kind: section, Remote: remote, Name: name, Version: version, Line: i + 1, indent: indent})
	}
	return out
}

func (l Locked) indentHint() int { return l.indent }

// lockEntry splits `Name (version) - settings` (or `(commit)` alone, or a file path
// with its commit) into name and version.
func lockEntry(text string) (name, version string) {
	if i := strings.Index(text, " - "); i >= 0 {
		text = text[:i]
	}
	text = strings.TrimSpace(text)
	if open := strings.Index(text, "("); open >= 0 {
		if end := strings.LastIndex(text, ")"); end > open {
			version = strings.TrimSpace(text[open+1 : end])
		}
		text = text[:open]
	}
	return strings.TrimSpace(text), version
}

// Reference is one line of paket.references: a package the project uses, or a
// remote file (`File: Globbing.fs`).
type Reference struct {
	Group string
	Name  string // package id, or the file name for File: lines
	File  bool
	Line  int
}

// ParseReferences reads a project's paket.references.
//
// Implements: REQ-FSHARP-006
func ParseReferences(src []byte) []Reference {
	var out []Reference
	group := MainGroup
	for i, raw := range strings.Split(string(src), "\n") {
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		if rest, ok := cutWord(line, "group"); ok {
			if f := strings.Fields(rest); len(f) > 0 {
				group = f[0]
			}
			continue
		}
		if rest, ok := strings.CutPrefix(line, "File:"); ok {
			if f := strings.Fields(rest); len(f) > 0 {
				out = append(out, Reference{Group: group, Name: f[0], File: true, Line: i + 1})
			}
			continue
		}
		if _, ok := cutWord(line, "exclude"); ok {
			continue // settings of the package above
		}
		if _, ok := cutWord(line, "alias"); ok {
			continue
		}
		if rest, ok := cutWord(line, "nuget"); ok {
			line = rest // tolerated: some write "nuget X" here too
		}
		if f := strings.Fields(line); len(f) > 0 && !strings.Contains(f[0], ":") {
			out = append(out, Reference{Group: group, Name: f[0], Line: i + 1})
		}
	}
	return out
}

func cutWord(line, word string) (string, bool) {
	if len(line) > len(word) && strings.EqualFold(line[:len(word)], word) && (line[len(word)] == ' ' || line[len(word)] == '\t') {
		return strings.TrimSpace(line[len(word):]), true
	}
	return "", false
}

// RemoteName names what a github, gist, git or http line or lock entry fetches:
// github.com/owner/repo, gist.github.com/owner/id, or the URL without scheme.
//
// Implements: REQ-FSHARP-007
func RemoteName(kind, name string) string {
	switch kind {
	case "github":
		return "github.com/" + strings.Trim(name, "/")
	case "gist":
		return "gist.github.com/" + strings.Trim(name, "/")
	}
	if strings.HasPrefix(name, "file:") || !strings.Contains(name, "/") && !strings.Contains(name, ":") {
		return ""
	}
	return lang.RepoName(name)
}

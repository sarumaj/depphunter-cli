package commonlisp

import (
	"bytes"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// qlEntry is a line of a qlfile: <source> <project name> [args].
type qlEntry struct {
	source  string // ql, ultralisp, ql-dist, git, github, http, local
	name    string // the project name, lower case
	version string // ql's dist version (2023-10-21, :latest, :upstream)
	url     string // git's URL, github's user/repository as a URL, http's URL, local's path
	ref     string
	branch  string
	tag     string
	md5     string
	line    int
}

// qlfile is what a qlfile declares: its projects and, from `ql :all <date>`
// or a `dist <url> <version>` of the Quicklisp dist, the dist version every
// other Quicklisp project comes from.
type qlfile struct {
	entries []qlEntry
	dist    string
}

// quicklispDist reports whether a dist URL is the Quicklisp dist itself.
func quicklispDist(u string) bool {
	return strings.Contains(u, "beta.quicklisp.org/dist/quicklisp")
}

// readQlfile reads a qlfile; # starts a comment.
//
// Implements: REQ-COMMONLISP-006
func readQlfile(src []byte) qlfile {
	var q qlfile
	for i, line := range strings.Split(string(src), "\n") {
		if j := strings.IndexByte(line, '#'); j >= 0 {
			line = line[:j]
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		e := qlEntry{source: strings.ToLower(f[0]), line: i + 1}
		args := f[1:]
		switch e.source {
		case "ql", "ultralisp":
			if strings.EqualFold(args[0], ":all") {
				if len(args) > 1 && dated(args[1]) && e.source == "ql" {
					q.dist = args[1]
				}
				continue
			}
			e.name = args[0]
			if len(args) > 1 {
				e.version = args[1]
			}
		case "ql-dist":
			if len(args) < 2 {
				continue
			}
			e.name = args[1]
			if len(args) > 2 {
				e.version = args[2]
			}
		case "git", "http", "local":
			if len(args) < 2 {
				continue
			}
			e.name, e.url = args[0], args[1]
			args = args[2:]
			if e.source == "http" && len(args) > 0 && !strings.HasPrefix(args[0], ":") {
				e.md5 = args[0]
			}
		case "github":
			// github <user/repository>, or the older github <name> <user/repository>
			repos := args[0]
			if !strings.Contains(repos, "/") && len(args) > 1 {
				e.name, repos, args = args[0], args[1], args[2:]
			} else {
				args = args[1:]
			}
			e.url = "https://github.com/" + strings.TrimSuffix(repos, ".git")
			if e.name == "" {
				e.name = repos[strings.LastIndexByte(repos, '/')+1:]
			}
		case "dist":
			// dist <url> [version], or dist <name> <url> [version]
			u, rest := args[0], args[1:]
			if !strings.Contains(u, "/") && len(rest) > 0 {
				u, rest = rest[0], rest[1:]
			}
			if quicklispDist(u) && len(rest) > 0 && dated(rest[0]) {
				q.dist = rest[0]
			}
			continue
		default:
			continue // asdf <version>, unknown sources
		}
		for k := 0; k+1 < len(args); k++ {
			switch strings.ToLower(args[k]) {
			case ":ref":
				e.ref = args[k+1]
			case ":branch":
				e.branch = args[k+1]
			case ":tag":
				e.tag = args[k+1]
			}
		}
		e.name = strings.ToLower(e.name)
		if e.name != "" {
			q.entries = append(q.entries, e)
		}
	}
	return q
}

// dated reports whether v is a dist version: a date, 2023-10-21.
func dated(v string) bool {
	if len(v) != 10 || v[4] != '-' || v[7] != '-' {
		return false
	}
	for i, c := range v {
		if i != 4 && i != 7 && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// lockEntry is a project a qlfile.lock records.
type lockEntry struct {
	name    string
	class   string // qlot/source/ql:source-ql, ...git:source-git, ...dist:source-dist
	version string // ql-2023-10-21, git-<commit>, a dist's 2023-10-21
	commit  string
	url     string // :remote-url, or a github source's repository
	branch  string // what the qlfile asked for (:branch, :tag, :ref in :initargs)
	dist    string // a dist entry's :distribution
	line    int
}

// qlock is a qlfile.lock: its projects and the dist versions it fixed, in
// the lock's order (the last dist has priority for projects not listed).
type qlock struct {
	entries []lockEntry
	dists   []lockEntry
}

// readLock reads a qlfile.lock: ("name" . (:class ... :initargs (...)
// :version "..." ...)) forms.
//
// Implements: REQ-COMMONLISP-006
func readLock(src []byte) qlock {
	var l qlock
	for _, f := range Read(src) {
		if f.Kind != List || len(f.Kids) < 2 || f.Kids[0].Kind != String {
			continue
		}
		body := f.Kids[1:]
		if len(body) == 2 && body[0].Kind == Other && body[0].Text == "." && body[1].Kind == List {
			body = body[1].Kids
		}
		opts := plist(body)
		e := lockEntry{name: strings.ToLower(f.Kids[0].Text), line: f.Line}
		e.class = strings.ToLower(symbolText(opts["class"]))
		e.version = stringOf(opts["version"])
		e.url = stringOf(opts["remote-url"])
		ref := stringOf(opts["ref"])
		if init := opts["initargs"]; init != nil && init.Kind == List {
			io := plist(init.Kids)
			e.dist = stringOf(io["distribution"])
			if e.url == "" {
				e.url = stringOf(io["remote-url"])
			}
			if r := stringOf(io["repos"]); e.url == "" && r != "" {
				e.url = "https://github.com/" + r
			}
			for _, k := range []string{"branch", "tag", "ref"} {
				if v := stringOf(io[k]); v != "" && e.branch == "" {
					e.branch = v
				}
			}
			if v := io["%version"]; e.branch == "" && v != nil {
				if s := stringOf(v); s != "" {
					e.branch = s
				} else if v.Kind == Keyword {
					e.branch = ":" + strings.ToLower(v.Text)
				}
			}
		}
		switch {
		case lang.Commit(ref):
			e.commit = ref
		case strings.Contains(e.version, "-"):
			if c := e.version[strings.LastIndexByte(e.version, '-')+1:]; lang.Commit(c) {
				e.commit = c
			}
		}
		if strings.Contains(e.class, "source-dist") {
			l.dists = append(l.dists, e)
			continue
		}
		l.entries = append(l.entries, e)
	}
	return l
}

func stringOf(n *Node) string {
	if n != nil && n.Kind == String {
		return n.Text
	}
	return ""
}

func symbolText(n *Node) string {
	if n == nil || n.Kind != Symbol && n.Kind != Keyword {
		return ""
	}
	if n.Pkg != "" {
		return n.Pkg + ":" + n.Text
	}
	return n.Text
}

// ociclEntry is a row of ocicl.csv: a system, the OCI image ocicl fetched
// it from (by digest) and the release directory holding its .asd.
type ociclEntry struct {
	system  string
	image   string // ghcr.io/ocicl/alexandria@sha256:...
	project string // the image's name: alexandria
	digest  string
	release string // alexandria-20240503-8514d8e
	line    int
}

// readOcicl reads ocicl.csv: `system, image@sha256:digest, release/x.asd`.
//
// Implements: REQ-COMMONLISP-006
func readOcicl(src []byte) []ociclEntry {
	var out []ociclEntry
	for i, line := range bytes.Split(src, []byte("\n")) {
		cols := strings.Split(string(line), ",")
		if len(cols) < 2 {
			continue
		}
		e := ociclEntry{system: strings.ToLower(strings.TrimSpace(cols[0])), image: strings.TrimSpace(cols[1]), line: i + 1}
		if e.system == "" || e.image == "" || strings.HasPrefix(e.system, "#") {
			continue
		}
		ref, digest, _ := strings.Cut(e.image, "@")
		e.digest = digest
		if j := strings.LastIndexByte(ref, ':'); j > strings.LastIndexByte(ref, '/') {
			ref = ref[:j] // a tag
		}
		e.project = strings.ToLower(ref[strings.LastIndexByte(ref, '/')+1:])
		if len(cols) > 2 {
			rel := strings.TrimSpace(cols[2])
			if j := strings.IndexByte(rel, '/'); j > 0 {
				e.release = rel[:j]
			}
		}
		out = append(out, e)
	}
	return out
}

// release is the version an ocicl release directory names: 20240503-8514d8e
// for alexandria-20240503-8514d8e.
func (e ociclEntry) version() string {
	if v, ok := strings.CutPrefix(e.release, e.project+"-"); ok {
		return v
	}
	if v, ok := strings.CutPrefix(e.release, e.system+"-"); ok {
		return v
	}
	return e.release
}

// extractQlfile makes every project of a qlfile an import.
func extractQlfile(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	seen := map[string]bool{}
	for _, e := range readQlfile(src).entries {
		spec := e.source + " " + e.name
		if !seen[spec] {
			seen[spec] = true
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: e.name, Name: kindQlfile, Line: e.line})
		}
	}
	return ex
}

// extractLock makes every project a qlfile.lock records an import.
func extractLock(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	seen := map[string]bool{}
	for _, e := range readLock(src).entries {
		if !seen[e.name] {
			seen[e.name] = true
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: "lock " + e.name, Module: e.name, Name: kindLock, Line: e.line})
		}
	}
	return ex
}

// extractOcicl makes every system of ocicl.csv an import.
func extractOcicl(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	seen := map[string]bool{}
	for _, e := range readOcicl(src) {
		if !seen[e.system] {
			seen[e.system] = true
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: "ocicl " + e.system, Module: e.system, Name: kindOcicl, Line: e.line})
		}
	}
	return ex
}

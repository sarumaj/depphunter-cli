package racket

import (
	"path"
	"strings"
)

// dependency is one entry of an info.rkt's deps or build-deps.
type dependency struct {
	source    string // as written: a package name or a package source
	name      string // the package name raco derives from the source
	url       string // a git or archive source's URL without its #ref
	reference string // a git source's #ref (a commit, branch or tag)
	local     string // a directory source ("../x", "file:///...")
	version   string // #:version, a minimum
	checksum  string // #:checksum
	build     bool   // listed in build-deps
	line      int
}

// info is what an info.rkt says about its package.
type info struct {
	collection   string // the collection's name, "multi", "use-pkg-name" or "" (not defined)
	defined      bool   // collection is defined
	isPackage    bool   // it is a package's info.rkt: it defines collection, deps, build-deps or pkg-desc
	version      string
	dependencies []dependency
}

// readInfo reads an info.rkt (#lang info or #lang setup/infotab): its
// top-level defines, never evaluated. A value built by code (append, if)
// gives what its literal parts say.
//
// Implements: REQ-RACKET-006
func readInfo(source []byte) info {
	var in info
	for _, f := range Read(source, false).Forms {
		if f.Head() != "define" || len(f.Kids) < 3 || f.Kids[1].Kind != Symbol {
			continue
		}
		v := Unquote(f.Kids[2])
		switch name := f.Kids[1].Text; name {
		case "collection":
			in.defined, in.isPackage = true, true
			switch v.Kind {
			case String:
				in.collection = v.Text
			case Symbol:
				in.collection = v.Text
			}
		case "version":
			if v.Kind == String {
				in.version = v.Text
			}
		case "pkg-desc":
			in.isPackage = true
		case "deps", "build-deps":
			in.isPackage = true
			for _, e := range elements(v) {
				if d, ok := readDependency(e); ok {
					d.build = name == "build-deps"
					in.dependencies = append(in.dependencies, d)
				}
			}
		}
	}
	return in
}

// elements lists the elements of a quoted list or a (list ...) call.
func elements(v *Node) []*Node {
	if v == nil || v.Kind != List {
		return nil
	}
	switch v.Head() {
	case "list", "list*":
		return v.Kids[1:]
	case "append":
		var out []*Node
		for _, k := range v.Kids[1:] {
			out = append(out, elements(Unquote(k))...)
		}
		return out
	}
	return v.Kids
}

// readDependency reads a deps element: "source" or ("source" #:version "1.2" ...).
func readDependency(e *Node) (dependency, bool) {
	e = Unquote(e)
	var d dependency
	switch {
	case e.Kind == String:
		d.source = e.Text
	case e.Kind == List:
		kids := elements(e)
		if len(kids) == 0 || Unquote(kids[0]).Kind != String {
			return d, false
		}
		d.source = Unquote(kids[0]).Text
		for i := 1; i+1 < len(kids); i++ {
			k, v := Unquote(kids[i]), Unquote(kids[i+1])
			if k.Kind != Keyword || v.Kind != String {
				continue
			}
			switch k.Text {
			case "version":
				d.version = v.Text
			case "checksum":
				d.checksum = v.Text
			}
		}
	default:
		return d, false
	}
	d.line = e.Line
	d.source = strings.TrimSpace(d.source)
	if d.source == "" {
		return d, false
	}
	d.name, d.url, d.reference, d.local = parseSource(d.source)
	return d, d.name != ""
}

// parseSource derives what raco does from a package source: its name, and a
// URL and #ref for a remote source or a path for a directory. A git source
// is named by the last element of its path (or of its ?path=), without
// .git; an archive by its file name without the archive suffix.
//
// Implements: REQ-RACKET-006
func parseSource(source string) (name, url, reference, local string) {
	s := source
	switch {
	case strings.HasPrefix(s, "file://"):
		local = strings.TrimPrefix(s, "file://")
		return strings.TrimSuffix(path.Base(strings.TrimRight(local, "/")), ".git"), "", "", local
	case strings.HasPrefix(s, "/"), strings.HasPrefix(s, "./"), strings.HasPrefix(s, "../"), s == ".", s == "..":
		return path.Base(strings.TrimRight(s, "/")), "", "", s
	case !strings.Contains(s, "://") && !strings.HasPrefix(s, "git@"):
		if strings.ContainsAny(s, "/\\ ") {
			return "", "", "", "" // not a package name
		}
		return s, "", "", ""
	}
	if u, r, ok := strings.Cut(s, "#"); ok {
		s, reference = u, r
	}
	subpath := ""
	if u, q, ok := strings.Cut(s, "?"); ok {
		s = u
		for _, keyValue := range strings.Split(q, "&") {
			if p, ok := strings.CutPrefix(keyValue, "path="); ok {
				subpath = strings.Trim(p, "/")
			}
		}
	}
	url = s
	if rest, ok := strings.CutPrefix(s, "github://"); ok {
		// github://github.com/<user>/<repo>/<branch>[/<path>], raco's old form
		parts := strings.Split(strings.Trim(rest, "/"), "/")
		url = "https://github.com/"
		if len(parts) >= 3 {
			url += parts[1] + "/" + parts[2]
			name = parts[2]
		}
		if len(parts) >= 4 && reference == "" {
			reference = parts[3]
		}
		if len(parts) >= 5 {
			subpath = strings.Join(parts[4:], "/")
		}
	} else {
		name = path.Base(strings.TrimRight(s, "/"))
	}
	if subpath != "" {
		name = path.Base(subpath)
	}
	for _, suffix := range []string{".git", ".zip", ".tar.gz", ".tgz", ".plt"} {
		name = strings.TrimSuffix(name, suffix)
	}
	url = strings.TrimPrefix(url, "git+")
	return name, url, reference, ""
}

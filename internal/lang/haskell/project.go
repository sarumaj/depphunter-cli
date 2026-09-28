package haskell

import (
	"path"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// maxProjectFiles bounds the project files one cabal.project brings in through
// import: (cabal refuses cycles; a long chain is not a project).
const maxProjectFiles = 32

// projectFiles reads a cabal.project and, depth first, the local project files
// its import: fields name, each once: the file itself first. An import is
// relative to the importing file's directory; a URL (cabal fetches it) and a
// path outside the repository are not read.
//
// Implements: REQ-HASKELL-007
func projectFiles(file string, read func(string) ([]byte, bool)) []*cabalProject {
	var out []*cabalProject
	seen := map[string]bool{}
	var visit func(file string)
	visit = func(file string) {
		if seen[file] || len(seen) >= maxProjectFiles {
			return
		}
		seen[file] = true
		src, ok := read(file)
		if !ok {
			return
		}
		cp := readCabalProject(src)
		out = append(out, cp)
		for _, im := range cp.imports {
			if p, ok := includePath(file, im.text); ok {
				visit(p)
			}
		}
	}
	visit(file)
	return out
}

// includePath is the repository path of a project file an import: in file
// names, unless it is a URL or lies outside the repository.
func includePath(file, name string) (string, bool) {
	if strings.Contains(name, "://") || path.IsAbs(name) {
		return "", false
	}
	p := path.Join(path.Dir(file), name)
	return p, p != ".." && !strings.HasPrefix(p, "../")
}

// include resolves an import: of cabal.project to the project file it names.
func (r *resolver) include(file, name string) lang.Target {
	if p, ok := includePath(file, name); ok && r.files[p] {
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

// globFields splits a packages: line like fields, but keeps a glob's `{a,b}`
// alternatives in one entry.
func globFields(s string) []string {
	var out []string
	depth, from := 0, -1
	for i := 0; i <= len(s); i++ {
		sep := i == len(s)
		if !sep {
			switch s[i] {
			case '{':
				depth++
			case '}':
				depth = max(depth-1, 0)
			case ',':
				sep = depth == 0
			case ' ', '\t', '"':
				sep = true
			}
		}
		switch {
		case sep && from >= 0:
			out = append(out, s[from:i])
			from = -1
		case !sep && from < 0:
			from = i
		}
	}
	return out
}

// glob reports whether a packages: entry is a cabal file glob.
func glob(entry string) bool { return strings.ContainsAny(entry, "*?[{") }

// memberDirs are the package directories a packages: entry of the project in
// dir names: the entry's directory, or those of the repository's packages whose
// directory or .cabal file a glob matches.
func (r *resolver) memberDirs(dir, entry string) []string {
	if !glob(entry) {
		return []string{path.Join(dir, strings.TrimSuffix(entry, "/"))}
	}
	var out []string
	for _, p := range r.globbed(dir, entry) {
		out = append(out, p.dir)
	}
	return out
}

// globbed are the repository's packages a packages: glob, relative to dir,
// matches, in path order: `*`, `?` and `[...]` within one path segment and
// `{a,b}` alternatives, as cabal reads them, against a package's directory
// (`libs/*/`) or its .cabal file (`*/*.cabal`).
func (r *resolver) globbed(dir, entry string) []*pkgInfo {
	alts := braces(path.Join(dir, strings.TrimPrefix(entry, "./")))
	var out []*pkgInfo
	for _, p := range r.pkgs {
		for _, alt := range alts {
			if m, _ := path.Match(alt, p.dir); m && p.dir != "." {
				out = append(out, p)
				break
			}
			if m, _ := path.Match(alt, p.file); m {
				out = append(out, p)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].file < out[j].file })
	return out
}

// maxAlternatives caps what `{a,b}` groups expand to.
const maxAlternatives = 64

// braces expands a glob's `{a,b}` groups into plain globs.
func braces(g string) []string {
	i := strings.IndexByte(g, '{')
	if i < 0 {
		return []string{g}
	}
	j := strings.IndexByte(g[i:], '}')
	if j < 0 {
		return []string{g}
	}
	j += i
	rests := braces(g[j+1:])
	var out []string
	for _, alt := range strings.Split(g[i+1:j], ",") {
		for _, rest := range rests {
			if len(out) == maxAlternatives {
				return out
			}
			out = append(out, g[:i]+alt+rest)
		}
	}
	return out
}

// Expand turns a packages: glob of cabal.project into one import per package
// of the repository it matches.
//
// Implements: REQ-HASKELL-005
func (r *resolver) Expand(file string, imp lang.RawImport) ([]lang.Import, bool) {
	if imp.Name != kindMember || !glob(imp.Module) || strings.Contains(imp.Module, "://") {
		return nil, false
	}
	var out []lang.Import
	for _, p := range r.globbed(path.Dir(file), imp.Module) {
		out = append(out, lang.Import{Spec: imp.Spec + " (" + p.file + ")", Line: imp.Line, Target: lang.Target{Local: p.file}})
	}
	return out, len(out) > 0
}

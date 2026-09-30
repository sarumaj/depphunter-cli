package nim

import (
	"encoding/json"
	"path"
	"path/filepath"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// developFile is nimble's nimble.develop: the packages a project builds from
// local directories instead of the versions it requires (`nimble develop`),
// and other develop files whose packages it takes too. Paths are relative to
// the file's directory.
type developFile struct {
	Includes     []string `json:"includes"`
	Dependencies []string `json:"dependencies"`
}

// maxIncludes bounds how deep develop files include each other.
const maxIncludes = 8

// readDevelop reads the nimble.develop of a project's directory and the develop
// files it includes. A dependency path that is a package directory of the
// repository maps that package's folded name to its project; the first file to
// name a package wins, as in nimble. A path outside the repository names
// nothing (its package keeps resolving as required).
//
// Implements: REQ-NIM-005
func (r *resolver) readDevelop(directory string, read func(string) []byte) map[string]*project {
	out := map[string]*project{}
	seen := map[string]bool{}
	var visit func(file string, depth int)
	visit = func(file string, depth int) {
		if file == "" || seen[file] || depth > maxIncludes {
			return
		}
		seen[file] = true
		var df developFile
		if json.Unmarshal(read(file), &df) != nil {
			return
		}
		base := path.Dir(file)
		for _, d := range df.Dependencies {
			if q := r.projects[r.inRepository(base, d)]; q != nil {
				if key := fold(q.nimble.name); key != "" && out[key] == nil {
					out[key] = q
				}
			}
		}
		for _, include := range df.Includes {
			visit(r.inRepository(base, include), depth+1)
		}
	}
	visit(path.Join(directory, "nimble.develop"), 0)
	return out
}

// inRepository is a path a develop file writes, relative to its directory base or
// absolute, as a cleaned path of the repository, or "" when it lies outside.
func (r *resolver) inRepository(base, p string) string {
	if p = strings.TrimSpace(p); p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		root, err := filepath.Abs(r.root)
		if r.root == "" || err != nil {
			return ""
		}
		relative, err := filepath.Rel(root, p)
		if err != nil {
			return ""
		}
		p, base = relative, "."
	}
	p = path.Join(base, filepath.ToSlash(p))
	if !lang.Inside(p) {
		return ""
	}
	return p
}

// developed is the package of the repository a project develops in place of
// the requirement named name (a package name or a repository), or nil.
func (p *project) developed(name string) *project {
	if q := p.develop[fold(name)]; q != nil {
		return q
	}
	if strings.Contains(name, "/") {
		return p.develop[fold(repositoryBase(name))]
	}
	return nil
}

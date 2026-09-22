package markdown

import (
	"net/url"
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type resolver struct {
	files map[string]bool
	dirs  map[string]bool
}

func newResolver(all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}}
	for _, f := range all {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); d != "." && d != "/"; d = path.Dir(d) {
			r.dirs[d] = true
		}
	}
	return r
}

// Resolve points a link at what it names in this repository. Anything else - a URL, a
// mail address, a fragment of this same document - is not a dependency the map can
// draw, and is left to the link check to judge.
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	p, ok := Target(file, imp.Module)
	if !ok {
		return lang.Target{}
	}
	if r.files[p] {
		return lang.Target{Local: p}
	}
	if r.dirs[p] {
		return lang.Target{Local: p}
	}
	// It may be there and simply not scanned - ignored, excluded, generated at build
	// time - which is not a broken link and not a node either.
	return lang.Target{}
}

// Target is the repository path a link names, and whether it names one at all.
//
// It is here rather than in the resolver because the link check asks the same
// question: a link that points outside the repository, or at something other than a
// path, is neither an edge on the map nor a defect in the document.
func Target(from, dest string) (string, bool) {
	dest, _, _ = strings.Cut(dest, "#")
	dest, _, _ = strings.Cut(dest, "?")
	if dest == "" || External(dest) || scheme(dest) != "" {
		return "", false
	}
	// A destination is a URL, so its spaces and brackets arrive escaped.
	if unescaped, err := url.PathUnescape(dest); err == nil {
		dest = unescaped
	}
	if strings.HasPrefix(dest, "/") {
		// Rooted, which means the repository root to one renderer and the host root
		// to another. Read as the repository's, and silently where it is not there:
		// guessing wrong in the other direction would report every site link broken.
		dest = strings.TrimPrefix(dest, "/")
	} else {
		dest = path.Join(path.Dir(from), dest)
	}
	p := path.Clean(dest)
	if p == "." || p == "/" || strings.HasPrefix(p, "..") {
		return "", false
	}
	return p, true
}

// External reports whether a link leaves for the web, which is the only kind that
// cannot be checked without asking somebody else.
func External(dest string) bool {
	s := scheme(dest)
	return s == "http" || s == "https"
}

// scheme is the URL scheme a destination names, or empty for a path. A Windows drive
// letter is not a scheme, and neither is the colon in a file name.
func scheme(dest string) string {
	i := strings.IndexByte(dest, ':')
	if i <= 1 || strings.ContainsAny(dest[:i], "/?#") {
		return ""
	}
	return strings.ToLower(dest[:i])
}

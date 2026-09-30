package markdown

import (
	"net/url"
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type resolver struct {
	lang.Layout
}

func newResolver(all []*scan.File) *resolver {
	return &resolver{Layout: lang.LayoutOf(all)}
}

// Resolve points a link at what it names in this repository. Anything else - a URL, a
// mail address, a fragment of this same document - is not a dependency the map can
// draw, and is left to the link check to judge.
//
// Implements: REQ-MD-001, REQ-MD-005
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	p, ok := Target(file, rawImport.Module)
	if !ok {
		return lang.Target{}
	}
	if r.Files[p] {
		return lang.Target{Local: p}
	}
	if r.Directories[p] {
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
//
// Implements: REQ-MD-001
func Target(from, destination string) (string, bool) {
	destination, _, _ = strings.Cut(destination, "#")
	destination, _, _ = strings.Cut(destination, "?")
	if destination == "" || External(destination) || scheme(destination) != "" {
		return "", false
	}
	// A destination is a URL, so its spaces and brackets arrive escaped.
	if unescaped, err := url.PathUnescape(destination); err == nil {
		destination = unescaped
	}
	if strings.HasPrefix(destination, "/") {
		// Rooted, which means the repository root to one renderer and the host root
		// to another. Read as the repository's, and silently where it is not there:
		// guessing wrong in the other direction would report every site link broken.
		destination = strings.TrimPrefix(destination, "/")
	} else {
		destination = path.Join(path.Dir(from), destination)
	}
	p := path.Clean(destination)
	if p == "." || p == "/" || strings.HasPrefix(p, "..") {
		return "", false
	}
	return p, true
}

// External reports whether a link leaves for the web, which is the only kind that
// cannot be checked without asking somebody else.
func External(destination string) bool {
	s := scheme(destination)
	return s == "http" || s == "https"
}

// scheme is the URL scheme a destination names, or empty for a path. A Windows drive
// letter is not a scheme, and neither is the colon in a file name.
func scheme(destination string) string {
	i := strings.IndexByte(destination, ':')
	if i <= 1 || strings.ContainsAny(destination[:i], "/?#") {
		return ""
	}
	return strings.ToLower(destination[:i])
}

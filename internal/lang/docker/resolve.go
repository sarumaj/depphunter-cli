package docker

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/oci"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type resolver struct {
	files map[string]bool
}

func newResolver(all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}}
	for _, f := range all {
		r.files[f.Path] = true
	}
	return r
}

// Implements: REQ-DOCKER-003, REQ-DOCKER-004, REQ-DOCKER-007
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	switch imp.Name {
	case kindBuild:
		// Compose reads a build context from the Compose file's own directory.
		p := path.Join(path.Dir(file), imp.Module)
		if strings.HasPrefix(p, "../") || p == ".." || !r.files[p] {
			return lang.Target{}
		}
		return lang.Target{Local: p}
	case kindImage:
		t := oci.Image(imp.Module)
		// A variable left in the name means the image itself is chosen at build time:
		// it is unresolved, under the reference as written. One left only in the tag
		// or digest names a known image at a version nobody can read off the file,
		// which is kept as written and pins nothing.
		if strings.Contains(t.Package, "$") {
			return lang.Target{Ecosystem: oci.Ecosystem, Package: imp.Module, Unresolved: true}
		}
		if strings.Contains(t.Version, "$") {
			t.Pinned, t.Requested = false, ""
		}
		return t
	}
	return lang.Target{}
}

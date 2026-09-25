package ci

import (
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

// Implements: REQ-CI-005, REQ-CI-011, REQ-CI-013
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	// A pinned reference may carry the version it documents in a comment.
	kind, requested, _ := strings.Cut(imp.Name, "\x00")
	switch kind {
	case kindLocal:
		return r.local(imp.Module)
	case kindImage:
		return image(imp.Module)
	case kindAction, kindWorkflow:
		return action(imp.Module, requested)
	case kindProject:
		project, ref, _ := strings.Cut(imp.Module, "@")
		// Without a ref the include follows the project's default branch.
		return lang.Target{
			Ecosystem: ecoGitLab, Package: project, Version: ref,
			Pinned: lang.Commit(ref), Floating: ref == "",
		}
	case kindComponent:
		name, version, ok := strings.Cut(imp.Module, "@")
		if !ok {
			// Without a version the component follows its project's default branch.
			return lang.Target{Ecosystem: ecoGitLab, Package: name, Floating: true}
		}
		return lang.Target{Ecosystem: ecoGitLab, Package: name, Version: version, Pinned: lang.Commit(version)}
	case kindTemplate:
		// A template is served by the GitLab instance and versioned with it: there is
		// no reference to pin, which makes it as loose as a dependency gets.
		return lang.Target{Ecosystem: ecoGitLab, Package: "template: " + imp.Module, Floating: true}
	case kindRemote:
		// Whatever that URL returns today, with nothing to say it is the same file
		// that was reviewed.
		return lang.Target{Ecosystem: ecoGitLab, Package: remoteName(imp.Module), Floating: true}
	}
	return lang.Target{}
}

// local resolves a path inside this repository. Both platforms read such a path from
// the repository root, whichever directory the file naming it sits in.
//
// Implements: REQ-CI-009
func (r *resolver) local(p string) lang.Target {
	p = path.Clean(strings.TrimPrefix(strings.TrimPrefix(p, "./"), "/"))
	if p == "" || strings.HasPrefix(p, "..") {
		return lang.Target{}
	}
	if r.files[p] {
		return lang.Target{Local: p}
	}
	// A composite action is named by its directory, but what it depends on is written
	// in the action.yml inside it: point at the file, so its own uses: chain on.
	for _, candidate := range []string{path.Join(p, "action.yml"), path.Join(p, "action.yaml")} {
		if r.files[candidate] {
			return lang.Target{Local: candidate}
		}
	}
	if r.dirs[p] {
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

// action resolves "owner/repo[/path][@ref]". The dependency is the repository: a
// reference to a sub-directory still runs whatever that repository holds at ref.
// Only a commit pins it - a tag can be moved to other code at any time.
//
// Implements: REQ-CI-002, REQ-CI-011, REQ-CI-013, REQ-CI-014, REQ-CI-015
func action(ref, requested string) lang.Target {
	spec, version, _ := strings.Cut(ref, "@")
	segments := strings.Split(spec, "/")
	if len(segments) < 2 || segments[0] == "" || segments[1] == "" {
		return lang.Target{Ecosystem: ecoActions, Package: spec, Unresolved: true}
	}
	t := lang.Target{
		Ecosystem: ecoActions,
		Package:   segments[0] + "/" + segments[1],
		Version:   version,
		Pinned:    lang.Commit(version),
		Floating:  version == "", // a local reusable workflow of another repository
	}
	if t.Pinned && requested != "" {
		t.Requested = requested
	}
	return t
}

// image resolves a container reference, "[registry/]name[:tag][@digest]". A tag is
// republished whenever its owner likes, so only a digest pins an image.
//
// Implements: REQ-CI-012, REQ-CI-013
func image(ref string) lang.Target {
	name, digest, hasDigest := strings.Cut(ref, "@")
	tag := ""
	// A colon in the registry part is a port, not a tag: "localhost:5000/img".
	if i := strings.LastIndex(name, ":"); i >= 0 && !strings.Contains(name[i:], "/") {
		name, tag = name[:i], name[i+1:]
	}
	if name == "" {
		return lang.Target{}
	}
	switch {
	case hasDigest:
		return lang.Target{Ecosystem: ecoOCI, Package: name, Version: digest, Requested: tag, Pinned: lang.Pinned(digest)}
	case tag == "":
		// No tag at all means whatever :latest is today, the loosest reference there
		// is; it names no version, so none is made up for it.
		return lang.Target{Ecosystem: ecoOCI, Package: name, Floating: true}
	}
	return lang.Target{Ecosystem: ecoOCI, Package: name, Version: tag}
}

// remoteName shortens a remote include's URL to host and path, which is what names it
// on the map; the scheme and any query carry no meaning there.
func remoteName(u string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	return s
}

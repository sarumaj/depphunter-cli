package cpp

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Fetched is content a build downloads when it is configured (CMake's FetchContent,
// ExternalProject and CPM.cmake): the package it is, the names its headers may go
// by (the name it is declared under, its repository's) and the directory of the
// build file declaring it.
type Fetched struct {
	Directory string
	Names     []string
	Target    lang.Target
}

// FetchReader reads what the project's build files fetch, and names the island the
// fetched packages belong to.
type FetchReader interface {
	Fetched(all []*scan.File) []Fetched
	FetchIsland() lang.Ecosystem
}

// Fetch adds the content the build fetches to what Library attributes includes to,
// its names folded as normalizeName folds an include's, the shallowest declaration first.
func (pk Packages) Fetch(list []Fetched) {
	p := pk.p
	for _, f := range list {
		names := make([]string, 0, len(f.Names))
		for _, n := range f.Names {
			if n = normalizeName(n); n != "" && !slices.Contains(names, n) {
				names = append(names, n)
			}
		}
		f.Names = names
		p.fetched = append(p.fetched, f)
	}
	slices.SortStableFunc(p.fetched, func(a, b Fetched) int { return cmp.Compare(lang.Depth(a.Directory), lang.Depth(b.Directory)) })
}

// fetchedFor finds the fetched content an include belongs to by its candidate
// names: the content declared nearest above the file (its directory, then each one
// above it), else the shallowest declared anywhere, since what one build file
// fetches is there for every target of the build.
//
// Implements: REQ-CPP-017
func (p *packages) fetchedFor(file string, names []string) *Fetched {
	var near, first *Fetched
	for i := range p.fetched {
		f := &p.fetched[i]
		if !slices.ContainsFunc(names, func(n string) bool { return slices.Contains(f.Names, n) }) {
			continue
		}
		if first == nil {
			first = f
		}
		above := f.Directory == "." || strings.HasPrefix(path.Dir(file)+"/", f.Directory+"/")
		if above && (near == nil || lang.Depth(f.Directory) > lang.Depth(near.Directory)) {
			near = f
		}
	}
	if near != nil {
		return near
	}
	return first
}

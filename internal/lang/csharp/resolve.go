package csharp

import (
	"encoding/xml"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type project struct {
	dir  string
	root string // root namespace
}

type resolver struct {
	projects []project         // longest root namespace first
	csDirs   map[string]bool   // directories holding C# files
	packages map[string]string // NuGet package id -> version
}

type msbuild struct {
	Groups []struct {
		RootNamespace string `xml:"RootNamespace"`
		AssemblyName  string `xml:"AssemblyName"`
	} `xml:"PropertyGroup"`
	Items []struct {
		Refs     []packageRef `xml:"PackageReference"`
		Versions []packageRef `xml:"PackageVersion"`
	} `xml:"ItemGroup"`
}

type packageRef struct {
	Include      string `xml:"Include,attr"`
	Version      string `xml:"Version,attr"`
	VersionChild string `xml:"Version"`
}

func (p packageRef) version() string {
	if p.Version != "" {
		return p.Version
	}
	return strings.TrimSpace(p.VersionChild)
}

func newResolver(all []*scan.File) *resolver {
	r := &resolver{csDirs: map[string]bool{}, packages: map[string]string{}}
	central := map[string]string{} // Directory.Packages.props
	for _, f := range all {
		base := path.Base(f.Path)
		switch {
		case strings.HasSuffix(base, ".cs"):
			for d := path.Dir(f.Path); !r.csDirs[d]; d = path.Dir(d) {
				r.csDirs[d] = true
				if d == "." {
					break
				}
			}
		case strings.HasSuffix(base, ".csproj") || base == "Directory.Packages.props":
			data, err := os.ReadFile(f.Abs)
			if err != nil {
				continue
			}
			var doc msbuild
			if xml.Unmarshal(data, &doc) != nil {
				continue
			}
			for _, ig := range doc.Items {
				for _, p := range ig.Versions {
					central[p.Include] = p.version()
				}
				for _, p := range ig.Refs {
					if p.Include != "" {
						r.packages[p.Include] = p.version()
					}
				}
			}
			if strings.HasSuffix(base, ".csproj") {
				root := strings.TrimSuffix(base, ".csproj")
				for _, g := range doc.Groups {
					if g.AssemblyName != "" {
						root = g.AssemblyName
					}
				}
				for _, g := range doc.Groups {
					if g.RootNamespace != "" {
						root = g.RootNamespace
					}
				}
				r.projects = append(r.projects, project{dir: path.Dir(f.Path), root: root})
			}
		}
	}
	for id, v := range r.packages {
		if v == "" {
			r.packages[id] = central[id]
		}
	}
	sort.Slice(r.projects, func(i, j int) bool { return len(r.projects[i].root) > len(r.projects[j].root) })
	return r
}

func within(ns, prefix string) (string, bool) {
	if ns == prefix {
		return "", true
	}
	if strings.HasPrefix(ns, prefix+".") {
		return ns[len(prefix)+1:], true
	}
	return "", false
}

func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	ns := imp.Module
	for _, p := range r.projects {
		rest, ok := within(ns, p.root)
		if !ok {
			continue
		}
		// The deepest existing folder along the namespace path.
		segments := []string{}
		if rest != "" {
			segments = strings.Split(rest, ".")
		}
		for n := len(segments); n >= 0; n-- {
			if d := path.Join(append([]string{p.dir}, segments[:n]...)...); r.csDirs[d] {
				return lang.Target{Local: d}
			}
		}
	}
	best := "" // NuGet ids are case-insensitive: the xunit package provides Xunit.*
	for id := range r.packages {
		if _, ok := within(strings.ToLower(ns), strings.ToLower(id)); ok && len(id) > len(best) {
			best = id
		}
	}
	if best != "" {
		// A PackageReference version is a minimum, but restore installs exactly it
		// when it exists: the versions that move are the wildcards and the ranges.
		v := r.packages[best]
		return lang.Target{Ecosystem: ecoNuGet, Package: best, Version: v, Pinned: lang.Pinned(v)}
	}
	segments := strings.Split(ns, ".")
	top := strings.Join(segments[:min(2, len(segments))], ".")
	switch segments[0] {
	case "System", "Microsoft", "Windows":
		return lang.Target{Ecosystem: ecoDotnet, Package: top}
	}
	return lang.Target{Ecosystem: ecoNuGet, Package: top, Unresolved: true}
}

package csharp

import (
	"bytes"
	"encoding/xml"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/nuget"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type project struct {
	directory string
	root      string // root namespace
}

type resolver struct {
	projects      []project       // longest root namespace first
	csDirectories map[string]bool // directories holding C# files
	*nuget.Store                  // packages, versions and what the lock files say they depend on
}

type msbuild struct {
	Groups []struct {
		RootNamespace string `xml:"RootNamespace"`
		AssemblyName  string `xml:"AssemblyName"`
	} `xml:"PropertyGroup"`
}

// Packages come from the nuget package, which the F# plugin reads the same way, so a
// package both languages use is one node. NuGet ids are case-insensitive: a
// reference to serilog takes the central version of Serilog.
//
// Implements: REQ-CS-001, REQ-CS-002, REQ-CS-003, REQ-CS-004
func newResolver(all []*scan.File) *resolver {
	r := &resolver{csDirectories: map[string]bool{}, Store: nuget.Read(all)}
	for _, f := range all {
		base := path.Base(f.Path)
		switch {
		case strings.HasSuffix(base, ".cs"):
			for d := path.Dir(f.Path); !r.csDirectories[d]; d = path.Dir(d) {
				r.csDirectories[d] = true
				if d == "." {
					break
				}
			}
		case strings.HasSuffix(base, ".csproj"):
			data, err := os.ReadFile(f.AbsolutePath)
			if err != nil {
				continue
			}
			var doc msbuild
			if xml.Unmarshal(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), &doc) != nil {
				continue
			}
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
			r.projects = append(r.projects, project{directory: path.Dir(f.Path), root: root})
		}
	}
	sort.Slice(r.projects, func(i, j int) bool { return len(r.projects[i].root) > len(r.projects[j].root) })
	return r
}

func within(namespace, prefix string) (string, bool) {
	if namespace == prefix {
		return "", true
	}
	if strings.HasPrefix(namespace, prefix+".") {
		return namespace[len(prefix)+1:], true
	}
	return "", false
}

// Implements: REQ-CS-001, REQ-CS-002, REQ-CS-004, REQ-CS-007
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	namespace := rawImport.Module
	for _, p := range r.projects {
		rest, ok := within(namespace, p.root)
		if !ok {
			continue
		}
		// The deepest existing folder along the namespace path.
		segments := []string{}
		if rest != "" {
			segments = strings.Split(rest, ".")
		}
		for n := len(segments); n >= 0; n-- {
			if d := path.Join(append([]string{p.directory}, segments[:n]...)...); r.csDirectories[d] {
				return lang.Target{Local: d}
			}
		}
	}
	return r.Namespace(namespace)
}

// Package perl analyzes Perl 5 and its CPAN distributions. A `use`, `no` or
// `require` of a module resolves to the project file it names (Foo::Bar is
// Foo/Bar.pm) under the file's `use lib` directories (literal, or computed from
// FindBin, __FILE__ or Mojo::File's curfile), its distribution's lib/, root and
// t/lib, the lib/ of each directory above it and the repository's; then to the
// hidden perl-std island for modules perl ships; then to the CPAN distribution
// that cpanfile.snapshot says provides the module, or that the manifests
// (cpanfile, META.json/META.yml, Makefile.PL, Build.PL, dist.ini) require it or a
// namespace above it from; then to a project file ending in the module's path or
// declaring its package; else to an unresolved distribution named after the
// module. Classes named by use parent, use base, Mojo::Base, Moose's extends and
// with, and Corinna's :isa are imports too, as are require/do of files.
//
// Packages on the map are distributions (libwww-perl, not LWP::UserAgent), as
// CPAN releases, versions and advisories are.
//
// Sources are read by a scanner of their own (lex.go), not the tree-sitter grammar:
// the grammar took 5-23 ms per file and parsed 3-6% of real files with errors.
package perl

import (
	"path"
	"sort"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoCPAN = "cpan"
	ecoStd  = "perl-std"
)

// Implements: REQ-PERL-001
type Plugin struct{}

func (Plugin) Name() string { return "perl" }
func (Plugin) Version() int { return 1 }

// Claims takes Perl sources - what scan labels Perl: .pl (unless it reads as
// Prolog), .pm, .t (when it reads as Perl), .psgi, .PL and scripts whose #! line
// runs perl - and the CPAN manifests (cpanfile, META.json, META.yml, MYMETA.*,
// Makefile.PL, Build.PL, dist.ini), except what Carton installs into local/ and a
// build's blib/.
//
// Implements: REQ-PERL-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary || ignored(f.Path) {
		return false
	}
	return manifestClass(path.Base(f.Path)) != "" || f.Lang == "Perl"
}

// Class tells the manifests apart from sources and from each other.
//
// Implements: REQ-PERL-001
func (Plugin) Class(f *scan.File) string { return manifestClass(path.Base(f.Path)) }

func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoCPAN, Name: "CPAN"},
		{ID: ecoStd, Name: "Perl core modules", Std: true},
	}
}

func (p Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all, p), nil
}

// Extract reads a source's imports and definitions, or a manifest's requirements;
// Makefile.PL and Build.PL are both.
//
// Implements: REQ-PERL-002, REQ-PERL-003, REQ-PERL-006
func (p Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	base := path.Base(f.Path)
	switch class := manifestClass(base); class {
	case classCpanfile, classMeta, classDistIni:
		return &lang.Extraction{Imports: readManifest(class, base, src).imports()}, nil
	case classMakefile, classBuild:
		ex := readSource(src)
		ex.Imports = append(ex.Imports, readManifest(class, base, src).imports()...)
		sort.SliceStable(ex.Imports, func(i, j int) bool { return ex.Imports[i].Line < ex.Imports[j].Line })
		return ex, nil
	}
	return readSource(src), nil
}

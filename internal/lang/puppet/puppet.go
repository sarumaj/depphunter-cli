// Package puppet analyzes Puppet: manifests (.pp), r10k's Puppetfile, a
// module's metadata.json and puppetlabs_spec_helper's .fixtures.yml.
//
// A manifest's references name classes (include, require, contain, class {
// 'x': }, Class['x'], inherits), defined and custom resource types, functions
// (x::y()), type aliases (X::Y), templates (template('x/y.erb'), epp()) and
// files (file(), puppet:///modules/x/...). The first segment is the module:
// a module of the repository (site-modules/, site/, modules/, dist/, or a
// module at the root, named by its metadata.json) links to the file Puppet's
// autoloader reads (x::y::z -> manifests/y/z.pp); another module is the
// Forge module (or git repository) the Puppetfile, metadata.json or
// .fixtures.yml declares for it. Puppet's own resource types, data types and
// functions are no dependency.
//
// Puppet is read by a lexer of its own (lex.go, source.go); there is no
// vendored grammar for it (REQ-PUPPET-009).
package puppet

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// ecoForge is the island of Puppet modules: Forge modules named by their
// slug (puppetlabs-stdlib), git modules by their repository.
const ecoForge = "puppet-forge"

// Classes of the files the plugin claims besides manifests.
const (
	classPuppetfile = "puppetfile"
	classMetadata   = "metadata"
	classFixtures   = "fixtures"
)

// Implements: REQ-PUPPET-001
type Plugin struct{}

func (Plugin) Name() string { return "puppet" }
func (Plugin) Version() int { return 1 }

// Claims takes manifests, Puppetfiles, .fixtures.yml and the metadata.json
// of a directory laid out as a Puppet module, except what r10k installed
// into modules/ beside a Puppetfile and what the spec helper installed into
// spec/fixtures/modules.
//
// Implements: REQ-PUPPET-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary {
		return false
	}
	switch class(f.Path) {
	case "":
		if path.Ext(f.Path) != ".pp" {
			return false
		}
	case classMetadata:
		if !moduleLayout(filepath.Dir(f.Abs)) {
			return false
		}
	}
	return !installed(f.Path, f.Abs)
}

// Implements: REQ-PUPPET-001
func (Plugin) Class(f *scan.File) string { return class(f.Path) }

func class(p string) string {
	switch path.Base(p) {
	case "Puppetfile":
		return classPuppetfile
	case "metadata.json":
		return classMetadata
	case ".fixtures.yml":
		return classFixtures
	}
	return ""
}

// installed reports whether a file lies where modules are installed: in
// spec/fixtures/modules, or in modules/ beside a Puppetfile.
func installed(rel, abs string) bool {
	segments := strings.Split(rel, "/")
	base := ""
	if a := filepath.ToSlash(abs); abs != "" && strings.HasSuffix(a, rel) {
		base = a[:len(a)-len(rel)]
	}
	for i, s := range segments[:len(segments)-1] {
		if s != "modules" {
			continue
		}
		if i >= 2 && segments[i-1] == "fixtures" && segments[i-2] == "spec" {
			return true
		}
		if base != "" && exists(filepath.FromSlash(base+strings.Join(append(segments[:i:i], "Puppetfile"), "/"))) {
			return true
		}
	}
	return false
}

var statMemo sync.Map // absolute path -> bool: it exists

func exists(p string) bool {
	if v, ok := statMemo.Load(p); ok {
		return v.(bool)
	}
	_, err := os.Stat(p)
	statMemo.Store(p, err == nil)
	return err == nil
}

// moduleLayout reports whether a directory is laid out as a Puppet module.
func moduleLayout(dir string) bool {
	for _, d := range []string{"manifests", "functions", "types", "plans", "tasks", filepath.Join("lib", "puppet"), "templates"} {
		if exists(filepath.Join(dir, d)) {
			return true
		}
	}
	return false
}

// Implements: REQ-PUPPET-008
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{{ID: ecoForge, Name: "Puppet modules"}}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Implements: REQ-PUPPET-002, REQ-PUPPET-003, REQ-PUPPET-005
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	switch class(f.Path) {
	case classPuppetfile:
		return depImports(readPuppetfile(src).deps, kindMod, nil), nil
	case classMetadata:
		m := readMetadata(src)
		if m == nil {
			return &lang.Extraction{}, nil
		}
		return depImports(m.deps, kindMetadata, []lang.Symbol{{Name: m.name, Kind: "module", Line: m.line}}), nil
	case classFixtures:
		return depImports(readFixtures(src), kindFixture, nil), nil
	}
	return extractSource(src), nil
}

// depImports makes each module a manifest names an import of it.
func depImports(deps []*dep, kind string, symbols []lang.Symbol) *lang.Extraction {
	ex := &lang.Extraction{Symbols: symbols}
	seen := map[string]bool{}
	for _, d := range deps {
		if !seen[d.key] {
			seen[d.key] = true
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: d.key, Module: d.key, Name: kind, Line: d.line})
		}
	}
	return ex
}

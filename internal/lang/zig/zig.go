// Package zig analyzes Zig sources (.zig) with the build system's build.zig and the
// package manager's build.zig.zon.
//
// @import("x.zig") and @embedFile("x") name files beside the importer;
// @import("std") and @import("builtin") the standard library (the hidden zig-std
// island); @import("root") the root source file of the compilation reaching the
// file; any other @import("name") a module the package's build code wires under
// that name (b.addModule, b.createModule, addImport, .imports, a dependency's
// .module("x")), else the build.zig.zon dependency of that name. A build.zig.zon's
// dependencies are packages of the zig island named by their URL, pinned by their
// .hash, or directories of the repository (.path). @cInclude inside @cImport is
// resolved as the cpp plugin resolves an #include (resolve.go).
//
// Zig is read by a lexer of its own (lex.go), not the vendored tree-sitter grammar
// (REQ-ZIG-012).
package zig

import (
	"path"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/cpp"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoZig = "zig"
	ecoStd = "zig-std"
)

// ignoredDirs are Zig's build output and package caches: zig-out, the local cache
// (.zig-cache, zig-cache before 0.13) and fetched packages (zig-pkg since 0.16).
var ignoredDirs = map[string]bool{".zig-cache": true, "zig-cache": true, "zig-out": true, "zig-pkg": true}

// Implements: REQ-ZIG-001
type Plugin struct{}

func (Plugin) Name() string { return "zig" }
func (Plugin) Version() int { return 1 }

// Claims takes Zig sources and build.zig.zon (and other .zon files), outside Zig's
// caches and build output.
//
// Implements: REQ-ZIG-001
func (Plugin) Claims(f *scan.File) bool {
	ext := path.Ext(f.Path)
	return (ext == ".zig" || ext == ".zon") && !f.Binary && !skipped(f.Path)
}

// Ecosystems: Zig packages, the standard library, and the islands a C header
// included through @cImport lands on.
func (Plugin) Ecosystems() []lang.Ecosystem {
	return append([]lang.Ecosystem{
		{ID: ecoZig, Name: "Zig"},
		{ID: ecoStd, Name: "Zig standard library", Std: true},
	}, cpp.Plugin{}.Ecosystems()...)
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Class tells build.zig.zon from other .zon files (data a program imports), which
// are claimed as files but declare nothing.
//
// Implements: REQ-ZIG-001
func (Plugin) Class(f *scan.File) string {
	if path.Base(f.Path) == "build.zig.zon" {
		return "manifest"
	}
	return ""
}

// Extract reads a build.zig.zon's dependencies, or a Zig file's imports and
// declarations.
//
// Implements: REQ-ZIG-002, REQ-ZIG-003, REQ-ZIG-007
func (p Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	if path.Ext(f.Path) == ".zon" {
		if p.Class(f) != "manifest" {
			return &lang.Extraction{Symbols: []lang.Symbol{}}, nil
		}
		return extractZon(src), nil
	}
	s := readSource(src)
	syms := s.symbols.List()
	if syms == nil {
		syms = []lang.Symbol{}
	}
	return &lang.Extraction{Imports: s.imports, Symbols: syms}, nil
}

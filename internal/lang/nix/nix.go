// Package nix analyzes Nix expressions (.nix), flakes (flake.nix and flake.lock)
// and the pinning tools niv (nix/sources.json) and npins (npins/sources.json).
//
// `import ./x.nix`, `callPackage ./x { }` and a NixOS module's `imports = [ ./a
// ./b ]` name files, a directory meaning its default.nix; any other relative path
// literal (`builtins.readFile ./x`, `src = ./src`) is an edge to that file or
// directory. A flake's inputs are packages of the nix island named by their URL
// (github:NixOS/nixpkgs is github.com/NixOS/nixpkgs), pinned by flake.lock;
// `<nixpkgs>` and registry names (flake:nixpkgs) are the registry alias nixpkgs,
// which floats. `inputs.x` elsewhere is an edge to that input. The attributes of
// package lists (buildInputs, nativeBuildInputs, packages, systemPackages...) are
// packages of the nixpkgs island, versioned by the project's nixpkgs input.
//
// Nix is read by a lexer and parser of its own (lex.go, parse.go), not the vendored
// tree-sitter grammar (REQ-NIX-011).
package nix

import (
	"path"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecosystemNix     = "nix"
	ecosystemNixpkgs = "nixpkgs"
)

// Implements: REQ-NIX-001
type Plugin struct{}

func (Plugin) Name() string { return "nix" }
func (Plugin) Version() int { return 1 }

// Claims takes .nix files, flake.lock, and niv's and npins' sources.json.
//
// Implements: REQ-NIX-001
func (p Plugin) Claims(f *scan.File) bool {
	return !f.Binary && (path.Ext(f.Path) == ".nix" || p.Class(f) != "")
}

// Class tells flake.nix (inputs and outputs), flake.lock and the pin files apart
// from other Nix and JSON files.
//
// Implements: REQ-NIX-001
func (Plugin) Class(f *scan.File) string { return class(f.Path) }

func class(p string) string {
	base := path.Base(p)
	switch {
	case base == "flake.nix":
		return "flake"
	case base == "flake.lock":
		return "lock"
	case base == "sources.json" && path.Base(path.Dir(p)) == "nix":
		return "niv"
	case base == "sources.json" && path.Base(path.Dir(p)) == "npins":
		return "npins"
	}
	return ""
}

// Ecosystems: flake inputs, channels and pinned sources; and nixpkgs' packages.
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecosystemNix, Name: "Nix flakes and sources"},
		{ID: ecosystemNixpkgs, Name: "Nixpkgs"},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all), nil
}

// Implements: REQ-NIX-002, REQ-NIX-003, REQ-NIX-004, REQ-NIX-005, REQ-NIX-008
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	switch c := class(f.Path); c {
	case "lock", "niv", "npins":
		return extractLock(c, source), nil
	case "flake":
		return extract(source, true), nil
	}
	return extract(source, false), nil
}

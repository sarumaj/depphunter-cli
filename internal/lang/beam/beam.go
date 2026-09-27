// Package beam analyzes the languages of the Erlang VM - Elixir and Erlang - and
// their Hex packages, which Mix and rebar3 share. Elixir module references (alias,
// import, require, use, remote calls, structs, behaviors) and Erlang remote calls,
// -behaviour, -import and includes resolve to the project file that defines the
// module (an index of defmodule declarations and of <module>.erl files, umbrella
// applications included), to the hidden elixir-std and erlang-std (OTP) islands,
// and to Hex packages by what mix.exs, rebar.config, mix.lock and rebar.lock
// declare and pin (resolve.go). Manifest dependencies and an .app.src's
// applications are imports of what they name.
//
// Both languages are read by lexers of their own (lex.go), not the tree-sitter
// grammars: those were slower and lost files to parse errors (see REQ-BEAM-012).
package beam

// cSpell: words: behaviour

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoHex    = "hex"
	ecoElixir = "elixir-std"
	ecoOTP    = "erlang-std"
)

// Implements: REQ-BEAM-001
type Plugin struct{}

func (Plugin) Name() string { return "beam" }
func (Plugin) Version() int { return 1 }

// Claims takes Elixir (.ex, .exs) and Erlang (.erl, .hrl) sources, application
// resource files (.app.src) and rebar.config, except what Mix and rebar3 fetch and
// build under deps/<app>/ and _build/.
//
// Implements: REQ-BEAM-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary {
		return false
	}
	segments := strings.Split(path.Dir(f.Path), "/")
	for i, seg := range segments {
		// deps/<app>/..., not a lib/app/deps/helper.ex of the project's own.
		if seg == "_build" || seg == "deps" && i+1 < len(segments) {
			return false
		}
	}
	base := path.Base(f.Path)
	switch strings.ToLower(path.Ext(base)) {
	case ".ex", ".exs", ".erl", ".hrl":
		return true
	}
	return base == "rebar.config" || strings.HasSuffix(base, ".app.src")
}

func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoHex, Name: "Hex"},
		{ID: ecoElixir, Name: "Elixir standard library", Std: true},
		{ID: ecoOTP, Name: "Erlang/OTP", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Extract dispatches on the extension: Elixir, Erlang, an application resource
// file (.src) or rebar.config (.config, the only such file claimed).
//
// Implements: REQ-BEAM-002, REQ-BEAM-003, REQ-BEAM-004, REQ-BEAM-005, REQ-BEAM-009
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	switch strings.ToLower(path.Ext(f.Path)) {
	case ".ex", ".exs":
		return extractElixir(src), nil
	case ".erl", ".hrl":
		return extractErlang(src), nil
	case ".src":
		return extractAppSrc(src), nil
	case ".config":
		return &lang.Extraction{Imports: rebarConfigImports(src)}, nil
	}
	return &lang.Extraction{}, nil
}

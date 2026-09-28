// Package dhall analyzes Dhall (.dhall) files, and holds the small Dhall
// reader (eval.go) the purescript plugin shares for spago's configurations.
//
// Every import of a file is an edge: a local path (./x.dhall, ../x) to the
// file in the repository, a remote URL to the package it downloads - the
// Prelude, a GitHub repository, else the URL's host and directory - with the
// version its path names, pinned when a sha256 integrity check freezes its
// content. Absolute paths, home-relative paths and environment variables
// (env:VAR) name nothing in the repository and are dropped. Each branch of an
// alternative (`a ? b`) counts, and `as Text`, `as Location` and `as Bytes`
// still import the file.
//
// spago's Dhall files (spago.dhall, packages.dhall, test.dhall) belong to the
// purescript plugin (Spago).
package dhall

import (
	"path"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// ecoDhall is the island of remote Dhall packages.
const ecoDhall = "dhall"

// Implements: REQ-DHALL-001
type Plugin struct{}

func (Plugin) Name() string { return "dhall" }
func (Plugin) Version() int { return 1 }

// Claims takes .dhall files except spago's.
//
// Implements: REQ-DHALL-001
func (Plugin) Claims(f *scan.File) bool {
	return !f.Binary && path.Ext(f.Path) == ".dhall" && !Spago(f.Path)
}

// Implements: REQ-DHALL-006
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{{ID: ecoDhall, Name: "Dhall packages"}}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all), nil
}

// Implements: REQ-DHALL-002, REQ-DHALL-003
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	return extractSource(src), nil
}

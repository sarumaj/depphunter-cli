// Package crystal analyzes Crystal sources (.cr) and the shards dependency
// manager's files: shard.yml, shard.lock and shard.override.yml.
//
// `require "./x"` and `require "../x/*"` name files relative to the requiring
// file (a glob expands to the files it matches). `require "x/y"` is looked up
// the way the compiler's CRYSTAL_PATH does: in the shards installed in lib/ -
// the directory's name is the shard's name - then in the project's own src/
// (a shard requiring itself, and the standard library requiring itself in
// crystal-lang/crystal), then in the standard library, then among the shards
// shard.yml and shard.lock name. Shards are git repositories and are named by
// their shard name, the name `require` and shard.yml use (resolve.go).
//
// Crystal is read by a small lexer of its own (lex.go), not the vendored
// tree-sitter grammar (REQ-CRYSTAL-010).
package crystal

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecosystemShards = "shards"
	ecosystemStd    = "crystal-std"
)

const (
	classShard    = "shard"
	classLock     = "lock"
	classOverride = "override"
)

// Implements: REQ-CRYSTAL-001
type Plugin struct{}

func (Plugin) Name() string { return "crystal" }
func (Plugin) Version() int { return 1 }

// Claims takes Crystal sources and the shards files, except what shards
// installed into a lib/ directory beside a shard.yml and the compiler's
// .crystal/ cache.
//
// Implements: REQ-CRYSTAL-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary || (path.Ext(f.Path) != ".cr" && class(f.Path) == "") {
		return false
	}
	return !installed(f)
}

// installed reports whether f lies in the compiler's cache or in a lib/
// directory that has a shard.yml beside it: there shards installs the
// dependencies. Other projects keep sources in lib/, so a lib/ without a
// shard.yml next to it is read.
func installed(f *scan.File) bool {
	segments := strings.Split(f.Path, "/")
	for i, s := range segments[:len(segments)-1] {
		if s == ".crystal" {
			return true
		}
		if s == "lib" && f.AbsolutePath != "" && strings.HasSuffix(filepath.ToSlash(f.AbsolutePath), f.Path) {
			base := f.AbsolutePath[:len(f.AbsolutePath)-len(f.Path)]
			if hasShard(filepath.Join(base, filepath.FromSlash(strings.Join(segments[:i], "/")))) {
				return true
			}
		}
	}
	return false
}

// hasShard reports whether an absolute directory has a shard.yml.
var hasShard = lang.MarkerMemo("shard.yml")

// Class tells the shards files apart from other YAML files and from other lock
// files.
//
// Implements: REQ-CRYSTAL-001
func (Plugin) Class(f *scan.File) string { return class(f.Path) }

func class(p string) string {
	switch path.Base(p) {
	case "shard.yml":
		return classShard
	case "shard.lock":
		return classLock
	case "shard.override.yml":
		return classOverride
	}
	return ""
}

// Implements: REQ-CRYSTAL-009
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecosystemShards, Name: "Crystal shards"},
		{ID: ecosystemStd, Name: "Crystal standard library", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all, os.Getenv), nil
}

// Implements: REQ-CRYSTAL-002, REQ-CRYSTAL-003, REQ-CRYSTAL-005
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	switch class(f.Path) {
	case classShard:
		return extractShard(source), nil
	case classLock:
		return extractLock(source), nil
	case classOverride:
		return extractOverride(source), nil
	}
	return extractSource(source), nil
}

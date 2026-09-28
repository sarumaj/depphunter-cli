// Package solidity analyzes Solidity sources (.sol) and the files Foundry,
// Soldeer and Hardhat projects declare their dependencies in: foundry.toml,
// remappings.txt, soldeer.lock and, beside a foundry.toml, .gitmodules.
//
// An import resolves as solc and the tools resolve it: a relative path is a
// file of the project; a path a remapping (foundry.toml's, remappings.txt's,
// or one Foundry or Soldeer infers for an installed library) maps is a file of
// the project or, when the remapping points into a library, the library as a
// package: a git submodule named by its repository and pinned by the commit
// git records for it, or a Soldeer package pinned by soldeer.lock; a Hardhat
// import from node_modules (@openzeppelin/contracts/...) is the npm package
// JavaScript's imports reach too, versioned by package.json and its lock; a
// path from the project's root is a file. Contracts, interfaces, libraries
// and their members, and the free functions, structs, enums, events, errors,
// value types and constants of a file are its symbols.
//
// Solidity is read by a scanner of its own (lex.go, source.go), not the
// vendored tree-sitter grammar (REQ-SOLIDITY-010).
package solidity

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecosystemSoldeer = "soldeer"
	ecosystemGit     = "git-submodule"
	ecosystemNPM     = "npm"
)

// The manifests, told apart from other files of their extensions by name.
const (
	classFoundry    = "foundry.toml"
	classRemappings = "remappings.txt"
	classLock       = "soldeer.lock"
	classGitmodules = ".gitmodules"
)

// Implements: REQ-SOLIDITY-001
type Plugin struct{}

func (Plugin) Name() string { return "solidity" }
func (Plugin) Version() int { return 1 }

// Claims takes .sol files, foundry.toml, remappings.txt, soldeer.lock and a
// .gitmodules beside a foundry.toml, except what lies in node_modules, in a
// Foundry project's lib/, dependencies/, out/ or cache/, or in a Hardhat
// project's artifacts/, cache/ or typechain-types/.
//
// Implements: REQ-SOLIDITY-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary {
		return false
	}
	switch base := path.Base(f.Path); {
	case strings.EqualFold(path.Ext(base), ".sol"):
	case base == classFoundry, base == classRemappings, base == classLock:
	case base == classGitmodules:
		root, ok := absoluteRoot(f)
		if !ok || !exists(path.Join(root, path.Dir(f.Path), classFoundry)) {
			return false
		}
	default:
		return false
	}
	return !ignored(f)
}

// Class tells the manifests apart from other files of their extensions.
//
// Implements: REQ-SOLIDITY-001
func (Plugin) Class(f *scan.File) string {
	switch base := path.Base(f.Path); base {
	case classFoundry, classRemappings, classLock, classGitmodules:
		return base
	}
	return ""
}

// Implements: REQ-SOLIDITY-009
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecosystemSoldeer, Name: "Soldeer packages"},
		{ID: ecosystemGit, Name: "Git submodules"},
		{ID: ecosystemNPM, Name: "npm"},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Implements: REQ-SOLIDITY-002, REQ-SOLIDITY-003, REQ-SOLIDITY-005
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	switch path.Base(f.Path) {
	case classFoundry:
		return extractFoundry(source), nil
	case classRemappings:
		return extractRemappings(source), nil
	case classLock:
		return extractLock(source), nil
	case classGitmodules:
		return extractGitmodules(source), nil
	}
	return scanSource(source), nil
}

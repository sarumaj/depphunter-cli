// Package shell analyzes shell scripts (sh, Bash, zsh, ksh, bats) and direnv's .envrc
// files.
//
// A script depends on the files it sources (source x, . x) and the scripts it runs
// (./x.sh, bash x.sh, "$DIR/x.sh", python3 tools/gen.py): both become edges to
// those project files. Paths are evaluated as far as the file says: literals,
// variables assigned earlier in it, and the idioms for the script's own directory
// ("$(dirname "$0")", "${BASH_SOURCE%/*}", "$(cd "$(dirname "${BASH_SOURCE[0]}")" &&
// pwd)", zsh's ${0:A:h}) and the repository root ($(git rev-parse --show-toplevel)).
// A path relative to the working directory is looked up beside the script, then at
// the repository root (resolve.go). Packages installed with pip, npm, go install,
// cargo install and gem install are packages of their ecosystems (install.go).
//
// Scripts are read by a scanner of its own (lex.go), not the vendored tree-sitter
// bash grammar, which is no faster and fails on most zsh files (REQ-SHELL-010).
package shell

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The ecosystems of the package managers read, with the ids their own plugins use,
// so the packages merge with the manifests' ones.
const (
	ecoPyPI   = "pypi"
	ecoNPM    = "npm"
	ecoGo     = "go"
	ecoCrates = "crates"
	ecoGems   = "rubygems"
)

// Import kinds, carried in RawImport.Name. A package's is its ecosystem, "@", and the
// version asked for.
const (
	kindSource = "source" // source x, . x
	kindExec   = "exec"   // ./x.sh, bash x.sh
	kindBats   = "bats"   // bats' load x: x.bash, else x
	kindEnvrc  = "envrc"  // direnv's source_env: a file, or a directory's .envrc
	kindUp     = "up"     // direnv's source_up: the nearest .envrc above
	kindDotenv = "dotenv" // direnv's dotenv
	kindFile   = "file"   // pip install -r requirements.txt
)

// Implements: REQ-SHELL-001
type Plugin struct{}

func (Plugin) Name() string { return "shell" }
func (Plugin) Version() int { return 1 }

// extensions are the shell script extensions; zsh-theme is Oh My Zsh's.
var extensions = map[string]bool{".sh": true, ".bash": true, ".zsh": true, ".ksh": true, ".bats": true, ".zsh-theme": true}

// startupFiles are the files shells and direnv read by name.
var startupFiles = map[string]bool{
	".envrc": true, ".profile": true, ".bashrc": true, ".bash_profile": true, ".bash_login": true,
	".bash_logout": true, ".bash_aliases": true, ".zshrc": true, ".zshenv": true, ".zprofile": true,
	".zlogin": true, ".zlogout": true, ".kshrc": true,
}

// Claims takes shell script extensions, the shells' and direnv's startup files, and
// files of no known language whose "#!" line runs a shell (a bin/ script without an
// extension).
//
// Implements: REQ-SHELL-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary {
		return false
	}
	base := path.Base(f.Path)
	if extensions[strings.ToLower(path.Ext(base))] || startupFiles[base] {
		return true
	}
	return scan.ShellInterpreter(f.Interpreter) && scan.Language(f.Path) == ""
}

// Implements: REQ-SHELL-007
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoPyPI, Name: "PyPI"},
		{ID: ecoNPM, Name: "npm"},
		{ID: ecoGo, Name: "Go modules"},
		{ID: ecoCrates, Name: "crates.io"},
		{ID: ecoGems, Name: "RubyGems"},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all), nil
}

// Extract depends on the extension beyond the content only for .envrc (direnv's
// commands) and .bats (bats' load), and the extension is part of the cache key.
//
// Implements: REQ-SHELL-002, REQ-SHELL-003
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	return extract(src, strings.ToLower(path.Ext(f.Path))), nil
}

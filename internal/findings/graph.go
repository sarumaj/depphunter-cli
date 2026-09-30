package findings

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Documents is the repository's own Markdown, which is where the links worth
// following are. The map has already read which files those are, so nothing is
// scanned twice to find them.
//
// Two kinds of Markdown are left out, because a finding against either would be a
// defect nobody is meant to fix: a vendored README links to the parts of its own
// repository that vendoring does not copy, and a fixture under testdata is wrong on
// purpose - a link that leads nowhere is what a link check is tested against.
//
// Implements: REQ-MD-015
func Documents(g *graph.Graph) []string {
	var out []string
	for n := range g.Of(graph.KindFile) {
		if n.Language == "Markdown" && !notOwnWriting(n.Path) {
			out = append(out, n.Path)
		}
	}
	return out
}

// notProse names the directories whose Markdown is not this repository's own writing.
//
// Implements: REQ-MD-015
var notProse = map[string]bool{
	"vendor": true, "node_modules": true, "third_party": true, "thirdparty": true,
	"site-packages": true, ".venv": true, "venv": true, "testdata": true,
}

// notOwnWriting reports whether a path lies under one of them, at any depth: a Go
// module's vendor/ is at the root, a workspace's node_modules and a package's
// testdata are not.
func notOwnWriting(p string) bool {
	for _, segment := range strings.Split(p, "/") {
		if notProse[segment] {
			return true
		}
	}
	return false
}

// Pinned is every external package the map fixes to one version: the only ones a
// vulnerability database can answer about, since a floating range resolves to
// something else on the next install. A package fixed to a git commit on a public
// forge carries the commit too (lang.GitPin), which OSV answers for whatever the
// ecosystem. private reports the organization's own packages and repositories
// (scope.Private.Match); nil treats nothing as private by pattern.
//
// Implements: REQ-FND-013, REQ-SUP-040, REQ-FND-026
func Pinned(g *graph.Graph, private func(ecosystem, name string) bool) []Package {
	if private == nil {
		private = func(string, string) bool { return false }
	}
	var out []Package
	for n := range g.Of(graph.KindPackage) {
		if n.Version == "" || n.Floating {
			continue
		}
		ecosystem := graph.EcosystemOf(n.Parent)
		p := Package{Ecosystem: ecosystem, Name: n.Name, Version: n.Version}
		commit, repository, public := lang.GitPin(ecosystem, n.Name, n.Version, n.Origin, n.Git)
		// A version that is itself a git reference (npm's github:owner/repo#<sha>, a
		// Python "@ git+<url>@<sha>") is no release of the ecosystem's index: only
		// the commit is a question, and the reference names the repository.
		reference := commit != "" && !strings.EqualFold(n.Version, commit) &&
			strings.Contains(strings.ToLower(n.Version), commit)
		// The commit of a private repository is not sent: its repository is on a
		// public forge (a private host would be the disclosure), the package is not
		// private by name (--private, GOPRIVATE: it is only private for having been
		// installed from outside every index), and neither the package nor its
		// repository matches a private pattern.
		if commit != "" && public && !(n.Private && n.Origin == "") &&
			!private(ecosystem, n.Name) && (repository == "" || !private(ecosystem, repository)) {
			p.Commit, p.Repository = commit, repository
		}
		// An organization's own package is not asked about: the question hands the
		// name and version of internal code to somebody else's server.
		p.CommitOnly = n.Private || reference
		if p.CommitOnly && p.Commit == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

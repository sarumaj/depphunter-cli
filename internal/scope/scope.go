// Package scope decides which dependencies are the organization's own.
//
// A map of an enterprise repository is mostly internal: packages from a Nexus or an
// Artifactory, modules on a GitHub Enterprise host, images in a private registry.
// depphunter can read all of those - the index configuration says where they come
// from - but two things it does for public packages must not be done for these:
//
//   - asking a public index what a package depends on, which both fails and tells
//     proxy.golang.org (or registry.npmjs.org) that the package exists; and
//   - asking the vulnerability database about it, which sends the name and the
//     version of internal code to a third party.
//
// Neither can be inferred reliably. A module path on a company host looks like any
// other module path, and an npm scope is a string. So it is declared: --private, or
// `private:` in the config file, holding the patterns that name what is yours. For Go
// the answer is usually already on the machine, in GOPRIVATE or GONOPROXY, and those
// are read so the common case needs no configuration at all.
package scope

import (
	"strings"

	"golang.org/x/mod/module"
)

// Private matches the packages an organization owns.
//
// A pattern is a comma-separated glob list with Go's GOPRIVATE meaning: it matches a
// package whose leading path elements match it, so "corp.example/..." is written
// "corp.example/*" and matches corp.example/team/lib. That is deliberate - anyone
// setting this has met the rule before - and it works as well for an npm scope
// ("@acme/*"), a Maven group ("com.acme.*") or a registry path ("harbor.corp/*").
//
// A pattern may be limited to one ecosystem by naming it first: "npm:@acme/*" matches
// only npm packages, where the bare "@acme/*" would match anything so named.
type Private struct {
	// any applies to every ecosystem; byEco only to the one that named it. Both are
	// comma-joined, because that is what MatchPrefixPatterns takes.
	any   string
	byEco map[string]string
}

// New builds a matcher. Patterns may themselves be comma-separated, so one flag, one
// environment variable and one config entry all say the same thing.
func New(patterns []string) *Private {
	p := &Private{byEco: map[string]string{}}
	var any []string
	for _, entry := range patterns {
		for _, pattern := range strings.Split(entry, ",") {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}
			// "npm:@acme/*" is scoped; "corp.example/*" is not. A colon inside what
			// looks like a host and port ("localhost:5000/*") is not a scope either,
			// so only a known-shaped ecosystem id counts - which is to say one with
			// no slash before the colon.
			if eco, rest, ok := strings.Cut(pattern, ":"); ok && rest != "" && !strings.Contains(eco, "/") && isEcosystem(eco) {
				p.byEco[eco] = join(p.byEco[eco], rest)
				continue
			}
			any = append(any, pattern)
		}
	}
	p.any = strings.Join(any, ",")
	return p
}

// Match reports whether a package is the organization's own.
func (p *Private) Match(eco, name string) bool {
	if p == nil || name == "" {
		return false
	}
	if p.any != "" && module.MatchPrefixPatterns(p.any, name) {
		return true
	}
	globs := p.byEco[eco]
	return globs != "" && module.MatchPrefixPatterns(globs, name)
}

// Empty reports whether nothing was declared private, which is the ordinary case for
// a repository of public dependencies.
func (p *Private) Empty() bool { return p == nil || (p.any == "" && len(p.byEco) == 0) }

// Patterns lists what was declared, ecosystem-scoped entries included, for the log
// line that says what a run considers private.
func (p *Private) Patterns() []string {
	if p == nil {
		return nil
	}
	var out []string
	if p.any != "" {
		out = append(out, strings.Split(p.any, ",")...)
	}
	for eco, globs := range p.byEco {
		for _, g := range strings.Split(globs, ",") {
			out = append(out, eco+":"+g)
		}
	}
	return out
}

// FromGoEnv reads what the machine already says about private Go modules. GOPRIVATE
// is the usual place; GONOPROXY is what actually governs whether a module is fetched
// from the proxy, and it defaults to GOPRIVATE when it is not set itself.
func FromGoEnv(env func(string) string) []string {
	var out []string
	for _, name := range []string{"GOPRIVATE", "GONOPROXY", "GONOSUMDB", "GONOSUMCHECK"} {
		for _, pattern := range strings.Split(env(name), ",") {
			if pattern = strings.TrimSpace(pattern); pattern != "" && pattern != "none" && pattern != "*" {
				out = append(out, "go:"+pattern)
			}
		}
	}
	return out
}

// The ecosystem ids a pattern may be limited to, as the language plugins emit them.
var ecosystems = map[string]bool{
	"go": true, "npm": true, "pypi": true, "crates": true, "maven": true,
	"nuget": true, "oci": true, "actions": true, "gitlab-ci": true, "powershell": true,
}

func isEcosystem(s string) bool { return ecosystems[strings.ToLower(s)] }

func join(have, add string) string {
	if have == "" {
		return add
	}
	return have + "," + add
}

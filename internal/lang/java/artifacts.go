package java

import "strings"

// Artifacts matches the classes another JVM language imports (Clojure's :import) to
// the Maven artifacts its manifests declare, by the rules a Java import is matched
// by: the table of known package prefixes, the prefixes an artifact's name gives,
// its group, and the undeclared artifacts a declared one brings along
// (jackson-annotations beside jackson-databind). Declare every artifact, call Finish
// once, then Match from any number of goroutines.
//
// Implements: REQ-JAVA-012, REQ-CLOJURE-006
type Artifacts struct {
	m        maven
	language Language
}

// NewArtifacts starts an empty set. l's Prefixes are the importing language's own
// root packages (clojure.), which no artifact's name may claim as its root package:
// org.clojure/clojure does not ship clojure.core.async.
func NewArtifacts(l Language) *Artifacts {
	return &Artifacts{m: maven{artifacts: map[string]*artifact{}}, language: l}
}

// Declare adds an artifact a manifest declares. Declared twice at different
// versions, it carries none.
func (a *Artifacts) Declare(group, name, version string) { a.m.addArtifact(group, name, version) }

// Finish indexes the declared artifacts; Match is ready after it.
func (a *Artifacts) Finish() { a.m.finish(a.language) }

// Match is an artifact a class matched.
type Match struct {
	Name    string // group:artifact
	Version string // the declared one; for an artifact that arrives with its group's declared ones, the version they share
	// Virtual marks an artifact nothing declares that comes with a declared artifact
	// of its group.
	Virtual bool
}

// Match finds the artifact shipping a class: the candidate with the longest package
// prefix of the class - one of the artifact's own or its group. Java's weaker rules
// (shared group segments, the group's last segment) are not applied: an importing
// language has rules of its own for a class no prefix places.
func (a *Artifacts) Match(class string) (Match, bool) {
	best, score := a.m.best(class, strings.Split(class, "."))
	if best == nil || score < prefixScore {
		return Match{}, false
	}
	return Match{Name: best.key(), Version: best.version, Virtual: best.virtual}, true
}

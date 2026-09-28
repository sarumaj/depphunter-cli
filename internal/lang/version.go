package lang

import "strings"

// Version specifiers differ per ecosystem, but the question the map asks is the same
// everywhere: does this specifier name one version, or will it drift? A caret range, a
// moving tag or a wildcard resolves to something else tomorrow; an exact version, a
// single-version range or a digest does not.

// Pinned reports whether spec fixes a single version. It accepts what the manifests
// write for one: a plain version ("1.2.3", "v1.2.3", a Go pseudo-version), Python's
// "==1.2.3", a bracketed single-version range ("[1.2.3]" in NuGet and Maven), a git
// commit, and an OCI digest. Everything else - ranges, carets, tildes, wildcards,
// "latest", "RELEASE" - floats.
//
// Implements: REQ-SUP-001
func Pinned(spec string) bool {
	s := strings.TrimSpace(spec)
	if s == "" {
		return false
	}
	if digest, ok := strings.CutPrefix(s, "sha256:"); ok {
		return isHex(digest, 64)
	}
	if isHex(s, 40) || isHex(s, 64) { // a git commit, the only immutable git reference
		return true
	}
	s = strings.TrimPrefix(s, "==") // Python and PowerShell write equality
	// "[1.2.3]" is one version; "[1.0,2.0)" is a range.
	if inner, ok := strings.CutPrefix(s, "["); ok {
		if inner, ok := strings.CutSuffix(inner, "]"); ok && !strings.Contains(inner, ",") {
			s = inner
		}
	}
	return exact(s)
}

// PinnedMaven reports whether a Maven version names one artifact. A plain version
// does, whatever its shape (Spring writes 1.2.3.RELEASE); a range, the LATEST and
// RELEASE keywords, Gradle's and Ivy's dynamic versions (1.+, latest.release), an
// unexpanded property and a snapshot - republished under the same name - do not.
//
// Implements: REQ-JAVA-008, REQ-CLOJURE-008
func PinnedMaven(v string) bool {
	v = strings.TrimSpace(v)
	switch {
	case v == "", strings.Contains(v, "$"), strings.Contains(v, "+"):
		return false
	case strings.HasPrefix(strings.ToLower(v), "latest."):
		return false
	case strings.HasPrefix(v, "["), strings.HasPrefix(v, "("):
		return Pinned(v) // "[1.2.3]" is one version, "[1.0,2.0)" is not
	case strings.EqualFold(v, "LATEST"), strings.EqualFold(v, "RELEASE"):
		return false
	case strings.HasSuffix(strings.ToUpper(v), "-SNAPSHOT"):
		return false
	}
	return true
}

// Commit reports whether reference is a full git commit, the only git reference that cannot
// be moved: a tag points wherever its owner last pushed it.
//
// Implements: REQ-CI-011
func Commit(reference string) bool {
	reference = strings.TrimSpace(reference)
	return isHex(reference, 40) || isHex(reference, 64) // SHA-1 today, SHA-256 where it is enabled
}

// PinnedSemver is Pinned for the ecosystems where a shortened version is itself a
// range: npm reads "1.2" as 1.2.x, so only a complete "1.2.3" pins.
//
// Implements: REQ-JS-010, REQ-SUP-002
func PinnedSemver(spec string) bool {
	if !Pinned(spec) {
		return false
	}
	s := strings.TrimSpace(spec)
	if isHex(s, 40) || isHex(s, 64) || strings.HasPrefix(s, "sha256:") {
		return true
	}
	return len(strings.Split(core(strings.TrimPrefix(strings.TrimPrefix(s, "=="), "v")), ".")) >= 3
}

// exact reports whether s is a version and nothing else: digits and dots, optionally
// followed by a pre-release or build part.
func exact(s string) bool {
	s = strings.TrimPrefix(s, "v")
	if s == "" || s[0] < '0' || s[0] > '9' {
		return false // an operator, a wildcard, "latest", a branch name
	}
	if strings.ContainsAny(s, "^~<>=*|,!()[] ") {
		return false
	}
	for _, segment := range strings.Split(core(s), ".") {
		if segment == "" {
			return false
		}
		for _, r := range segment {
			if r < '0' || r > '9' { // "1.2.x" and "1.2.RELEASE" are not one version
				return false
			}
		}
	}
	return true
}

// core is the numeric part of a version, without its pre-release or build suffix.
func core(s string) string {
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		return s[:i]
	}
	return s
}

func isHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

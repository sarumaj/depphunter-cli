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
func Pinned(spec string) bool {
	s := strings.TrimSpace(spec)
	if s == "" {
		return false
	}
	if digest, ok := strings.CutPrefix(s, "sha256:"); ok {
		return isHex(digest, 64)
	}
	if isHex(s, 40) || isHex(s, 64) { // a git commit, the only immutable git ref
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

// PinnedSemver is Pinned for the ecosystems where a shortened version is itself a
// range: npm reads "1.2" as 1.2.x, so only a complete "1.2.3" pins.
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
	for _, seg := range strings.Split(core(s), ".") {
		if seg == "" {
			return false
		}
		for _, r := range seg {
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

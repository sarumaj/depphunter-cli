package ada

import (
	"strconv"
	"strings"
)

// CompareVersions orders two Alire (semantic) versions: the dot-separated
// numbers first, a missing one counting as 0; then a version with a pre-release
// part (1.0.0-rc1) before the same version without one, and pre-release parts
// by their dot-separated identifiers, numbers before words. Build metadata
// (+x) is ignored.
//
// Implements: REQ-SUP-061
func CompareVersions(a, b string) int {
	coreA, prereleaseA := semverParts(a)
	cb, pb := semverParts(b)
	for i := 0; i < len(coreA) || i < len(cb); i++ {
		x, y := part(coreA, i), part(cb, i)
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	switch {
	case prereleaseA == pb:
		return 0
	case prereleaseA == "":
		return 1
	case pb == "":
		return -1
	}
	aParts, bParts := strings.Split(prereleaseA, "."), strings.Split(pb, ".")
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		na, ea := strconv.Atoi(aParts[i])
		nb, eb := strconv.Atoi(bParts[i])
		switch {
		case ea == nil && eb == nil:
			if na != nb {
				return na - nb
			}
		case ea == nil:
			return -1
		case eb == nil:
			return 1
		default:
			if c := strings.Compare(aParts[i], bParts[i]); c != 0 {
				return c
			}
		}
	}
	return len(aParts) - len(bParts)
}

// semverParts splits a version into its numbers and its pre-release part.
func semverParts(v string) ([]string, string) {
	v = strings.TrimSpace(v)
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	core, prerelease, _ := strings.Cut(v, "-")
	return strings.Split(core, "."), prerelease
}

func part(p []string, i int) int {
	if i >= len(p) {
		return 0
	}
	n, _ := strconv.Atoi(p[i])
	return n
}

// Satisfies reports whether an Alire version meets a constraint in Alire's
// notation: `*` or `any`; `=`, `/=` (or `≠`), `>`, `>=` (`≥`), `<`, `<=` (`≤`)
// and a bare version (exactly that one); `^1.2` (up to the next major version,
// 0.x included, as Alire's Semantic_Versioning reads it) and `~1.2.3` (up to
// the next minor version); `&` and `|` combinations, `&` binding tighter, with
// parentheses ignored. A term that is not a version (a commit, a branch) is met
// by none.
//
// Implements: REQ-SUP-061
func Satisfies(version, constraint string) bool {
	constraint = strings.NewReplacer("(", " ", ")", " ", "≠", "/=", "≥", ">=", "≤", "<=").Replace(constraint)
	if strings.TrimSpace(constraint) == "" {
		return true
	}
	for _, alternative := range strings.Split(constraint, "|") {
		ok := true
		for _, term := range strings.Split(alternative, "&") {
			ok = ok && meets(version, strings.TrimSpace(term))
		}
		if ok {
			return true
		}
	}
	return false
}

func meets(version, term string) bool {
	if term == "*" || strings.EqualFold(term, "any") {
		return true
	}
	operator := ""
	for _, o := range []string{"/=", ">=", "<=", "=", ">", "<", "^", "~"} {
		if strings.HasPrefix(term, o) {
			operator, term = o, strings.TrimSpace(term[len(o):])
			break
		}
	}
	if term == "" || term[0] < '0' || term[0] > '9' || !validSemver(term) {
		return false
	}
	c := CompareVersions(version, term)
	switch operator {
	case "", "=":
		return c == 0
	case "/=":
		return c != 0
	case ">":
		return c > 0
	case ">=":
		return c >= 0
	case "<":
		return c < 0
	case "<=":
		return c <= 0
	}
	core, _ := semverParts(term)
	next := []string{strconv.Itoa(part(core, 0) + 1)} // ^: the next major
	if operator == "~" {
		next = []string{core[0], strconv.Itoa(part(core, 1) + 1)}
	}
	// Below the next version's first pre-release, so that 2.0.0-rc1 is not ^1.2.
	return c >= 0 && CompareVersions(version, strings.Join(next, ".")+"-0") < 0
}

// Newest is the newest of versions that meets the constraint - a release
// before a pre-release - or "" when none does.
//
// Implements: REQ-SUP-061
func Newest(versions []string, constraint string) string {
	best := ""
	for _, v := range versions {
		if !validSemver(v) || !Satisfies(v, constraint) {
			continue
		}
		_, prerelease := semverParts(v)
		_, bestPre := semverParts(best)
		switch {
		case best == "":
			best = v
		case (prerelease == "") != (bestPre == ""):
			if prerelease == "" {
				best = v
			}
		case CompareVersions(v, best) > 0:
			best = v
		}
	}
	return best
}

// ValidConstraint reports whether c is a version constraint Satisfies can
// read - every term `*`, `any` or an operator and a version - rather than a
// commit, a branch or something else.
//
// Implements: REQ-SUP-061
func ValidConstraint(c string) bool {
	c = strings.NewReplacer("(", " ", ")", " ", "≠", "/=", "≥", ">=", "≤", "<=").Replace(c)
	if strings.TrimSpace(c) == "" {
		return true
	}
	for _, alternative := range strings.Split(c, "|") {
		for _, term := range strings.Split(alternative, "&") {
			term = strings.TrimSpace(term)
			if term == "*" || strings.EqualFold(term, "any") {
				continue
			}
			term = strings.TrimSpace(strings.TrimLeft(term, "/=<>^~"))
			if term == "" || term[0] < '0' || term[0] > '9' || !validSemver(term) {
				return false
			}
		}
	}
	return true
}

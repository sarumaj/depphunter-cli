package opam

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
)

// CompareVersions orders two opam versions as opam does (Debian's rules without
// the epoch): runs of non-digits compared character by character, where `~`
// sorts before anything - the end of the version included, so 1.0~beta1 comes
// before 1.0 - letters before other characters; runs of digits compared as
// numbers. It returns a negative number, zero or a positive number.
//
// Implements: REQ-SUP-054
func CompareVersions(a, b string) int {
	for a != "" || b != "" {
		var x, y string
		x, a = split(a, false)
		y, b = split(b, false)
		if c := compareText(x, y); c != 0 {
			return c
		}
		x, a = split(a, true)
		y, b = split(b, true)
		if c := compareNumber(x, y); c != 0 {
			return c
		}
	}
	return 0
}

// split cuts the leading run of digits (or of non-digits) off v.
func split(v string, digits bool) (string, string) {
	i := 0
	for i < len(v) && chars.IsDigit(v[i]) == digits {
		i++
	}
	return v[:i], v[i:]
}

// weight is a character's place in the order of non-digit runs; 0 is the end.
func weight(s string, i int) int {
	if i >= len(s) {
		return 0
	}
	c := s[i]
	switch {
	case c == '~':
		return -1
	case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
		return int(c)
	}
	return int(c) + 256
}

func compareText(a, b string) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		if d := weight(a, i) - weight(b, i); d != 0 {
			return d
		}
	}
	return 0
}

func compareNumber(a, b string) int {
	a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	return strings.Compare(a, b)
}

// Satisfies reports whether a version meets a constraint as Dep.Constraint
// writes it (`>= 5.6 & < 6`, `>= 0.17 | = 0.16`): `&` binds tighter than `|`,
// the grouping of the original formula being gone. A comparison with a variable
// rather than a version (`= version`) is taken as met, and so is no constraint.
//
// Implements: REQ-SUP-054
func Satisfies(version, constraint string) bool {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return true
	}
	for _, alternative := range strings.Split(constraint, "|") {
		ok := true
		for _, atom := range strings.Split(alternative, "&") {
			f := strings.Fields(atom)
			if len(f) != 2 || !ExactVersion(f[1]) || f[1] == "version" {
				continue
			}
			c := CompareVersions(version, f[1])
			switch f[0] {
			case "=":
				ok = ok && c == 0
			case "!=":
				ok = ok && c != 0
			case "<":
				ok = ok && c < 0
			case "<=":
				ok = ok && c <= 0
			case ">":
				ok = ok && c > 0
			case ">=":
				ok = ok && c >= 0
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// Newest is the newest of versions that meets the constraint, or "" when none
// does.
//
// Implements: REQ-SUP-054
func Newest(versions []string, constraint string) string {
	best := ""
	for _, v := range versions {
		if Satisfies(v, constraint) && (best == "" || CompareVersions(v, best) > 0) {
			best = v
		}
	}
	return best
}

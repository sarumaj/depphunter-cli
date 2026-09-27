// Package juliapkg holds what the julia plugin and the index both need to know about
// Julia's package manager, Pkg: which packages are Julia's standard libraries, how a
// Project.toml [compat] entry reads (a bare "1.2" is a caret range), and how the
// General registry's range-keyed Deps.toml and Compat.toml sections read ("0.21.2 -
// 0", "1"). It is not a plugin: the plugin reads projects with it and the index reads
// the registry with it.
package juliapkg

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// stdlibs are the packages a Julia installation ships: Base and Core, the standard
// libraries under share/julia/stdlib, and the JLL packages of the libraries Julia
// itself links. Some of them (Statistics, SparseArrays, DelimitedFiles, Distributed)
// are "upgradable": a newer release can come from the General registry, which a
// manifest entry with a git-tree-sha1 says.
var stdlibs = map[string]bool{
	"Base": true, "Core": true,
	"ArgTools": true, "Artifacts": true, "Base64": true, "CRC32c": true, "Dates": true, "DelimitedFiles": true,
	"Distributed": true, "Downloads": true, "FileWatching": true, "Future": true, "InteractiveUtils": true,
	"JuliaSyntaxHighlighting": true, "LazyArtifacts": true, "LibCURL": true, "LibGit2": true, "Libdl": true,
	"LinearAlgebra": true, "Logging": true, "Markdown": true, "Mmap": true, "NetworkOptions": true, "Pkg": true,
	"Printf": true, "Profile": true, "REPL": true, "Random": true, "SHA": true, "Serialization": true,
	"SharedArrays": true, "Sockets": true, "SparseArrays": true, "Statistics": true, "StyledStrings": true,
	"SuiteSparse": true, "TOML": true, "Tar": true, "Test": true, "UUIDs": true, "Unicode": true,
	"CompilerSupportLibraries_jll": true, "GMP_jll": true, "LLVMLibUnwind_jll": true, "LibCURL_jll": true,
	"LibGit2_jll": true, "LibSSH2_jll": true, "LibUnwind_jll": true, "MPFR_jll": true, "MbedTLS_jll": true,
	"MozillaCACerts_jll": true, "OpenBLAS_jll": true, "OpenLibm_jll": true, "PCRE2_jll": true,
	"SuiteSparse_jll": true, "Zlib_jll": true, "dSFMT_jll": true, "libLLVM_jll": true,
	"libblastrampoline_jll": true, "nghttp2_jll": true, "p7zip_jll": true,
}

// Stdlib reports whether a package name is one of Julia's standard libraries.
func Stdlib(name string) bool { return stdlibs[name] }

// RegistryDir is where the General registry keeps a package's files: the package's
// first letter in upper case, then its name (J/JSON, L/libpng_jll).
func RegistryDir(name string) string {
	r, n := utf8.DecodeRuneInString(name)
	if n == 0 {
		return ""
	}
	return string(unicode.ToUpper(r)) + "/" + name
}

// Version is a release version's numeric part, major first; a pre-release or build
// suffix is dropped.
type Version []int

// ParseVersion reads "1.2.3" (a leading v and a -pre or +build suffix allowed).
func ParseVersion(s string) (Version, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return nil, false
	}
	var v Version
	for _, p := range strings.Split(s, ".") {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, false
		}
		v = append(v, n)
	}
	return v, true
}

// At is the i-th part, 0 when it was not written.
func (v Version) At(i int) int {
	if i < len(v) {
		return v[i]
	}
	return 0
}

// Compare orders two versions, missing parts being 0.
func Compare(a, b Version) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		if x, y := a.At(i), b.At(i); x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// Range is the set of versions between two bounds. A bound has as many parts as were
// written: as an upper bound "1.5" admits every 1.5.x, as Pkg reads it. An empty
// upper bound is unbounded; hiExcl makes it exclusive (a caret's "< 2.0.0").
type Range struct {
	Lo, Hi Version
	hiExcl bool
	open   bool // no upper bound
}

// Contains reports whether v is in the range.
func (r Range) Contains(v Version) bool {
	if Compare(v, r.Lo) < 0 {
		return false
	}
	if r.open {
		return true
	}
	if r.hiExcl {
		return Compare(v, r.Hi) < 0
	}
	// An inclusive bound compares only the parts it has.
	for i := range r.Hi {
		if x, y := v.At(i), r.Hi[i]; x != y {
			return x < y
		}
	}
	return true
}

// bound reads one side of a registry range; "*" and "" are unbounded.
func bound(s string) (Version, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" {
		return nil, true
	}
	return ParseVersion(s)
}

// RegistryRange reads a range as the registry writes it, in a Deps.toml or
// Compat.toml section key or a Compat.toml value: "1" (every 1.x), "0.21.0" (that
// release), "0 - 0.20.0" and the compressed "0.1-0.3".
func RegistryRange(s string) (Range, bool) {
	s = strings.TrimSpace(s)
	lo, hi, ok := strings.Cut(s, " - ")
	if !ok {
		// "0.1-0.3": a hyphen between two versions. A pre-release suffix never
		// appears in a range, so the first hyphen splits.
		lo, hi, ok = strings.Cut(s, "-")
	}
	if !ok {
		hi = lo
	}
	l, ok1 := bound(lo)
	h, ok2 := bound(hi)
	if !ok1 || !ok2 {
		return Range{}, false
	}
	return Range{Lo: l, Hi: h, open: h == nil}, true
}

// CompatRanges reads a Project.toml [compat] entry: comma-separated specifiers, each
// a caret range when bare ("1.2" is ^1.2: [1.2.0, 2.0.0)), ^, ~, =, an inequality
// (>=, ≥, <, <=, ≤) or a hyphen range "1.2 - 1.5". It reports false when a part
// does not read.
func CompatRanges(spec string) ([]Range, bool) {
	var out []Range
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		r, ok := compatPart(part)
		if !ok {
			return nil, false
		}
		out = append(out, r)
	}
	return out, len(out) > 0
}

func compatPart(s string) (Range, bool) {
	if lo, hi, ok := strings.Cut(s, " - "); ok {
		return RegistryRange(lo + " - " + hi)
	}
	for _, op := range []string{">=", "≥", "<=", "≤", "<", "=", "^", "~"} {
		rest, ok := strings.CutPrefix(s, op)
		if !ok {
			continue
		}
		v, ok := ParseVersion(rest)
		if !ok {
			return Range{}, false
		}
		switch op {
		case ">=", "≥":
			return Range{Lo: v, open: true}, true
		case "<":
			return Range{Hi: v, hiExcl: true}, true
		case "<=", "≤":
			return Range{Hi: pad(v), hiExcl: false}, true
		case "=":
			return Range{Lo: v, Hi: v}, true
		case "~":
			return tilde(v), true
		}
		return caret(v), true
	}
	v, ok := ParseVersion(s)
	if !ok {
		return Range{}, false
	}
	return caret(v), true
}

func pad(v Version) Version {
	for len(v) < 3 {
		v = append(v, 0)
	}
	return v
}

// caret is Pkg's default: the leftmost non-zero part may not change ("^0.2.3" is
// [0.2.3, 0.3.0), "^0" is [0.0.0, 1.0.0)).
func caret(v Version) Range {
	hi := make(Version, 3)
	switch {
	case v.At(0) != 0 || len(v) == 1:
		hi[0] = v.At(0) + 1
	case v.At(1) != 0 || len(v) == 2:
		hi[1] = v.At(1) + 1
	default:
		hi[1], hi[2] = v.At(1), v.At(2)+1
	}
	return Range{Lo: v, Hi: hi, hiExcl: true}
}

// tilde lets only the last written part below the minor change ("~1.2.3" is [1.2.3,
// 1.3.0), "~1" is [1.0.0, 2.0.0)).
func tilde(v Version) Range {
	hi := make(Version, 3)
	switch {
	case len(v) == 1:
		hi[0] = v.At(0) + 1
	case v.At(0) == 0 && v.At(1) == 0 && len(v) == 3:
		hi[2] = v.At(2) + 1
	default:
		hi[0], hi[1] = v.At(0), v.At(1)+1
	}
	return Range{Lo: v, Hi: hi, hiExcl: true}
}

// ExactCompat is the version a [compat] entry pins: a single "=1.2.3" with all three
// parts ("=1.2" admits every 1.2.x, as Pkg reads a bound).
func ExactCompat(spec string) (string, bool) {
	s := strings.TrimSpace(spec)
	rest, ok := strings.CutPrefix(s, "=")
	rest = strings.TrimSpace(rest)
	if !ok || strings.ContainsAny(rest, ",<>=^~ ") {
		return "", false
	}
	v, ok := ParseVersion(rest)
	if !ok || len(v) < 3 {
		return "", false
	}
	return rest, true
}

// Newest picks the newest of versions that some range admits; with no ranges, the
// newest of all. It reports "" when none is admitted.
func Newest(versions []string, ranges []Range) string {
	best, bestV := "", Version(nil)
	for _, s := range versions {
		v, ok := ParseVersion(s)
		if !ok || strings.ContainsAny(s, "-+") { // a pre-release is chosen only by name
			continue
		}
		admitted := len(ranges) == 0
		for _, r := range ranges {
			if r.Contains(v) {
				admitted = true
				break
			}
		}
		if admitted && (best == "" || Compare(v, bestV) > 0) {
			best, bestV = s, v
		}
	}
	return best
}

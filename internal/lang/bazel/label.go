package bazel

import "strings"

// label is a parsed Bazel label: @repo//pkg:target, //pkg:target, :target, or a
// bare target relative to the package.
type label struct {
	repo      string // the repository name, without @ ("" for the main repository)
	hasRepo   bool   // an @ was written (@//x names the main repository)
	canonical bool   // @@repo: a canonical name, not an apparent one
	abs       bool   // the package is written (//pkg), not relative
	pkg       string
	target    string
}

// splitRepo cuts a label's repository part off: "@repo//x" -> "repo", "//x".
func splitRepo(s string) (repo, rest string, ok bool) {
	if !strings.HasPrefix(s, "@") {
		return "", s, false
	}
	s = strings.TrimPrefix(strings.TrimPrefix(s, "@"), "@")
	if i := strings.IndexAny(s, "/:"); i >= 0 {
		return s[:i], s[i:], true
	}
	return s, "", true
}

// parseLabel reads a label; ok is false for a string that cannot be one.
func parseLabel(s string) (label, bool) {
	var l label
	if s == "" || strings.ContainsAny(s, " \t\n$\"'") || strings.HasPrefix(s, "-") {
		return l, false
	}
	l.canonical = strings.HasPrefix(s, "@@")
	if repo, rest, ok := splitRepo(s); ok {
		l.repo, l.hasRepo = repo, true
		s = rest
		if s == "" { // @repo is @repo//:repo
			l.abs, l.target = true, repo
			return l, repo != ""
		}
	}
	switch {
	case strings.HasPrefix(s, "//"):
		l.abs = true
		s = s[2:]
		pkg, target, hasTarget := strings.Cut(s, ":")
		l.pkg = strings.TrimSuffix(pkg, "/")
		switch {
		case hasTarget:
			l.target = target
		case l.pkg == "":
			l.target = l.repo
		default:
			l.target = l.pkg[strings.LastIndex(l.pkg, "/")+1:]
		}
	case strings.HasPrefix(s, ":") && !l.hasRepo:
		l.target = s[1:]
	case l.hasRepo:
		return l, false // @repo:x is not a label
	default:
		l.target = s
	}
	if l.target == "" || strings.HasPrefix(l.pkg, "/") || strings.Contains(l.pkg, "..") || strings.HasPrefix(l.target, "/") {
		return l, false
	}
	return l, true
}

// moduleName is the module a canonical repository name belongs to: rules_go~0.50.1,
// rules_go~, rules_go+ (Bazel 8) and rules_go+ext+name all start with it.
func moduleName(canonical string) string {
	if i := strings.IndexAny(canonical, "~+"); i > 0 {
		return canonical[:i]
	}
	return canonical
}

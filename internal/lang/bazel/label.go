package bazel

import "strings"

// label is a parsed Bazel label: @repo//pkg:target, //pkg:target, :target, or a
// bare target relative to the package.
type label struct {
	repository    string // the repository name, without @ ("" for the main repository)
	hasRepository bool   // an @ was written (@//x names the main repository)
	canonical     bool   // @@repo: a canonical name, not an apparent one
	absolute      bool   // the package is written (//pkg), not relative
	packageName   string
	target        string
}

// splitRepository cuts a label's repository part off: "@repo//x" -> "repo", "//x".
func splitRepository(s string) (repository, rest string, ok bool) {
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
	if repository, rest, ok := splitRepository(s); ok {
		l.repository, l.hasRepository = repository, true
		s = rest
		if s == "" { // @repo is @repo//:repo
			l.absolute, l.target = true, repository
			return l, repository != ""
		}
	}
	switch {
	case strings.HasPrefix(s, "//"):
		l.absolute = true
		s = s[2:]
		packageName, target, hasTarget := strings.Cut(s, ":")
		l.packageName = strings.TrimSuffix(packageName, "/")
		switch {
		case hasTarget:
			l.target = target
		case l.packageName == "":
			l.target = l.repository
		default:
			l.target = l.packageName[strings.LastIndex(l.packageName, "/")+1:]
		}
	case strings.HasPrefix(s, ":") && !l.hasRepository:
		l.target = s[1:]
	case l.hasRepository:
		return l, false // @repo:x is not a label
	default:
		l.target = s
	}
	if l.target == "" || strings.HasPrefix(l.packageName, "/") || strings.Contains(l.packageName, "..") || strings.HasPrefix(l.target, "/") {
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

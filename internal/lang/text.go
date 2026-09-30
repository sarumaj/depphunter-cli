package lang

import (
	"bytes"
	"regexp"
	"strings"
)

// FoldAlphanumeric reduces a name to lower-case letters and digits, the part
// that a package name and the module or require path naming it usually
// share: swift-argument-parser and SwiftArgumentParser, GuzzleHttp and
// guzzlehttp.
func FoldAlphanumeric(s string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(s) {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// separatorFolder drops the separators FoldSeparators ignores.
var separatorFolder = strings.NewReplacer("-", "", "_", "", ".", "")

// FoldSeparators reduces a name to lower case without dashes, underscores and
// dots, for ecosystems whose package and module names differ in those alone:
// Hasql.Pool and hasql-pool.
func FoldSeparators(s string) string {
	return separatorFolder.Replace(strings.ToLower(s))
}

// WordSet makes a set of the space-separated words of s, which keeps long
// lists of known names (standard modules, system headers) readable in source.
func WordSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, word := range strings.Fields(s) {
		out[word] = true
	}
	return out
}

// BalancedParentheses reports whether s closes every parenthesis it opens, so
// that "(a) . (b)" is not mistaken for one parenthesized expression.
func BalancedParentheses(s string) bool {
	depth := 0
	for _, c := range s {
		switch c {
		case '(':
			depth++
		case ')':
			if depth--; depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// NPMPackageName is the package a JavaScript module specifier names, for the
// plugins that import npm packages from another language: "@mui/material/Button"
// is @mui/material, "react-dom/client" is react-dom.
func NPMPackageName(specifier string) string {
	parts := strings.Split(specifier, "/")
	if strings.HasPrefix(specifier, "@") && len(parts) > 1 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

// LineOf is the line of the first needle in source at or after byte offset
// from, 0 when there is none.
func LineOf(source []byte, needle string, from int) int {
	from = min(max(from, 0), len(source))
	i := bytes.Index(source[from:], []byte(needle))
	if i < 0 {
		return 0
	}
	return bytes.Count(source[:from+i], []byte("\n")) + 1
}

// jsonString matches a JSON string literal, capturing its contents.
var jsonString = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)

// JSONStringLines maps each string written in a JSON document to the first
// line it is on. encoding/json keeps no positions, and a manifest's
// dependencies should point at the line that declares them.
func JSONStringLines(source []byte) map[string]int {
	out := map[string]int{}
	for i, line := range strings.Split(string(source), "\n") {
		for _, m := range jsonString.FindAllStringSubmatch(line, -1) {
			if _, ok := out[m[1]]; !ok {
				out[m[1]] = i + 1
			}
		}
	}
	return out
}

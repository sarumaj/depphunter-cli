package cmake

import "strings"

// maxDepth bounds how deep variable values are expanded into each other, so a value
// referring to itself (set(A ${A} x) in a directory that inherits A) ends.
const maxDepth = 16

// expand substitutes the variable references ${NAME} in s, inner ones first
// (${${PREFIX}_DIR}). A name lookup does not know is kept as written when keep is
// set; otherwise expand fails. $ENV{...} and generator expressions $<...> are left
// alone: nothing here knows them.
//
// Implements: REQ-CMAKE-003
func expand(s string, lookup func(string) (string, bool), keep bool) (string, bool) {
	if !strings.Contains(s, "${") {
		return s, true
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '$' || i+1 >= len(s) || s[i+1] != '{' {
			b.WriteByte(s[i])
			i++
			continue
		}
		end := closing(s, i+2)
		if end < 0 {
			b.WriteString(s[i:])
			break
		}
		name, ok := expand(s[i+2:end], lookup, keep)
		if !ok {
			return "", false
		}
		if v, found := lookup(name); found {
			b.WriteString(v)
		} else if keep {
			b.WriteString("${" + name + "}")
		} else {
			return "", false
		}
		i = end + 1
	}
	return b.String(), true
}

// closing finds the "}" closing a reference whose name starts at i.
func closing(s string, i int) int {
	depth := 1
	for ; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return -1
}

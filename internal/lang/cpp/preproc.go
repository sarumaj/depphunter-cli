package cpp

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Include kinds, kept in RawImport.Name: the two forms search different places.
const (
	quoted  = "quote" // #include "x"
	bracket = "angle" // #include <x>
)

// frame is one open #if group. Only a literal 0 or 1 condition is evaluated: zero
// is `#if 0` (or false), one is `#if 1` (or true); any other condition is taken
// as live on every branch.
//
// Implements: REQ-CPP-007, REQ-CPP-008
type frame struct {
	zero, one bool
	dead      bool // the current branch is skipped
}

// preprocess reads the directives of a C or C++ file line by line, which is how the
// preprocessor reads them: comments are removed, continued lines are joined, and
// the branches of `#if 0` (and the `#else` of `#if 1`) are skipped. It returns the
// includes of the live branches and whether each line (1-based index) is dead.
//
// Implements: REQ-CPP-002, REQ-CPP-007
func preprocess(src []byte) (includes []lang.RawImport, dead []bool) {
	return scanDirectives(src, false)
}

// Preprocess is preprocess for Objective-C (the objc plugin), where `#import` is an
// include that is read once: it is returned like `#include`.
//
// Implements: REQ-OBJC-002
func Preprocess(src []byte) (includes []lang.RawImport, dead []bool) {
	return scanDirectives(src, true)
}

// Quoted is the RawImport.Name of a quoted include ("x.h"); an angle include has
// another.
const Quoted = quoted

func scanDirectives(src []byte, objc bool) (includes []lang.RawImport, dead []bool) {
	lines := strings.Split(string(src), "\n")
	dead = make([]bool, len(lines)+2)
	var stack []frame
	inComment := false
	deadFrames := 0 // frames on the stack whose current branch is skipped
	isDead := func() bool { return deadFrames > 0 }
	for i := 0; i < len(lines); i++ {
		start := i + 1
		var text string
		text, inComment = stripComments(lines[i], inComment)
		// A backslash at the end continues the line, directives included.
		for strings.HasSuffix(strings.TrimRight(lines[i], " \t\r"), "\\") && i+1 < len(lines) {
			text = strings.TrimSuffix(strings.TrimRight(text, " \t\r"), "\\")
			i++
			var more string
			more, inComment = stripComments(lines[i], inComment)
			text += more
		}
		for l := start; l <= i+1; l++ {
			dead[l] = isDead()
		}
		d, ok := strings.CutPrefix(strings.TrimSpace(text), "#")
		if !ok {
			continue
		}
		d = strings.TrimSpace(d)
		word := d
		if j := strings.IndexAny(d, " \t<\"("); j >= 0 {
			word = d[:j]
		}
		rest := strings.TrimSpace(d[len(word):])
		switch word {
		case "if":
			cond := strings.Trim(strings.Join(strings.Fields(rest), ""), "()")
			stack = append(stack, frame{zero: cond == "0" || cond == "false", one: cond == "1" || cond == "true"})
			stack[len(stack)-1].dead = stack[len(stack)-1].zero
			if stack[len(stack)-1].dead {
				deadFrames++
			}
		case "ifdef", "ifndef":
			stack = append(stack, frame{})
		case "elif", "elifdef", "elifndef", "else":
			if n := len(stack); n > 0 {
				f := &stack[n-1]
				was := f.dead
				switch {
				case f.one: // the first branch was taken; no other is
					f.dead = true
				case f.zero && word == "else":
					f.dead, f.zero = false, false
				case f.zero: // an #elif after #if 0 may or may not hold: live
					f.dead, f.zero = false, false
				}
				if f.dead && !was {
					deadFrames++
				} else if was && !f.dead {
					deadFrames--
				}
			}
		case "endif":
			if n := len(stack); n > 0 {
				if stack[n-1].dead {
					deadFrames--
				}
				stack = stack[:n-1]
			}
		case "include", "include_next", "import":
			if (word == "import" && !objc) || isDead() {
				continue // #import is Objective-C's and MSVC's type libraries
			}
			if name, kind, ok := target(rest); ok {
				spec := "#" + word + " \"" + name + "\""
				if kind == bracket {
					spec = "#" + word + " <" + name + ">"
				}
				includes = append(includes, lang.RawImport{Spec: spec, Module: name, Name: kind, Line: start})
			}
		}
	}
	return includes, dead
}

// target reads `"x"` or `<x>`; an include through a macro (#include CONFIG) names
// nothing to follow.
func target(rest string) (name, kind string, ok bool) {
	if len(rest) < 2 {
		return "", "", false
	}
	closing, kind := byte('"'), quoted
	switch rest[0] {
	case '"':
	case '<':
		closing, kind = '>', bracket
	default:
		return "", "", false
	}
	end := strings.IndexByte(rest[1:], closing)
	if end <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(rest[1 : end+1]), kind, true
}

// stripComments removes the comments of one line, given whether it starts inside a
// block comment, and says whether it ends inside one. String and character literals
// are stepped over, so a "/*" in one does not open a comment.
func stripComments(line string, inComment bool) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(line); i++ {
		if inComment {
			if strings.HasPrefix(line[i:], "*/") {
				inComment = false
				i++
				b.WriteByte(' ')
			}
			continue
		}
		switch c := line[i]; {
		case strings.HasPrefix(line[i:], "//"):
			return b.String(), false
		case strings.HasPrefix(line[i:], "/*"):
			inComment = true
			i++
		case c == '"' || c == '\'':
			j := i + 1
			for j < len(line) && line[j] != c {
				if line[j] == '\\' {
					j++
				}
				j++
			}
			b.WriteString(line[i:min(j+1, len(line))])
			i = j
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), inComment
}

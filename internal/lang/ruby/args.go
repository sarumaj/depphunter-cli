package ruby

import (
	"regexp"
	"strings"
)

// splitArguments splits a call's argument list at the commas outside strings and
// brackets, its parentheses and `#` comments taken off.
func splitArguments(s string) []string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = s[1 : len(s)-1]
	}
	var parts []string
	var current strings.Builder
	depth := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			current.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				i++
				current.WriteByte(s[i])
			} else if c == quote {
				quote = 0
			}
			continue
		case c == '\'' || c == '"':
			quote = c
		case c == '#':
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == ',' && depth == 0:
			parts = append(parts, strings.TrimSpace(current.String()))
			current.Reset()
			continue
		}
		current.WriteByte(c)
	}
	if last := strings.TrimSpace(current.String()); last != "" || len(parts) > 0 {
		parts = append(parts, last)
	}
	return parts
}

// literal reads a string literal without interpolation: 'x', "x", %q(x), %(x).
func literal(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		switch q := s[0]; {
		case q == '\'' && s[len(s)-1] == '\'':
			return s[1 : len(s)-1], !strings.Contains(s[1:len(s)-1], "'")
		case q == '"' && s[len(s)-1] == '"':
			inner := s[1 : len(s)-1]
			return inner, !strings.ContainsAny(inner, `"\`) && !strings.Contains(inner, "#{")
		}
	}
	for _, open := range []string{"%q(", "%Q(", "%("} {
		if inner, ok := strings.CutPrefix(s, open); ok && strings.HasSuffix(inner, ")") {
			inner = inner[:len(inner)-1]
			return inner, !strings.ContainsAny(inner, "()") && !strings.Contains(inner, "#{")
		}
	}
	return "", false
}

var optionKey = regexp.MustCompile(`^(?::([a-z_]+)\s*=>|([a-z_]+):)\s*`)

// option reads a keyword argument whose value is a string literal: `path: "x"` or
// `:path => "x"`.
func option(arguments []string, key string) (string, bool) {
	for _, a := range arguments {
		m := optionKey.FindStringSubmatch(a)
		if m == nil || m[1]+m[2] != key {
			continue
		}
		return literal(a[len(m[0]):])
	}
	return "", false
}

// options lists every keyword argument with a string literal value.
func options(arguments []string) map[string]string {
	out := map[string]string{}
	for _, a := range arguments {
		if m := optionKey.FindStringSubmatch(a); m != nil {
			if v, ok := literal(a[len(m[0]):]); ok {
				out[m[1]+m[2]] = v
			}
		}
	}
	return out
}

// directoryOfFile are the expressions naming the directory of the file they are in.
var directoryOfFile = map[string]bool{
	"__dir__": true, "File.dirname(__FILE__)": true, "File.dirname __FILE__": true,
	"File.expand_path(File.dirname(__FILE__))": true, "File.expand_path(__dir__)": true,
}

// evalPath evaluates the path a require or load names, when the file spells it out:
// string literals, __dir__ and File.dirname(__FILE__) (also interpolated at the
// start of a string), File.expand_path(path, base), File.join and `+`. A path
// relative to the file comes back as "__DIR__/..." - the file itself, as a base
// File.expand_path takes, as "__DIR__/_" - and is cleaned by the resolver, which
// knows the directory. Anything else (a variable, a method call) is not evaluated.
//
// Implements: REQ-RUBY-002
func evalPath(expression string) (string, bool) {
	expression = strings.TrimSpace(expression)
	for strings.HasPrefix(expression, "(") && strings.HasSuffix(expression, ")") && balanced(expression[1:len(expression)-1]) {
		expression = strings.TrimSpace(expression[1 : len(expression)-1])
	}
	if s, ok := literal(expression); ok {
		return s, s != ""
	}
	if directoryOfFile[strings.Join(strings.Fields(expression), " ")] {
		return "__DIR__", true
	}
	if expression == "__FILE__" {
		return "__DIR__/_", true
	}
	if strings.HasPrefix(expression, `"#{`) && strings.HasSuffix(expression, `"`) {
		inner := expression[3 : len(expression)-1]
		if i := strings.Index(inner, "}"); i > 0 && directoryOfFile[inner[:i]] {
			if rest, ok := literal(`"` + inner[i+1:] + `"`); ok {
				return "__DIR__" + rest, true
			}
		}
		return "", false
	}
	if parts := splitTop(expression, '+'); len(parts) > 1 {
		var out strings.Builder
		for _, p := range parts {
			s, ok := evalPath(p)
			if !ok {
				return "", false
			}
			out.WriteString(s)
		}
		return out.String(), true
	}
	for _, function := range []string{"File.expand_path", "File.join", "::File.expand_path", "::File.join"} {
		rest, ok := strings.CutPrefix(expression, function)
		if !ok {
			continue
		}
		arguments := splitArguments(rest)
		var values []string
		for _, a := range arguments {
			v, ok := evalPath(a)
			if !ok {
				return "", false
			}
			values = append(values, v)
		}
		switch {
		case len(values) == 0:
			return "", false
		case strings.HasSuffix(function, "join"):
			return strings.Join(values, "/"), true
		case len(values) == 1:
			return values[0], true
		case len(values) == 2 && strings.HasPrefix(values[1], "__DIR__"):
			return values[1] + "/" + values[0], true
		}
		return "", false
	}
	return "", false
}

// splitTop splits an expression at an operator outside strings and brackets.
func splitTop(s string, separator byte) []string {
	var parts []string
	depth, start := 0, 0
	var quote byte
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case quote != 0:
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '(' || c == '[':
			depth++
		case c == ')' || c == ']':
			depth--
		case c == separator && depth == 0:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

func balanced(s string) bool {
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

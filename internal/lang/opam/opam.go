// Package opam reads opam's package description format - `*.opam` files, `opam`
// files and the `*.opam.locked` files `opam lock` writes - as far as dependencies
// go: the package's name and version, `depends`, `depopts` and `pin-depends`. It is
// not a plugin: the ocaml plugin reads a project's descriptions with it, and the
// index reads the descriptions opam-repository serves.
package opam

import (
	"strings"
	"unicode/utf8"
)

// File is what a description says about dependencies.
type File struct {
	Name    string
	Version string
	Depends []Dep
	Depopts []Dep
	Pins    []Pin
}

// Dep is one package of a `depends` or `depopts` formula.
type Dep struct {
	Name string
	// Constraint is the version part of the package's filter as written, with its
	// connectives: `>= 5.6 & < 6`. Filters that are not about the version
	// (`with-test`, `os = "linux"`) are left out of it.
	Constraint string
	// Exact is the version when the constraint is a single `=`: what a lock writes.
	Exact string
	// Flags are the filter's variables: build, with-test, with-doc, dev, post...
	Flags []string
	Line  int
}

// Pin is a `pin-depends` entry: `["pkg.1.0" "git+https://host/repo.git#ref"]`.
type Pin struct {
	Name, Version, URL string
	Line               int
}

// Compiler reports whether a package is the compiler or one of the virtual packages
// that stand for what the compiler provides (base-unix, base-threads, ...): the
// runtime a project runs on, which a description names as it names packages.
func Compiler(name string) bool {
	switch name {
	case "ocaml", "ocaml-base-compiler", "ocaml-variants", "ocaml-system", "ocaml-config", "ocaml-beta",
		"base-unix", "base-threads", "base-bigarray", "base-domains", "base-nnp", "base-effects", "base-ocamlbuild":
		return true
	}
	for _, p := range []string{"ocaml-option-", "ocaml-options-", "host-arch-", "host-system-"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// ExactVersion reports whether v names one version (1.2.3, v0.16.0, 2.0.0~beta1)
// rather than a constraint or nothing.
func ExactVersion(v string) bool {
	if v == "" || strings.ContainsAny(v, " <>=!&|()") {
		return false
	}
	c := v[0]
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// SplitRef splits a pin's URL at its `#ref`.
func SplitRef(u string) (string, string) {
	if i := strings.LastIndexByte(u, '#'); i >= 0 {
		return u[:i], u[i+1:]
	}
	return u, ""
}

type tokKind uint8

const (
	tIdent tokKind = iota + 1
	tString
	tPunct
)

type token struct {
	k    tokKind
	s    string
	line int
}

// lex splits a description into tokens. Comments (`#` to the end of the line and
// nested `(* *)`) are dropped; strings are kept without their quotes.
func lex(src []byte) []token {
	var out []token
	line := 1
	n := len(src)
	for i := 0; i < n; {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '#':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '(' && i+1 < n && src[i+1] == '*':
			depth := 0
			for i < n {
				if src[i] == '(' && i+1 < n && src[i+1] == '*' {
					depth++
					i += 2
					continue
				}
				if src[i] == '*' && i+1 < n && src[i+1] == ')' {
					depth--
					i += 2
					if depth == 0 {
						break
					}
					continue
				}
				if src[i] == '\n' {
					line++
				}
				i++
			}
		case c == '"':
			start := line
			if i+2 < n && src[i+1] == '"' && src[i+2] == '"' {
				j := i + 3
				for j < n && !(src[j] == '"' && j+2 < n && src[j+1] == '"' && src[j+2] == '"') {
					if src[j] == '\n' {
						line++
					}
					j++
				}
				out = append(out, token{tString, string(src[min(i+3, n):min(j, n)]), start})
				i = min(j+3, n)
				continue
			}
			j := i + 1
			var b strings.Builder
			for j < n && src[j] != '"' {
				if src[j] == '\\' && j+1 < n {
					j++
					if src[j] == '\n' {
						line++
					}
				} else if src[j] == '\n' {
					line++
				}
				b.WriteByte(src[j])
				j++
			}
			out = append(out, token{tString, b.String(), start})
			i = min(j+1, n)
		case identByte(c):
			j := i
			for j < n {
				if identByte(src[j]) {
					j++
					continue
				}
				// pkg:var is one variable; `name:` followed by a space is a field.
				if src[j] == ':' && j+1 < n && identByte(src[j+1]) {
					j++
					continue
				}
				break
			}
			out = append(out, token{tIdent, string(src[i:j]), line})
			i = j
		case strings.IndexByte("=<>!+", c) >= 0:
			j := i
			for j < n && strings.IndexByte("=<>!+", src[j]) >= 0 {
				j++
			}
			out = append(out, token{tPunct, string(src[i:j]), line})
			i = j
		default:
			_, size := utf8.DecodeRune(src[i:])
			out = append(out, token{tPunct, string(src[i : i+size]), line})
			i += size
		}
	}
	return out
}

func identByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

// Read reads a description. It returns a result for any input.
func Read(src []byte) *File {
	tokens := lex(src)
	f := &File{}
	depth := 0
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t.k == tPunct {
			switch t.s {
			case "[", "{", "(":
				depth++
			case "]", "}", ")":
				depth = max(depth-1, 0)
			}
			continue
		}
		if depth != 0 || t.k != tIdent || i+1 >= len(tokens) || !punct(tokens[i+1], ":") {
			continue
		}
		// A field: its value runs to the next field or section at the top level.
		j := i + 2
		for d := 0; j < len(tokens); j++ {
			u := tokens[j]
			if d == 0 && u.k == tIdent && j+1 < len(tokens) && (punct(tokens[j+1], ":") || punct(tokens[j+1], "{") ||
				tokens[j+1].k == tString && j+2 < len(tokens) && punct(tokens[j+2], "{")) {
				break
			}
			if u.k == tPunct {
				switch u.s {
				case "[", "{", "(":
					d++
				case "]", "}", ")":
					d--
				}
			}
			if d < 0 {
				break
			}
		}
		value := tokens[i+2 : min(j, len(tokens))]
		switch t.s {
		case "name":
			if len(value) > 0 && value[0].k == tString {
				f.Name = value[0].s
			}
		case "version":
			if len(value) > 0 && value[0].k == tString {
				f.Version = value[0].s
			}
		case "depends":
			f.Depends = formula(value)
		case "depopts":
			f.Depopts = formula(value)
		case "pin-depends":
			f.Pins = pins(value)
		}
		i = j - 1
	}
	return f
}

// formula reads the packages of a dependency formula: every string outside a
// filter is a package, the `{...}` right after it its filter.
func formula(tokens []token) []Dep {
	var out []Dep
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t.k == tPunct && t.s == "{" {
			i = closing(tokens, i) // a filter of a group: (a | b) {build}
			continue
		}
		if t.k != tString {
			continue
		}
		d := Dep{Name: t.s, Line: t.line}
		if i+1 < len(tokens) && tokens[i+1].k == tPunct && tokens[i+1].s == "{" {
			end := closing(tokens, i+1)
			filter(tokens[i+2:min(end, len(tokens))], &d)
			i = end
		}
		out = append(out, d)
	}
	return out
}

// closing is the index of the bracket closing the one at i (len(tokens) if none).
func closing(tokens []token, i int) int {
	d := 0
	for j := i; j < len(tokens); j++ {
		if tokens[j].k != tPunct {
			continue
		}
		switch tokens[j].s {
		case "[", "{", "(":
			d++
		case "]", "}", ")":
			d--
			if d == 0 {
				return j
			}
		}
	}
	return len(tokens)
}

// filter reads a package's filter: version constraints (an operator not preceded
// by a variable, followed by a version) and the variables.
func filter(tokens []token, d *Dep) {
	var parts []string
	ops := 0
	exact := ""
	for i, t := range tokens {
		if t.k == tIdent {
			if i+1 < len(tokens) && isRelop(tokens[i+1]) {
				continue // os = "linux": a variable compared, not the version
			}
			if i > 0 && isRelop(tokens[i-1]) && (i < 2 || tokens[i-2].k != tIdent && tokens[i-2].k != tString) {
				// {= version}: the version as a variable
			} else {
				d.Flags = append(d.Flags, t.s)
				continue
			}
		}
		if !isRelop(t) || i+1 >= len(tokens) || tokens[i+1].k == tPunct {
			continue
		}
		if i > 0 && (tokens[i-1].k == tIdent || tokens[i-1].k == tString) {
			continue // the comparison of a variable
		}
		v := tokens[i+1].s
		if len(parts) > 0 {
			conn := "&"
			for k := i - 1; k >= 0 && tokens[k].k == tPunct; k-- {
				if tokens[k].s == "|" {
					conn = "|"
				}
			}
			parts = append(parts, conn)
		}
		parts = append(parts, t.s+" "+v)
		ops++
		if t.s == "=" && tokens[i+1].k == tString {
			exact = v
		}
	}
	d.Constraint = strings.Join(parts, " ")
	if ops == 1 {
		d.Exact = exact
	}
}

func isRelop(t token) bool {
	if t.k != tPunct {
		return false
	}
	switch t.s {
	case "=", "!=", "<", "<=", ">", ">=":
		return true
	}
	return false
}

// pins reads pin-depends: pairs of strings, in a list of lists or alone.
func pins(tokens []token) []Pin {
	var out []Pin
	for i := 0; i+1 < len(tokens); i++ {
		if tokens[i].k != tString || tokens[i+1].k != tString {
			continue
		}
		name, version, _ := strings.Cut(tokens[i].s, ".")
		out = append(out, Pin{Name: name, Version: version, URL: tokens[i+1].s, Line: tokens[i].line})
		i++
	}
	return out
}

func punct(t token, s string) bool { return t.k == tPunct && t.s == s }

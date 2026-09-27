package dart

import (
	"bytes"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Dart is read with a small scanner rather than the tree-sitter grammar. Measured on
// real projects (flutter/gallery, felangel/bloc, dart-lang/http), the grammar took
// 25-45 ms per file and failed on 5-11% of them - mostly deep Flutter widget trees -
// sometimes with an error node that swallowed the whole file from its first line.
// What the map needs from a Dart file is little and regular: the directives at the
// top, and the declarations at the top level and one level into a class body. The
// scanner reads both from a token stream that knows comments, every string form and
// balanced brackets, and skips function bodies whole.

type tokKind int

const (
	tIdent tokKind = iota
	tString
	tPunct
	tNumber
)

type token struct {
	kind tokKind
	text string // an identifier, a punctuator, a number; a string's value
	line int
	// plain marks a string without interpolation, whose value is its text.
	plain bool
}

// lexer turns Dart source into tokens. It never fails: an unterminated string or
// comment ends at the end of the file, and bytes that start nothing are skipped.
type lexer struct {
	src  []byte
	pos  int
	line int
	out  []token
}

// tokenize reads all of src.
func tokenize(src []byte) []token {
	src = bytes.TrimPrefix(src, []byte("\xef\xbb\xbf")) // a byte order mark
	l := &lexer{src: src, line: 1}
	if len(src) > 1 && src[0] == '#' && src[1] == '!' { // a script tag
		for l.pos < len(src) && src[l.pos] != '\n' {
			l.pos++
		}
	}
	for l.pos < len(l.src) {
		if t, ok := l.next(); ok {
			l.out = append(l.out, t)
		}
	}
	return l.out
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func isIdentPart(c byte) bool { return isIdentStart(c) || c >= '0' && c <= '9' }

// punctuators longer than one byte, longest first.
var punctuators = func() [][]byte {
	var out [][]byte
	for _, p := range []string{
		">>>=", "...?", "<<=", ">>=", "??=", "...", "~/=", ">>>",
		"=>", "==", "!=", "<=", ">=", "&&", "||", "??", "?.", "++", "--", "+=", "-=", "*=",
		"/=", "%=", "&=", "|=", "^=", "<<", "~/", "..",
	} {
		out = append(out, []byte(p))
	}
	return out
}()

// next reads one token; false when what it read was not one (space, a comment).
func (l *lexer) next() (token, bool) {
	c := l.src[l.pos]
	switch {
	case c == '\n':
		l.line++
		l.pos++
		return token{}, false
	case c == ' ' || c == '\t' || c == '\r' || c == '\f':
		l.pos++
		return token{}, false
	case c == '/' && l.peek(1) == '/':
		for l.pos < len(l.src) && l.src[l.pos] != '\n' {
			l.pos++
		}
		return token{}, false
	case c == '/' && l.peek(1) == '*':
		l.blockComment()
		return token{}, false
	case c == 'r' && (l.peek(1) == '\'' || l.peek(1) == '"'):
		l.pos++
		return l.str(true), true
	case c == '\'' || c == '"':
		return l.str(false), true
	case isIdentStart(c):
		start := l.pos
		for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
			l.pos++
		}
		return token{kind: tIdent, text: string(l.src[start:l.pos]), line: l.line}, true
	case c >= '0' && c <= '9' || c == '.' && l.peek(1) >= '0' && l.peek(1) <= '9':
		start := l.pos
		for l.pos < len(l.src) && (isIdentPart(l.src[l.pos]) || l.src[l.pos] == '.' && l.peek(1) >= '0' && l.peek(1) <= '9') {
			l.pos++
		}
		return token{kind: tNumber, text: string(l.src[start:l.pos]), line: l.line}, true
	}
	for _, p := range punctuators {
		if bytes.HasPrefix(l.src[l.pos:], p) {
			l.pos += len(p)
			return token{kind: tPunct, text: string(p), line: l.line}, true
		}
	}
	l.pos++
	return token{kind: tPunct, text: string(c), line: l.line}, true
}

func (l *lexer) peek(n int) byte {
	if l.pos+n < len(l.src) {
		return l.src[l.pos+n]
	}
	return 0
}

// blockComment skips a /* */ comment; Dart's nest.
func (l *lexer) blockComment() {
	depth := 0
	for l.pos < len(l.src) {
		switch {
		case l.src[l.pos] == '/' && l.peek(1) == '*':
			depth++
			l.pos += 2
		case l.src[l.pos] == '*' && l.peek(1) == '/':
			depth--
			l.pos += 2
			if depth == 0 {
				return
			}
		default:
			if l.src[l.pos] == '\n' {
				l.line++
			}
			l.pos++
		}
	}
}

// str reads a string literal at l.pos (its r prefix already taken): single or triple
// quoted, raw or with escapes and ${...} interpolations, which are read as code so a
// quote or brace inside one does not end anything early. A single-line string that
// reaches a line break is ended there, which keeps a broken file from being read as
// one long string.
func (l *lexer) str(raw bool) token {
	t := token{kind: tString, line: l.line, plain: true}
	q := l.src[l.pos]
	triple := l.peek(1) == q && l.peek(2) == q
	if triple {
		l.pos += 3
	} else {
		l.pos++
	}
	var b strings.Builder
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == q && (!triple || l.peek(1) == q && l.peek(2) == q):
			if triple {
				l.pos += 3
			} else {
				l.pos++
			}
			t.text = b.String()
			return t
		case c == '\n' && !triple:
			t.text = b.String()
			return t
		case c == '\\' && !raw:
			if l.peek(1) == '\n' {
				l.line++
			}
			b.WriteByte(l.peek(1))
			l.pos += 2
		case c == '$' && !raw && l.peek(1) == '{':
			t.plain = false
			l.pos += 2
			l.interpolation()
		case c == '$' && !raw && isIdentStart(l.peek(1)):
			t.plain = false
			b.WriteByte(c)
			l.pos++
		default:
			if c == '\n' {
				l.line++
			}
			b.WriteByte(c)
			l.pos++
		}
	}
	t.text = b.String()
	return t
}

// interpolation skips the code of a ${...} up to its closing brace.
func (l *lexer) interpolation() {
	depth := 1
	for l.pos < len(l.src) {
		t, ok := l.next()
		if !ok || t.kind != tPunct {
			continue
		}
		switch t.text {
		case "{":
			depth++
		case "}":
			if depth--; depth == 0 {
				return
			}
		}
	}
}

// Import kinds, carried in RawImport.Name.
const (
	kindImport = "import"
	kindExport = "export"
	kindPart   = "part"
	kindPartOf = "partof"
	kindDep    = "dep"    // a dependency pubspec.yaml declares; Module is its name
	kindMember = "member" // a pub workspace member directory of pubspec.yaml
)

// directives reads the directives at the top of a library: import and export with
// every configurable URI (`if (dart.library.io) 'io.dart'`), part, and part of by
// URI. They must precede every declaration, so reading stops at the first token that
// starts none, and returns where the declarations begin.
//
// Implements: REQ-DART-002
func directives(tokens []token) ([]lang.RawImport, int) {
	var out []lang.RawImport
	i := 0
	for i < len(tokens) {
		i = skipMetadata(tokens, i)
		if i >= len(tokens) || tokens[i].kind != tIdent {
			return out, i
		}
		kw := tokens[i].text
		switch {
		case kw == "library":
			i = skipTo(tokens, i, ";")
		case kw == "import" || kw == "export":
			end := skipTo(tokens, i, ";")
			out = append(out, uris(tokens[i:end], kw)...)
			i = end
		case kw == "part" && i+1 < len(tokens) && tokens[i+1].text == "of" && tokens[i+1].kind == tIdent:
			end := skipTo(tokens, i, ";")
			if s := tokens[i+2 : max(end-1, i+2)]; len(s) > 0 && s[0].kind == tString && s[0].plain {
				out = append(out, lang.RawImport{Spec: "part of '" + s[0].text + "'", Module: s[0].text, Name: kindPartOf, Line: tokens[i].line})
			}
			i = end
		case kw == "part" && i+1 < len(tokens) && tokens[i+1].kind == tString:
			end := skipTo(tokens, i, ";")
			if tokens[i+1].plain {
				out = append(out, lang.RawImport{Spec: "part '" + tokens[i+1].text + "'", Module: tokens[i+1].text, Name: kindPart, Line: tokens[i].line})
			}
			i = end
		default:
			return out, i
		}
	}
	return out, i
}

// uris reads an import or export directive's URIs: the default one, then each
// configuration's, spelled with the condition that selects it.
func uris(d []token, kw string) []lang.RawImport {
	if len(d) < 2 || d[1].kind != tString || !d[1].plain {
		return nil
	}
	out := []lang.RawImport{{Spec: kw + " '" + d[1].text + "'", Module: d[1].text, Name: kw, Line: d[0].line}}
	for j := 2; j < len(d); j++ {
		if d[j].kind != tIdent || d[j].text != "if" || j+1 >= len(d) || d[j+1].text != "(" {
			continue
		}
		k := j + 2
		var cond []string
		for k < len(d) && d[k].text != ")" {
			if d[k].kind == tString {
				cond = append(cond, "'"+d[k].text+"'")
			} else {
				cond = append(cond, d[k].text)
			}
			k++
		}
		if k+1 < len(d) && d[k+1].kind == tString && d[k+1].plain {
			c := strings.ReplaceAll(strings.Join(cond, ""), "==", " == ")
			out = append(out, lang.RawImport{
				Spec: kw + " '" + d[k+1].text + "' if (" + c + ")", Module: d[k+1].text, Name: kw, Line: d[k+1].line,
			})
			j = k + 1
		}
	}
	return out
}

// skipTo returns the index after the first `sep` at bracket depth 0 from i.
func skipTo(tokens []token, i int, sep string) int {
	depth := 0
	for ; i < len(tokens); i++ {
		if tokens[i].kind != tPunct {
			continue
		}
		switch tokens[i].text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		}
		if depth <= 0 && tokens[i].text == sep {
			return i + 1
		}
		if depth < 0 {
			return i
		}
	}
	return i
}

// skipMetadata skips annotations: @name, @a.b, @Name(args), @Name<T>(args).
func skipMetadata(tokens []token, i int) int {
	for i < len(tokens) && tokens[i].text == "@" && tokens[i].kind == tPunct {
		i++
		for i < len(tokens) && tokens[i].kind == tIdent {
			i++
			if i < len(tokens) && tokens[i].text == "." {
				i++
				continue
			}
			break
		}
		if i < len(tokens) && tokens[i].text == "<" {
			i = skipAngles(tokens, i)
		}
		if i < len(tokens) && tokens[i].text == "(" {
			i = matching(tokens, i) + 1
		}
	}
	return i
}

// matching returns the index of the bracket closing the one at i (or the last token).
func matching(tokens []token, i int) int {
	depth := 0
	for j := i; j < len(tokens); j++ {
		if tokens[j].kind != tPunct {
			continue
		}
		switch tokens[j].text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			if depth--; depth == 0 {
				return j
			}
		}
	}
	return len(tokens) - 1
}

// skipAngles skips type arguments starting at i ("<"), returning the index after them.
func skipAngles(tokens []token, i int) int {
	depth := 0
	for j := i; j < len(tokens); j++ {
		switch tokens[j].text {
		case "<":
			depth++
		case ">":
			depth--
		case ">>":
			depth -= 2
		case ">>>":
			depth -= 3
		case "(", ")", "{", "}", ";", "=":
			return j // not type arguments after all
		}
		if depth <= 0 {
			return j + 1
		}
	}
	return len(tokens)
}

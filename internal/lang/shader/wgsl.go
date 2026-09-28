package shader

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, kept in RawImport.Name.
const (
	nagaImport = "naga" // naga_oil's #import directive
	weslImport = "wesl" // WESL's import statement
)

// maxPaths bounds the paths one import statement expands to, and maxTree the
// nesting of its braces.
const (
	maxPaths = 256
	maxTree  = 16
)

// extractWGSL reads a WGSL or WESL file: naga_oil's #import directives, WESL's
// import statements and the module-scope declarations.
//
// Implements: REQ-SHADER-005
func extractWGSL(src []byte) *lang.Extraction {
	tokens, imports := lexWGSL(src)
	var set lang.SymbolSet
	i := 0
	for i < len(tokens) {
		t := tokens[i]
		switch {
		case t.is("@"): // an attribute: @vertex, @group(0), @if(FLAG)
			i++
			if i < len(tokens) && tokens[i].kind == tIdent {
				i++
			}
			if i < len(tokens) && tokens[i].is("(") {
				i = skipParens(tokens, i)
			}
			continue
		case t.kind != tIdent:
			i++
			continue
		}
		switch t.text {
		case "import":
			end := statementEnd(tokens, i+1)
			for _, p := range importPaths(tokens[i+1 : end]) {
				imports = append(imports, lang.RawImport{Spec: "import " + p, Module: p, Name: weslImport, Line: t.line})
			}
			i = end
		case "fn", "struct":
			kind := "func"
			if t.text == "struct" {
				kind = "struct"
			}
			if i+1 < len(tokens) && tokens[i+1].kind == tIdent {
				set.Add(tokens[i+1].text, kind, tokens[i+1].line)
			}
			i = skipBody(tokens, i+1)
		case "var", "const", "override", "alias":
			j := i + 1
			if j < len(tokens) && tokens[j].is("<") { // var<uniform>, var<storage, read>
				for j < len(tokens) && !tokens[j].is(">") && !tokens[j].is(";") {
					j++
				}
				j++
			}
			if j < len(tokens) && tokens[j].kind == tIdent {
				kind := map[string]string{"var": "var", "const": "const", "override": "const", "alias": "type"}[t.text]
				set.Add(tokens[j].text, kind, tokens[j].line)
			}
			i = statementEnd(tokens, j)
		default:
			i++
		}
	}
	return &lang.Extraction{Imports: imports, Symbols: set.List()}
}

// declStarts are the keywords starting a module-scope declaration: a statement
// missing its ';' ends before the next one.
var declStarts = map[string]bool{
	"fn": true, "struct": true, "var": true, "const": true, "override": true, "alias": true,
	"import": true, "enable": true, "requires": true, "diagnostic": true, "const_assert": true,
}

// statementEnd is the index after the ';' ending the statement at tokens[i:], or of
// the next declaration's keyword when the ';' is missing.
func statementEnd(tokens []token, i int) int {
	nest := 0
	for ; i < len(tokens); i++ {
		t := tokens[i]
		switch {
		case t.is("(") || t.is("[") || t.is("{"):
			nest++
		case t.is(")") || t.is("]") || t.is("}"):
			nest = max(nest-1, 0)
		case nest == 0 && t.is(";"):
			return i + 1
		case nest == 0 && t.kind == tIdent && declStarts[t.text]:
			return i
		}
	}
	return i
}

// skipBody steps past a declaration's body in braces: a function's or a struct's.
func skipBody(tokens []token, i int) int {
	for i < len(tokens) && !tokens[i].is("{") && !tokens[i].is(";") {
		if tokens[i].kind == tIdent && declStarts[tokens[i].text] {
			return i
		}
		i++
	}
	if i >= len(tokens) || tokens[i].is(";") {
		return i + 1
	}
	nest := 0
	for ; i < len(tokens); i++ {
		switch {
		case tokens[i].is("{"):
			nest++
		case tokens[i].is("}"):
			nest--
			if nest == 0 {
				return i + 1
			}
		}
	}
	return i
}

func skipParens(tokens []token, i int) int {
	nest := 0
	for ; i < len(tokens); i++ {
		switch {
		case tokens[i].is("("):
			nest++
		case tokens[i].is(")"):
			nest--
			if nest == 0 {
				return i + 1
			}
		}
	}
	return i
}

// importPaths expands an import tree (`a::{b, c::{d, e}}`, `"file.wgsl"::x`,
// `a::b as c`) into the paths it names. Several trees may follow each other
// separated by commas; anything else after a tree (naga_oil's old `#import a::b
// Item`) is ignored.
func importPaths(tokens []token) []string {
	var out []string
	i := 0
	for i < len(tokens) {
		i = importTree(tokens, i, "", 0, &out)
		for i < len(tokens) && tokens[i].kind == tIdent && tokens[i].text == "as" {
			i += 2
		}
		if i >= len(tokens) || !tokens[i].is(",") {
			break
		}
		i++
	}
	return out
}

// importTree reads one tree at tokens[i], appending its paths (prefixed) to out, and
// returns where it ends.
func importTree(tokens []token, i int, prefix string, depth int, out *[]string) int {
	path := prefix
	start := i
	for i < len(tokens) {
		t := tokens[i]
		switch {
		case t.kind == tIdent && (i == start || tokens[i-1].is("::")):
			path = join(path, t.text)
		case t.kind == tString && i == start && prefix == "":
			path = `"` + t.text + `"`
		case t.is("::"):
		case t.is("{") && i > start && tokens[i-1].is("::"):
			i++
			for i < len(tokens) && !tokens[i].is("}") {
				if depth >= maxTree {
					return len(tokens)
				}
				next := importTree(tokens, i, path, depth+1, out)
				for next < len(tokens) && tokens[next].kind == tIdent && tokens[next].text == "as" {
					next += 2
				}
				if next < len(tokens) && tokens[next].is(",") {
					next++
				}
				if next == i { // nothing read: step over what does not parse
					next++
				}
				i = next
			}
			return i + 1
		default:
			if path != prefix && path != "" && len(*out) < maxPaths {
				*out = append(*out, path)
			}
			return i
		}
		i++
	}
	if path != prefix && path != "" && len(*out) < maxPaths {
		*out = append(*out, path)
	}
	return i
}

func join(prefix, seg string) string {
	if prefix == "" {
		return seg
	}
	return prefix + "::" + seg
}

// lexWGSL splits a WGSL source into tokens and reads naga_oil's directives: an
// #import (continued over lines while its braces are open) becomes imports,
// #define_import_path and the conditionals (#ifdef, #else ...) are dropped - both
// branches of a conditional are read. Block comments nest, as WGSL's do.
func lexWGSL(src []byte) ([]token, []lang.RawImport) { return lexTokens(src, true) }

// lexTokens is lexWGSL; directives false reads a directive's own text.
func lexTokens(src []byte, directives bool) ([]token, []lang.RawImport) {
	var tokens []token
	var imports []lang.RawImport
	line, bol := 1, true
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '\n':
			line, bol = line+1, true
			i++
			continue
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			i++
			continue
		case c == '#' && bol && directives:
			start, at := i, line
			i = lineEnd(src, i)
			word, rest := directive(string(src[start:i]))
			if word != "import" {
				continue
			}
			// The directive goes on while its braces are open.
			text := rest
			for lines := 0; strings.Count(text, "{") > strings.Count(text, "}") && i < len(src) && lines < 64; lines++ {
				next := lineEnd(src, i+1)
				text += " " + stripLineComment(string(src[i+1:next]))
				line++
				i = next
			}
			ts, _ := lexTokens([]byte(text), false)
			for _, p := range importPaths(ts) {
				imports = append(imports, lang.RawImport{Spec: "#import " + p, Module: p, Name: nagaImport, Line: at})
			}
			continue
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			i = lineEnd(src, i)
			continue
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			depth := 0
			for i < len(src) {
				switch {
				case src[i] == '/' && i+1 < len(src) && src[i+1] == '*':
					depth++
					i += 2
				case src[i] == '*' && i+1 < len(src) && src[i+1] == '/':
					depth--
					i += 2
				default:
					if src[i] == '\n' {
						line++
					}
					i++
				}
				if depth == 0 {
					break
				}
			}
			continue
		}
		bol = false
		start, at := i, line
		switch {
		case identStart(c) || c >= 0x80:
			for i < len(src) && (identByte(src[i]) || src[i] >= 0x80) {
				i++
			}
			tokens = append(tokens, token{tIdent, string(src[start:i]), at})
		case c >= '0' && c <= '9':
			for i < len(src) && (identByte(src[i]) || src[i] == '.') {
				i++
			}
			tokens = append(tokens, token{tNumber, string(src[start:i]), at})
		case c == '"':
			i++
			for i < len(src) && src[i] != '"' && src[i] != '\n' {
				i++
			}
			tokens = append(tokens, token{tString, string(src[start+1 : i]), at})
			if i < len(src) && src[i] == '"' {
				i++
			}
		case c == ':' && i+1 < len(src) && src[i+1] == ':':
			i += 2
			tokens = append(tokens, token{tPunct, "::", at})
		default:
			i++
			tokens = append(tokens, token{tPunct, string(c), at})
		}
	}
	return tokens, imports
}

func lineEnd(src []byte, i int) int {
	for i < len(src) && src[i] != '\n' {
		i++
	}
	return i
}

// directive splits a naga_oil directive line into its word and the rest, without a
// line comment.
func directive(line string) (word, rest string) {
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
	word, rest, _ = strings.Cut(line, " ")
	if w, r, ok := strings.Cut(word, "\t"); ok {
		word, rest = w, r+" "+rest
	}
	return word, stripLineComment(rest)
}

// stripLineComment cuts a line at a // comment outside quotes: a quoted import may
// be a URL (embedded://crate/x.wgsl).
func stripLineComment(s string) string {
	quoted := false
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '"':
			quoted = !quoted
		case !quoted && s[i] == '/' && i+1 < len(s) && s[i+1] == '/':
			return s[:i]
		}
	}
	return s
}

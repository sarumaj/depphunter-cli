package javascript

import (
	"bytes"
	"cmp"
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
	"github.com/sarumaj/depphunter-cli/internal/lang/treesitter"
)

// Single-file components - Vue, Svelte and Astro - are HTML-like files whose
// JavaScript or TypeScript lives in blocks: <script> elements, and in Astro the
// frontmatter between two "---" fences at the top. Everything they import goes
// through the same resolver as a .ts file, so a component is simply another
// member of the npm ecosystem.
//
// The blocks are found by a small tag scanner rather than by the vue, svelte and
// astro tree-sitter grammars. Those grammars hand back a script's body as raw text
// all the same, which would have to be parsed a second time with the TypeScript
// grammar; the scanner does the one thing needed from the markup - tell a real
// top-level <script> from one in a comment, an attribute or an expression - in a
// page of code, and adds no grammar to the binary.
//
// Each block is parsed on its own, in a copy of the file in which everything but
// that block is blanked out with spaces while the line breaks stay. Line numbers
// therefore point into the component itself, and one block that does not parse
// cannot spill into the next.

// component reports whether p is a single-file component.
//
// Implements: REQ-JS-012
func component(p string) bool {
	switch path.Ext(p) {
	case ".vue", ".svelte", ".astro":
		return true
	}
	return false
}

// script is one block of code in a component.
type script struct {
	start, end int                 // the code's byte range in the file
	grammar    *treesitter.Grammar // nil for a block that is only a src reference
	source     string              // the src attribute of a <script> tag
	line       int                 // the tag's line, for src
}

// extractComponent reads a component's script blocks. The component is a symbol
// itself, named after its file as the frameworks name it on import, followed by
// the top-level definitions of every block.
//
// Implements: REQ-JS-012, REQ-JS-013, REQ-JS-014
func extractComponent(p string, source []byte, extraction *lang.Extraction, symbols *lang.SymbolSet) error {
	extension := path.Ext(p)
	symbols.Add(strings.TrimSuffix(path.Base(p), extension), "component", 1)
	var firstErr error
	for _, s := range componentScripts(extension, source) {
		if s.source != "" {
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: s.source, Module: s.source, Line: s.line})
		}
		if s.grammar == nil || s.start >= s.end {
			continue
		}
		if err := extract(s.grammar, blankOutside(source, s.start, s.end), extraction, symbols); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// blankOutside copies src with every byte outside [start, end) turned into a space,
// except line breaks.
func blankOutside(source []byte, start, end int) []byte {
	out := make([]byte, len(source))
	for i, b := range source {
		if (i < start || i >= end) && b != '\n' {
			b = ' '
		}
		out[i] = b
	}
	return out
}

// componentScripts finds the code blocks of a component with extension extension.
//
// Only the component's own scripts count. A Vue <script> inside <template> and a
// Svelte one inside <svelte:head> are markup the component renders; so is an Astro
// script with any attribute but src (is:inline, type="module", define:vars…),
// which Astro leaves as written rather than bundling. A <script> within an HTML
// comment, a quoted attribute or a template expression ({…}, Vue's {{…}}) is text.
// As in HTML, the first "</script" ends a script, which is why code that needs
// the string writes "<\/script>".
//
// Implements: REQ-JS-012, REQ-JS-013
func componentScripts(extension string, source []byte) []script {
	var out []script
	i := 0
	if extension == ".astro" {
		if s, next, ok := frontmatter(source); ok {
			out = append(out, s)
			i = next
		}
	}
	// Elements whose scripts are rendered rather than being the component's code.
	container := map[string]bool{}
	switch extension {
	case ".vue":
		container["template"] = true
	case ".svelte":
		container["svelte:head"] = true
	}
	depth := 0
	for i < len(source) {
		switch c := source[i]; {
		case bytes.HasPrefix(source[i:], []byte("<!--")):
			i = skipPast(source, i+4, "-->")
		case c == '<' && i+1 < len(source) && source[i+1] == '/':
			name, j := tagName(source, i+2)
			if container[strings.ToLower(name)] && depth > 0 {
				depth--
			}
			i = skipPast(source, j, ">")
		case c == '<' && i+1 < len(source) && chars.IsLetter(source[i+1]):
			name, j := tagName(source, i+1)
			name = strings.ToLower(name)
			attributeList, j, selfClosing := attributes(source, j, extension)
			switch {
			case name == "script" || name == "style":
				end, after := j, j
				if !selfClosing {
					end, after = rawTextEnd(source, j, name)
				}
				if name == "script" && depth == 0 {
					if s, ok := newScript(extension, attributeList, j, end); ok {
						s.line = bytes.Count(source[:i], []byte("\n")) + 1
						out = append(out, s)
					}
				}
				i = after
			case container[name] && !selfClosing:
				depth++
				i = j
			default:
				i = j
			}
		case c == '{' && extension == ".vue":
			if bytes.HasPrefix(source[i:], []byte("{{")) {
				i = skipPast(source, i+2, "}}")
			} else {
				i++
			}
		case c == '{':
			i = skipExpression(source, i)
		default:
			i++
		}
	}
	return out
}

// frontmatter finds Astro's frontmatter: TypeScript between a "---" line that
// opens the file (after blank space) and the next "---" line. next is where the
// template begins.
//
// Implements: REQ-JS-012
func frontmatter(source []byte) (s script, next int, ok bool) {
	i := 0
	if bytes.HasPrefix(source, []byte("\xef\xbb\xbf")) {
		i = 3
	}
	for i < len(source) && (source[i] == ' ' || source[i] == '\t' || source[i] == '\r' || source[i] == '\n') {
		i++
	}
	fence := func(line []byte) bool { return string(bytes.TrimRight(line, " \t\r")) == "---" }
	lineEnd := func(at int) int {
		if n := bytes.IndexByte(source[at:], '\n'); n >= 0 {
			return at + n + 1
		}
		return len(source)
	}
	open := lineEnd(i)
	if !fence(bytes.TrimSuffix(source[i:open], []byte("\n"))) {
		return script{}, 0, false
	}
	for at := open; at < len(source); {
		end := lineEnd(at)
		if fence(bytes.TrimSuffix(source[at:end], []byte("\n"))) {
			return script{start: open, end: at, grammar: tsGrammar}, end, true
		}
		at = end
	}
	// An unclosed fence: the whole file is frontmatter, as far as Astro is concerned.
	return script{start: open, end: len(source), grammar: tsGrammar}, len(source), true
}

// newScript turns a <script> tag whose code spans [start, end) into a block, or
// reports that it is not the component's JavaScript or TypeScript.
func newScript(extension string, attributes map[string]string, start, end int) (script, bool) {
	s := script{start: start, end: end, source: attributes["src"]}
	if extension == ".astro" {
		for name := range attributes {
			if name != "src" {
				return script{}, false
			}
		}
		s.grammar = tsGrammar // Astro compiles every processed script as TypeScript
		return s, true
	}
	language := strings.ToLower(attributes["lang"])
	switch t := strings.ToLower(attributes["type"]); t {
	case "", "module", "text/javascript", "application/javascript":
	case "text/typescript", "ts":
		language = cmp.Or(language, "ts")
	default:
		return script{}, false // JSON, templates and other data blocks
	}
	switch language {
	case "", "js", "javascript":
		s.grammar = jsGrammar
	case "ts", "typescript":
		s.grammar = tsGrammar
	case "tsx", "jsx":
		s.grammar = tsxGrammar
	default:
		return script{}, false // CoffeeScript and the like
	}
	return s, true
}

// attributes reads a tag's attributes from i, just past its name, and returns them
// by lower-cased name (a bare attribute has the value ""), the position after the
// tag and whether it closed itself. Values may be quoted with ', " or ` or be a
// {…} expression, which is how a ">" inside one does not end the tag.
func attributes(source []byte, i int, extension string) (map[string]string, int, bool) {
	attributes := map[string]string{}
	for i < len(source) {
		switch c := source[i]; {
		case c == '>':
			return attributes, i + 1, false
		case c == '/' && i+1 < len(source) && source[i+1] == '>':
			return attributes, i + 2, true
		case c == '{' && extension != ".vue": // {...spread} or {shorthand}
			i = skipExpression(source, i)
		case isSpace(c) || c == '/':
			i++
		default:
			start := i
			for i < len(source) && !isSpace(source[i]) && source[i] != '=' && source[i] != '>' && !(source[i] == '/' && i+1 < len(source) && source[i+1] == '>') {
				i++
			}
			name := strings.ToLower(string(source[start:i]))
			for i < len(source) && isSpace(source[i]) {
				i++
			}
			if i >= len(source) || source[i] != '=' {
				attributes[name] = ""
				continue
			}
			i++
			for i < len(source) && isSpace(source[i]) {
				i++
			}
			var value string
			switch {
			case i >= len(source):
			case source[i] == '"' || source[i] == '\'' || source[i] == '`':
				end := bytes.IndexByte(source[i+1:], source[i])
				if end < 0 {
					return attributes, len(source), false
				}
				value = string(source[i+1 : i+1+end])
				i += end + 2
			case source[i] == '{' && extension != ".vue":
				next := skipExpression(source, i)
				value = string(source[i+1 : max(i+1, next-1)])
				i = next
			default:
				start := i
				for i < len(source) && !isSpace(source[i]) && source[i] != '>' {
					i++
				}
				value = string(source[start:i])
			}
			attributes[name] = value
		}
	}
	return attributes, len(source), false
}

// rawTextEnd finds the end of a raw-text element's content from i: where its
// closing tag starts, and the position past that tag. Case does not matter, as in
// HTML.
func rawTextEnd(source []byte, i int, name string) (end, after int) {
	closing := []byte("</" + name)
	for at := i; ; {
		n := bytes.Index(source[at:], []byte("</"))
		if n < 0 {
			return len(source), len(source)
		}
		end = at + n
		if after := end + len(closing); after <= len(source) && bytes.EqualFold(source[end:after], closing) &&
			(after == len(source) || !chars.IsLetter(source[after])) {
			return end, skipPast(source, after, ">")
		}
		at = end + 2
	}
}

// skipExpression skips a {…} template expression starting at src[i] == '{',
// stepping over nested braces, strings, template literals and comments. An
// expression that never closes is taken for a literal "{".
func skipExpression(source []byte, i int) int {
	if end, ok := expressionEnd(source, i); ok {
		return end
	}
	return i + 1
}

func expressionEnd(source []byte, i int) (int, bool) {
	depth := 0
	for i < len(source) {
		switch c := source[i]; c {
		case '{':
			depth++
			i++
		case '}':
			depth--
			i++
			if depth == 0 {
				return i, true
			}
		case '"', '\'':
			// A quote left open at the end of the line is an apostrophe in JSX text.
			i++
			for i < len(source) && source[i] != c && source[i] != '\n' {
				if source[i] == '\\' {
					i++
				}
				i++
			}
			i++
		case '`':
			i++
			for i < len(source) && source[i] != '`' {
				switch {
				case source[i] == '\\':
					i += 2
				case source[i] == '$' && i+1 < len(source) && source[i+1] == '{':
					end, ok := expressionEnd(source, i+1)
					if !ok {
						return 0, false
					}
					i = end
				default:
					i++
				}
			}
			i++
		case '/':
			switch {
			case bytes.HasPrefix(source[i:], []byte("//")):
				i = skipPast(source, i, "\n")
			case bytes.HasPrefix(source[i:], []byte("/*")):
				i = skipPast(source, i+2, "*/")
			default:
				i++
			}
		default:
			i++
		}
	}
	return 0, false
}

// skipPast returns the position after the first separator at or after i, or len(src).
func skipPast(source []byte, i int, separator string) int {
	if i >= len(source) {
		return len(source)
	}
	if n := bytes.Index(source[i:], []byte(separator)); n >= 0 {
		return i + n + len(separator)
	}
	return len(source)
}

// tagName reads a tag name from i: letters, digits and the ":", "-", "." and "_"
// of svelte:head, custom elements and Astro's namespaced components.
func tagName(source []byte, i int) (string, int) {
	start := i
	for i < len(source) && (chars.IsLetter(source[i]) || source[i] >= '0' && source[i] <= '9' || strings.IndexByte(":-._", source[i]) >= 0) {
		i++
	}
	return string(source[start:i]), i
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }

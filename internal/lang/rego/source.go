package rego

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name.
const (
	kindImport = "import" // import data.a.b
	// kindReference is a reference to data.a.b.c in a rule (directly or through an
	// imported name). After the colon it carries the file's imports,
	// comma-separated ("ref:a.b,c"): one of them may link its package
	// already.
	kindReference = "ref:"
)

// keyword holds the words that do not start a rule.
var keyword = map[string]bool{
	"package": true, "import": true, "default": true, "else": true, "not": true, "some": true, "every": true,
	"with": true, "as": true, "if": true, "contains": true, "in": true, "true": true, "false": true, "null": true,
}

// extractSource reads a Rego file: its package, its imports of data, the
// references to data in its rules, and as symbols the package and the rules
// and functions it defines (each name once, however many definitions it
// has).
//
// Implements: REQ-REGO-002, REQ-REGO-003
func extractSource(source []byte) *lang.Extraction {
	tokens := lex(source)
	extraction := &lang.Extraction{}
	var symbols lang.SymbolSet
	defined := map[string]bool{}
	add := func(name, kind string, line int) {
		if !defined[name] {
			defined[name] = true
			symbols.Add(name, kind, line)
		}
	}
	seen := map[string]bool{}
	emit := func(spec, module, kind string, line int) {
		if !seen[spec] {
			seen[spec] = true
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
		}
	}
	aliases := map[string][]string{} // a name an import binds -> the data path
	var imported []string            // the data paths imported
	depth := 0
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t.kind == tPunctuation {
			switch t.text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				depth = max(depth-1, 0)
			}
			continue
		}
		if t.kind != tIdentifier {
			continue
		}
		dotted := i > 0 && tokens[i-1].kind == tPunctuation && tokens[i-1].text == "."
		if t.first && depth == 0 && !dotted {
			switch t.text {
			case "package":
				segments, end := readReference(tokens, i+1)
				if len(segments) > 0 {
					add(strings.Join(segments, "."), "package", t.line)
				}
				i = end - 1
				continue
			case "import":
				segments, end := readReference(tokens, i+1)
				alias := ""
				if len(segments) > 0 {
					alias = segments[len(segments)-1]
				}
				spec := ""
				if end+1 < len(tokens) && tokens[end].kind == tIdentifier && tokens[end].text == "as" && tokens[end+1].kind == tIdentifier {
					alias, spec = tokens[end+1].text, " as "+tokens[end+1].text
					end += 2
				}
				if len(segments) >= 2 && segments[0] == "data" {
					aliases[alias] = segments
					if len(imported) < maxSegments {
						imported = append(imported, strings.Join(segments[1:], "."))
					}
					emit("data."+strings.Join(segments[1:], ".")+spec, strings.Join(segments[1:], "."), kindImport, t.line)
				}
				i = end - 1
				continue
			case "default":
				if i+1 < len(tokens) && tokens[i+1].kind == tIdentifier {
					name, _ := ruleName(tokens, i+1)
					add(name, "rule", tokens[i+1].line)
				}
				continue
			}
			if !keyword[t.text] {
				name, end := ruleName(tokens, i)
				kind := "rule"
				if end < len(tokens) && tokens[end].kind == tPunctuation && tokens[end].text == "(" {
					kind = "function"
				}
				add(name, kind, t.line)
			}
		}
		if dotted {
			continue
		}
		var full []string
		if t.text == "data" {
			segments, _ := readReference(tokens, i)
			full = segments
		} else if a := aliases[t.text]; a != nil {
			segments, _ := readReference(tokens, i)
			if len(segments) > 1 {
				full = append(append([]string(nil), a...), segments[1:]...)
			}
		}
		if len(full) >= 2 {
			p := strings.Join(full[1:], ".")
			emit("data."+p, p, kindReference+strings.Join(imported, ","), t.line)
		}
	}
	extraction.Symbols = symbols.List()
	return extraction
}

// maxSegments bounds the length of a reference the reader keeps.
const maxSegments = 32

// readReference reads a reference starting at the identifier at i: a.b["c"].d.
// A bracket holding anything but a string ends it.
func readReference(tokens []token, i int) ([]string, int) {
	if i >= len(tokens) || tokens[i].kind != tIdentifier {
		return nil, i
	}
	segments := []string{tokens[i].text}
	i++
	for i+1 < len(tokens) && tokens[i].kind == tPunctuation {
		if len(segments) >= maxSegments {
			break
		}
		switch {
		case tokens[i].text == "." && tokens[i+1].kind == tIdentifier:
			segments = append(segments, tokens[i+1].text)
			i += 2
			continue
		case tokens[i].text == "[" && tokens[i+1].kind == tString && i+2 < len(tokens) && tokens[i+2].text == "]":
			segments = append(segments, tokens[i+1].text)
			i += 3
			continue
		}
		break
	}
	return segments, i
}

// ruleName is a rule's name, dotted when its head is a reference
// (a.b.c := 1), and the index past it.
func ruleName(tokens []token, i int) (string, int) {
	segments := []string{tokens[i].text}
	i++
	for i+1 < len(tokens) && tokens[i].kind == tPunctuation && tokens[i].text == "." && tokens[i+1].kind == tIdentifier {
		if len(segments) < maxSegments {
			segments = append(segments, tokens[i+1].text)
		}
		i += 2
	}
	return strings.Join(segments, "."), i
}

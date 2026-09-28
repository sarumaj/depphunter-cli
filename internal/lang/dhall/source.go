package dhall

import "github.com/sarumaj/depphunter-cli/internal/lang"

// extractSource reads a Dhall file: every import (a path, a URL or an
// environment variable) with the sha256 hash that checks it, and as symbols
// the file's top-level declarations (Declarations).
//
// Implements: REQ-DHALL-002, REQ-DHALL-003
func extractSource(src []byte) *lang.Extraction {
	tokens := Lex(src)
	ex := &lang.Extraction{}
	seen := map[string]bool{}
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t.Kind != TokURL && t.Kind != TokPath && t.Kind != TokEnv {
			continue
		}
		// import-hashed [as Text|Location|Bytes], where a URL may carry
		// `using <headers>` before its hash.
		j := i + 1
		if t.Kind == TokURL && label(tokens, j, "using") {
			j = skipOperand(tokens, j+1)
		}
		hash := ""
		if j < len(tokens) && tokens[j].Kind == TokHash {
			hash = tokens[j].Text
			j++
		}
		spec := t.Text
		if label(tokens, j, "as") && j+1 < len(tokens) && tokens[j+1].Kind == TokLabel {
			spec += " as " + tokens[j+1].Text
		}
		if seen[spec] {
			continue
		}
		seen[spec] = true
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: t.Text, Name: hash, Line: t.Line})
	}
	var symbols lang.SymbolSet
	defined := map[string]bool{}
	Declarations(src, func(name, kind string, line int) {
		if !defined[name] {
			defined[name] = true
			symbols.Add(name, kind, line)
		}
	})
	ex.Symbols = symbols.List()
	return ex
}

func label(tokens []Token, i int, text string) bool {
	return i < len(tokens) && tokens[i].Kind == TokLabel && tokens[i].Text == text
}

// skipOperand moves past the operand of `using`: a bracketed group or one
// token (an import or a variable).
func skipOperand(tokens []Token, i int) int {
	if i >= len(tokens) {
		return i
	}
	if tokens[i].Kind != TokPunct || tokens[i].Text != "(" {
		return i + 1
	}
	depth := 0
	for ; i < len(tokens); i++ {
		if tokens[i].Kind != TokPunct {
			continue
		}
		switch tokens[i].Text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			if depth--; depth <= 0 {
				return i + 1
			}
		}
	}
	return i
}

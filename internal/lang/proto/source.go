package proto

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// The kinds of import, as RawImport.Name carries them.
const (
	kindImport     = "import"    // import "a/b.proto";
	kindPublic     = "public"    // import public "a/b.proto";
	kindWeak       = "weak"      // import weak "a/b.proto";
	kindDependency = "dep"       // a buf.yaml deps entry or a buf.gen.yaml module input
	kindLock       = "lock"      // a buf.lock entry
	kindPlugin     = "plugin"    // a buf.gen.yaml remote plugin
	kindDirectory  = "directory" // a workspace directory, a v2 module path, a directory input
)

// parser reads a .proto file's declarations from its tokens. It does not validate:
// what it does not know it skips to the end of the statement.
type parser struct {
	tokens  []token
	i       int
	imports []lang.RawImport
	symbols lang.SymbolSet
}

// readProto extracts a .proto file's imports and declarations.
//
// Implements: REQ-PROTO-002, REQ-PROTO-003, REQ-PROTO-009
func readProto(source []byte) *lang.Extraction {
	p := &parser{tokens: lex(source)}
	p.body("", true)
	return &lang.Extraction{Imports: p.imports, Symbols: p.symbols.List()}
}

func (p *parser) peek(k int) token {
	if p.i+k < len(p.tokens) {
		return p.tokens[p.i+k]
	}
	return token{kind: tPunctuation, text: ""}
}

func (p *parser) isPunctuation(k int, s string) bool {
	t := p.peek(k)
	return t.kind == tPunctuation && t.text == s
}

func (p *parser) isIdentifier(k int) bool { return p.peek(k).kind == tIdentifier }

// dotted reads a possibly qualified name at the current position (".a.b.C", "a.b")
// and returns it with the number of tokens it spans; 0 when there is none.
func (p *parser) dotted(k int) (string, int) {
	var b strings.Builder
	n := 0
	if p.isPunctuation(k, ".") {
		b.WriteByte('.')
		n++
	}
	for {
		if !p.isIdentifier(k + n) {
			return "", 0
		}
		b.WriteString(p.peek(k + n).text)
		n++
		if !p.isPunctuation(k+n, ".") || !p.isIdentifier(k+n+1) {
			return b.String(), n
		}
		b.WriteByte('.')
		n++
	}
}

func qualify(scope, name string) string {
	if scope == "" {
		return name
	}
	return scope + "." + name
}

// body reads statements up to the closing brace of a block (consumed) or the end of
// the file. scope is the enclosing message's name ("Outer.Inner"), top whether this
// is the file's top level.
func (p *parser) body(scope string, top bool) {
	for p.i < len(p.tokens) {
		t := p.peek(0)
		if t.kind == tPunctuation {
			p.i++
			if t.text == "}" && !top {
				return
			}
			continue // an empty statement, a stray brace
		}
		if t.kind != tIdentifier {
			p.skipStatement(scope)
			continue
		}
		switch t.text {
		case "import":
			if top && p.importStatement() {
				continue
			}
		case "package":
			if name, n := p.dotted(1); top && n > 0 && p.isPunctuation(1+n, ";") {
				p.symbols.Add(name, "package", p.peek(1).line)
				p.i += n + 2
				continue
			}
		case "message", "enum", "service":
			if p.isIdentifier(1) && p.isPunctuation(2, "{") {
				name := qualify(scope, p.peek(1).text)
				p.symbols.Add(name, t.text, p.peek(1).line)
				p.i += 3
				switch t.text {
				case "message":
					p.body(name, false)
				case "service":
					p.service(name)
				default:
					p.skipBlock()
				}
				continue
			}
		case "extend":
			if name, n := p.dotted(1); n > 0 && p.isPunctuation(1+n, "{") {
				p.symbols.Add(qualify(scope, name), "extend", p.peek(1).line)
				p.i += n + 2
				p.body(scope, false) // its fields, and groups declared in the scope around it
				continue
			}
		case "oneof":
			if p.isIdentifier(1) && p.isPunctuation(2, "{") && !top {
				p.symbols.Add(qualify(scope, p.peek(1).text), "oneof", p.peek(1).line)
				p.i += 3
				p.body(scope, false) // a group inside a oneof belongs to the message
				continue
			}
		}
		p.skipStatement(scope)
	}
}

// importStatement reads `import [public|weak] "path";`, reporting whether it was one.
func (p *parser) importStatement() bool {
	k, kind, spec := 1, kindImport, "import "
	if p.isIdentifier(1) && (p.peek(1).text == "public" || p.peek(1).text == "weak") {
		kind = p.peek(1).text
		spec += kind + " "
		k = 2
	}
	if p.peek(k).kind != tString {
		return false
	}
	line := p.peek(0).line
	var name strings.Builder
	for p.peek(k).kind == tString { // adjacent strings are concatenated
		name.WriteString(p.peek(k).text)
		k++
	}
	p.imports = append(p.imports, lang.RawImport{Spec: spec + `"` + name.String() + `"`, Module: name.String(), Name: kind, Line: line})
	p.i += k
	if p.isPunctuation(0, ";") {
		p.i++
	}
	return true
}

// service reads a service's rpc methods, named Service.Method.
func (p *parser) service(name string) {
	for p.i < len(p.tokens) {
		if p.isPunctuation(0, "}") {
			p.i++
			return
		}
		if p.isIdentifier(0) && p.peek(0).text == "rpc" && p.isIdentifier(1) && p.isPunctuation(2, "(") {
			p.symbols.Add(name+"."+p.peek(1).text, "rpc", p.peek(1).line)
		}
		p.skipStatement("")
	}
}

// skipBlock skips to the brace closing the block just entered.
func (p *parser) skipBlock() {
	for depth := 1; p.i < len(p.tokens); p.i++ {
		if t := p.tokens[p.i]; t.kind == tPunctuation {
			switch t.text {
			case "{":
				depth++
			case "}":
				if depth--; depth == 0 {
					p.i++
					return
				}
			}
		}
	}
}

// skipStatement skips a field, an option, a reserved range or anything else to its
// semicolon. A block at the statement's own level is an option's aggregate value
// (`option (x) = { ... };`), an rpc's options or a proto2 group, whose body declares
// a nested message and is read as one.
func (p *parser) skipStatement(scope string) {
	start := p.i
	depth := 0
	for p.i < len(p.tokens) {
		t := p.tokens[p.i]
		if t.kind != tPunctuation {
			p.i++
			continue
		}
		switch t.text {
		case "(", "[":
			depth++
		case ")", "]":
			if depth > 0 {
				depth--
			}
		case ";":
			if depth == 0 {
				p.i++
				return
			}
		case "}":
			if depth == 0 {
				if p.i == start {
					p.i++ // never stand still
				}
				return // the enclosing block's end: a statement without its semicolon
			}
			depth--
		case "{":
			if depth > 0 {
				depth++
				break
			}
			if group, line := p.group(start); group != "" {
				name := qualify(scope, group)
				p.symbols.Add(name, "message", line)
				p.i++
				p.body(name, false)
				return
			}
			p.i++
			p.skipBlock()
			if p.tokens[start].kind == tIdentifier && p.tokens[start].text == "option" {
				continue // `option (x) = {...};` goes on to its semicolon
			}
			return
		}
		p.i++
	}
}

// group finds `group Name` among the tokens of the statement starting at start, the
// proto2 form declaring a nested message and a field at once.
func (p *parser) group(start int) (string, int) {
	for k := start; k+1 < p.i; k++ {
		if t := p.tokens[k]; t.kind == tIdentifier && t.text == "group" && p.tokens[k+1].kind == tIdentifier {
			return p.tokens[k+1].text, p.tokens[k+1].line
		}
	}
	return "", 0
}

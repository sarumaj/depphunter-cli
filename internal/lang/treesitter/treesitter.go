// Package treesitter adapts the pure-Go tree-sitter runtime (github.com/odvcencio/gotreesitter)
// to what language plugins need: parse a file, run one query, read captures. Plugins
// depend only on this package, so the runtime can be replaced in one place.
package treesitter

import (
	"fmt"
	"strings"
	"sync"

	ts "github.com/odvcencio/gotreesitter"
)

// parseTimeout bounds a single parse; the runtime then returns a partial tree,
// which is still useful for imports near the top of a file.
const parseTimeout = 3_000_000 // µs

type Grammar struct {
	name    string
	lang    *ts.Language
	query   *ts.Query
	parsers sync.Pool
}

// NewGrammar compiles query for lang. Capture names starting with "_" are helpers
// for predicates and are not reported.
func NewGrammar(name string, lang *ts.Language, query string) (*Grammar, error) {
	q, err := ts.NewQuery(query, lang)
	if err != nil {
		return nil, fmt.Errorf("%s query: %w", name, err)
	}
	g := &Grammar{name: name, lang: lang, query: q}
	g.parsers.New = func() any {
		p := ts.NewParser(lang)
		p.SetTimeoutMicros(parseTimeout)
		return p
	}
	return g, nil
}

// MustGrammar is NewGrammar for queries embedded in the program.
func MustGrammar(name string, lang *ts.Language, query string) *Grammar {
	g, err := NewGrammar(name, lang, query)
	if err != nil {
		panic(err)
	}
	return g
}

// Capture is one captured node of a query match.
type Capture struct {
	Name string // without the leading "@"
	Text string
	Line int // 1-based
	node *ts.Node
	g    *Grammar
	src  []byte
}

// Match holds the captures of one pattern match, in pattern order.
type Match []Capture

// Get returns the text of the first capture with the given name.
func (m Match) Get(name string) (string, bool) {
	for _, c := range m {
		if c.Name == name {
			return c.Text, true
		}
	}
	return "", false
}

// Matches parses src and runs the grammar's query. The tree is released before
// returning, so captures only keep their text, line and the ability to inspect
// their ancestors via the methods below — which must be called inside visit.
func (g *Grammar) Matches(src []byte, visit func(Match)) error {
	p := g.parsers.Get().(*ts.Parser)
	defer g.parsers.Put(p)
	tree, err := p.Parse(src)
	if err != nil {
		return err
	}
	if tree == nil || tree.RootNode() == nil {
		return nil
	}
	defer tree.Release()
	for _, m := range g.query.Execute(tree) {
		out := make(Match, 0, len(m.Captures))
		for _, c := range m.Captures {
			if strings.HasPrefix(c.Name, "_") {
				continue
			}
			out = append(out, Capture{
				Name: c.Name, Text: c.Text(src), Line: int(c.Node.StartPoint().Row) + 1,
				node: c.Node, g: g, src: src,
			})
		}
		if len(out) > 0 {
			visit(out)
		}
	}
	return nil
}

// EnclosingName returns the "name" field of the nearest ancestor whose type is one
// of types, e.g. the class of a method.
func (c Capture) EnclosingName(types ...string) string {
	return c.EnclosingField("name", types...)
}

// EnclosingField returns the text of field on the nearest ancestor whose type is one
// of types, e.g. the "type" of the Rust impl block around a method.
func (c Capture) EnclosingField(field string, types ...string) string {
	for n := c.node.Parent(); n != nil; n = n.Parent() {
		t := n.Type(c.g.lang)
		for _, want := range types {
			if t == want {
				if f := n.ChildByFieldName(field, c.g.lang); f != nil {
					return f.Text(c.src)
				}
				return ""
			}
		}
	}
	return ""
}

// EnclosingText returns the source text of the nearest ancestor whose type is one of
// types, for grammars whose nodes lack a "name" field (e.g. PowerShell classes).
func (c Capture) EnclosingText(types ...string) string {
	for n := c.node.Parent(); n != nil; n = n.Parent() {
		t := n.Type(c.g.lang)
		for _, want := range types {
			if t == want {
				return n.Text(c.src)
			}
		}
	}
	return ""
}

// SiblingFieldType returns the node type of field on the capture's parent, e.g. the
// kind of value a variable declarator is initialised with.
func (c Capture) SiblingFieldType(field string) string {
	if p := c.node.Parent(); p != nil {
		if f := p.ChildByFieldName(field, c.g.lang); f != nil {
			return f.Type(c.g.lang)
		}
	}
	return ""
}

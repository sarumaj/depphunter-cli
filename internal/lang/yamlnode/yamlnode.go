// Package yamlnode reads gopkg.in/yaml.v3 node trees, which the manifest readers use
// instead of decoding into structs so that every value keeps its line.
//
// A document stands for its content. An alias is read as the node it names only by
// a Reader with Aliases set; the package-level functions leave it an alias, which
// reads as absent. A nil node, or one of the wrong kind, reads as absent (nil, "" or
// no items). Get takes the first of duplicate keys, and Pairs lists every entry in
// order. A merge key ("<<: *base") is an ordinary key except through Merged.
package yamlnode

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// Reader reads node trees; the zero Reader is what the package-level functions use.
type Reader struct {
	// Aliases makes Mapping, Get, Pairs, Items and List follow aliases. Scalar and
	// Text never do on the node they are handed, only on the values of keys.
	Aliases bool
}

// Pair is one entry of a mapping. Key is as written.
type Pair struct{ Key, Value *yaml.Node }

// Parse is the top node of source's document; nil when source is not YAML or is
// empty.
func Parse(source []byte) *yaml.Node {
	var document yaml.Node
	if yaml.Unmarshal(source, &document) != nil || document.Kind != yaml.DocumentNode {
		return nil
	}
	return Reader{}.Resolve(&document)
}

// Resolve unwraps documents, and aliases when r.Aliases is set.
func (r Reader) Resolve(n *yaml.Node) *yaml.Node {
	for n != nil && (n.Kind == yaml.DocumentNode || n.Kind == yaml.AliasNode && r.Aliases) {
		if n.Kind == yaml.AliasNode {
			n = n.Alias
		} else if len(n.Content) > 0 {
			n = n.Content[0]
		} else {
			return nil
		}
	}
	return n
}

func (r Reader) Mapping(n *yaml.Node) *yaml.Node {
	if n = r.Resolve(n); n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	return n
}

// Get follows keys through nested mappings.
func (r Reader) Get(n *yaml.Node, keys ...string) *yaml.Node {
	n = r.Resolve(n)
	for _, key := range keys {
		m := r.Mapping(n)
		if m == nil {
			return nil
		}
		n = nil
		for i := 0; i+1 < len(m.Content); i += 2 {
			if m.Content[i].Value == key {
				n = r.Resolve(m.Content[i+1])
				break
			}
		}
	}
	return n
}

func (r Reader) Pairs(n *yaml.Node) []Pair {
	m := r.Mapping(n)
	if m == nil {
		return nil
	}
	out := make([]Pair, 0, len(m.Content)/2)
	for i := 0; i+1 < len(m.Content); i += 2 {
		out = append(out, Pair{m.Content[i], r.Resolve(m.Content[i+1])})
	}
	return out
}

// Items lists a sequence's elements.
func (r Reader) Items(n *yaml.Node) []*yaml.Node {
	if n = r.Resolve(n); n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]*yaml.Node, 0, len(n.Content))
	for _, item := range n.Content {
		out = append(out, r.Resolve(item))
	}
	return out
}

// List is Items, except that a lone value counts as a sequence of one.
func (r Reader) List(n *yaml.Node) []*yaml.Node {
	if n = r.Resolve(n); n == nil || n.Kind == yaml.SequenceNode {
		return r.Items(n)
	}
	return []*yaml.Node{n}
}

// Scalars are a scalar itself, or a sequence's scalar elements.
func (r Reader) Scalars(n *yaml.Node) []*yaml.Node {
	var out []*yaml.Node
	for _, item := range r.List(n) {
		if item.Kind == yaml.ScalarNode {
			out = append(out, item)
		}
	}
	return out
}

// Scalar is the value of the scalar at keys below n, as written ("~" for a null
// written so).
func (r Reader) Scalar(n *yaml.Node, keys ...string) string {
	if len(keys) > 0 {
		n = r.Get(n, keys...)
	}
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

// Text is Scalar without surrounding white space.
func (r Reader) Text(n *yaml.Node, keys ...string) string {
	return strings.TrimSpace(r.Scalar(n, keys...))
}

// Merged is the mapping n with its merge keys applied: the entries of the mappings
// a merge key names follow the mapping's own, each only where no earlier entry sets
// its key.
func (r Reader) Merged(n *yaml.Node) *yaml.Node {
	m := r.Mapping(n)
	if m == nil {
		return nil
	}
	var own, merged []*yaml.Node
	found := false
	for i := 0; i+1 < len(m.Content); i += 2 {
		key, value := m.Content[i], m.Content[i+1]
		if key.Value != "<<" || key.Tag != "!!merge" {
			own = append(own, key, value)
			continue
		}
		found = true
		for _, source := range r.List(value) {
			if source := r.Merged(source); source != nil {
				merged = append(merged, source.Content...)
			}
		}
	}
	if !found {
		return m
	}
	seen := map[string]bool{}
	for i := 0; i < len(own); i += 2 {
		seen[own[i].Value] = true
	}
	for i := 0; i+1 < len(merged); i += 2 {
		if !seen[merged[i].Value] {
			seen[merged[i].Value] = true
			own = append(own, merged[i], merged[i+1])
		}
	}
	result := *m
	result.Content = own
	return &result
}

func Mapping(n *yaml.Node) *yaml.Node             { return Reader{}.Mapping(n) }
func Get(n *yaml.Node, keys ...string) *yaml.Node { return Reader{}.Get(n, keys...) }
func Pairs(n *yaml.Node) []Pair                   { return Reader{}.Pairs(n) }
func Items(n *yaml.Node) []*yaml.Node             { return Reader{}.Items(n) }
func List(n *yaml.Node) []*yaml.Node              { return Reader{}.List(n) }
func Scalars(n *yaml.Node) []*yaml.Node           { return Reader{}.Scalars(n) }
func Scalar(n *yaml.Node, keys ...string) string  { return Reader{}.Scalar(n, keys...) }
func Text(n *yaml.Node, keys ...string) string    { return Reader{}.Text(n, keys...) }

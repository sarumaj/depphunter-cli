// Package yamlnode reads gopkg.in/yaml.v3 node trees, which the manifest readers use
// instead of decoding into structs so that every value keeps its line.
//
// Every helper unwraps a node the same way before looking at it: a document stands
// for its content and an alias for the node it names. A nil node, or one of the
// wrong kind, reads as absent (nil, "" or no items). Get takes the first of
// duplicate keys, and Pairs lists every entry in order. A merge key ("<<: *base")
// is an ordinary key except through Merged.
//
// A manifest is untrusted input, and anchors let it point back into itself. yaml.v3
// bounds alias expansion only when it decodes into Go values, not into a Node, so the
// walks here bound themselves: Resolve stops after a few hops, Merged merges each
// mapping at most once per call and reads one it is still merging as empty, and no
// other helper descends more than one level. A caller that recurses through values
// must remember the nodes it has visited.
package yamlnode

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// Pair is one entry of a mapping. Value is unwrapped, Key is as written.
type Pair struct{ Key, Value *yaml.Node }

// Parse is the top node of source's document; nil when source is not YAML or is
// empty.
func Parse(source []byte) *yaml.Node {
	var document yaml.Node
	if yaml.Unmarshal(source, &document) != nil || document.Kind != yaml.DocumentNode {
		return nil
	}
	return Resolve(&document)
}

// maxHops bounds Resolve. A parsed tree needs two (a document, then an alias, whose
// target is never another alias); only a node built by hand can hold a chain.
const maxHops = 8

// Resolve unwraps documents and aliases; nil for a chain longer than any parse makes.
//
// Implements: REQ-LANG-032
func Resolve(n *yaml.Node) *yaml.Node {
	for hops := 0; n != nil && (n.Kind == yaml.DocumentNode || n.Kind == yaml.AliasNode); hops++ {
		switch {
		case hops == maxHops:
			return nil
		case n.Kind == yaml.AliasNode:
			n = n.Alias
		case len(n.Content) > 0:
			n = n.Content[0]
		default:
			return nil
		}
	}
	return n
}

func Mapping(n *yaml.Node) *yaml.Node {
	if n = Resolve(n); n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	return n
}

// Get follows keys through nested mappings.
func Get(n *yaml.Node, keys ...string) *yaml.Node {
	n = Resolve(n)
	for _, key := range keys {
		m := Mapping(n)
		if m == nil {
			return nil
		}
		n = nil
		for i := 0; i+1 < len(m.Content); i += 2 {
			if m.Content[i].Value == key {
				n = Resolve(m.Content[i+1])
				break
			}
		}
	}
	return n
}

func Pairs(n *yaml.Node) []Pair {
	m := Mapping(n)
	if m == nil {
		return nil
	}
	out := make([]Pair, 0, len(m.Content)/2)
	for i := 0; i+1 < len(m.Content); i += 2 {
		if value := Resolve(m.Content[i+1]); value != nil {
			out = append(out, Pair{m.Content[i], value})
		}
	}
	return out
}

// Items lists a sequence's elements.
func Items(n *yaml.Node) []*yaml.Node {
	if n = Resolve(n); n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]*yaml.Node, 0, len(n.Content))
	for _, item := range n.Content {
		if item = Resolve(item); item != nil {
			out = append(out, item)
		}
	}
	return out
}

// List is Items, except that a lone value counts as a sequence of one.
func List(n *yaml.Node) []*yaml.Node {
	if n = Resolve(n); n == nil || n.Kind == yaml.SequenceNode {
		return Items(n)
	}
	return []*yaml.Node{n}
}

// Scalars are a scalar itself, or a sequence's scalar elements.
func Scalars(n *yaml.Node) []*yaml.Node {
	var out []*yaml.Node
	for _, item := range List(n) {
		if item.Kind == yaml.ScalarNode {
			out = append(out, item)
		}
	}
	return out
}

// Scalar is the value of the scalar at keys below n, as written ("~" for a null
// written so).
func Scalar(n *yaml.Node, keys ...string) string {
	if n = Get(n, keys...); n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

// Text is Scalar without surrounding white space.
func Text(n *yaml.Node, keys ...string) string {
	return strings.TrimSpace(Scalar(n, keys...))
}

// Merged is the mapping n with its merge keys applied: the entries of the mappings
// a merge key names follow the mapping's own, each only where no earlier entry sets
// its key.
//
// Implements: REQ-LANG-032
func Merged(n *yaml.Node) *yaml.Node {
	return merged(n, map[*yaml.Node]*yaml.Node{})
}

// merged records each mapping's result in done, and nil while the mapping is being
// merged: a mapping that merges itself, directly or through others, gets nothing
// from that merge, and one merged many times over is merged once.
func merged(n *yaml.Node, done map[*yaml.Node]*yaml.Node) *yaml.Node {
	m := Mapping(n)
	if m == nil {
		return nil
	}
	if result, ok := done[m]; ok {
		return result
	}
	done[m] = nil
	var own, inherited []*yaml.Node
	found := false
	for i := 0; i+1 < len(m.Content); i += 2 {
		key, value := m.Content[i], m.Content[i+1]
		if key.Value != "<<" || key.Tag != "!!merge" {
			own = append(own, key, value)
			continue
		}
		found = true
		for _, source := range List(value) {
			if source := merged(source, done); source != nil {
				inherited = append(inherited, source.Content...)
			}
		}
	}
	result := m
	if found {
		seen := map[string]bool{}
		for i := 0; i < len(own); i += 2 {
			seen[own[i].Value] = true
		}
		for i := 0; i+1 < len(inherited); i += 2 {
			if !seen[inherited[i].Value] {
				seen[inherited[i].Value] = true
				own = append(own, inherited[i], inherited[i+1])
			}
		}
		copied := *m
		copied.Content = own
		result = &copied
	}
	done[m] = result
	return result
}

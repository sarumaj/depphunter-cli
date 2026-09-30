package yamlnode

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// values renders nodes for comparison: a scalar as its value, a mapping as "{}", a
// sequence as "[]" and nil as "nil".
func values(nodes ...*yaml.Node) []string {
	out := []string{}
	for _, n := range nodes {
		switch {
		case n == nil:
			out = append(out, "nil")
		case n.Kind == yaml.MappingNode:
			out = append(out, "{}")
		case n.Kind == yaml.SequenceNode:
			out = append(out, "[]")
		default:
			out = append(out, n.Value)
		}
	}
	return out
}

func keys(pairs []Pair) []string {
	out := []string{}
	for _, p := range pairs {
		out = append(out, p.Key.Value+"="+values(p.Value)[0])
	}
	return out
}

// Verifies: REQ-LANG-032
func TestHelpers(t *testing.T) {
	const anchors = `
base: &base {image: alpine, tag: " 1.0 "}
list: &list [a, *base]
name: &name x
alias: *base
aliasList: *list
aliasName: *name
null:
tilde: ~
tagged: !!str 1.0
custom: !Ref value
duplicate: first
duplicate: second
merged:
  <<: *base
  image: debian
mergedList:
  <<: [*base, {extra: y, image: z}]
quotedMerge:
  "<<": *base
lone: one
`
	root := Parse([]byte(anchors))
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"document unwraps to its mapping", Mapping(root) == root && root.Kind == yaml.MappingNode, true},
		{"empty source", Parse(nil) == nil, true},
		{"not YAML", Parse([]byte("a: [")) == nil, true},
		{"document node", values(Get(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}, "name")), []string{"x"}},
		{"empty document node", Mapping(&yaml.Node{Kind: yaml.DocumentNode}) == nil, true},
		{"nil", values(Get(nil, "a"), Mapping(nil)), []string{"nil", "nil"}},
		{"scalar is no mapping", values(Get(root, "name", "x")), []string{"nil"}},
		{"path", Scalar(root, "alias", "image"), "alpine"},
		{"alias to mapping", keys(Pairs(Get(root, "alias"))), []string{"image=alpine", "tag= 1.0 "}},
		{"alias to scalar", Scalar(root, "aliasName"), "x"},
		{"Text of a raw alias", Text(root.Content[11]), "x"},
		{"alias to sequence", values(Items(Get(root, "aliasList"))...), []string{"a", "{}"}},
		{"alias item", values(Items(Get(root, "list"))...), []string{"a", "{}"}},
		{"null", []string{Scalar(root, "null"), Scalar(root, "tilde"), Get(root, "null").Tag}, []string{"", "~", "!!null"}},
		{"tagged scalars", []string{Scalar(root, "tagged"), Scalar(root, "custom")}, []string{"1.0", "value"}},
		{"Text trims, Scalar does not", []string{Text(root, "base", "tag"), Scalar(root, "base", "tag")}, []string{"1.0", " 1.0 "}},
		{"Text of a mapping", Text(root, "base"), ""},
		{"first duplicate wins", Scalar(root, "duplicate"), "first"},
		{"pairs keep duplicates", keys(Pairs(root))[10:12], []string{"duplicate=first", "duplicate=second"}},
		{"merge key is an ordinary key", keys(Pairs(Get(root, "merged"))), []string{"<<={}", "image=debian"}},
		{"merged", keys(Pairs(Merged(Get(root, "merged")))), []string{"image=debian", "tag= 1.0 "}},
		{"merged list", keys(Pairs(Merged(Get(root, "mergedList")))), []string{"image=alpine", "tag= 1.0 ", "extra=y"}},
		{"quoted << is no merge key", keys(Pairs(Merged(Get(root, "quotedMerge")))), []string{"<<={}"}},
		{"merged without merge keys", Merged(Get(root, "base")) == Get(root, "base"), true},
		{"merged scalar", Merged(Get(root, "name")) == nil, true},
		{"items of a scalar", values(Items(Get(root, "lone"))...), []string{}},
		{"list of a scalar", values(List(Get(root, "lone"))...), []string{"one"}},
		{"list of nil", values(List(nil)...), []string{}},
		{"scalars of a scalar", values(Scalars(Get(root, "lone"))...), []string{"one"}},
		{"scalars of a sequence", values(Scalars(Get(root, "list"))...), []string{"a"}},
		{"scalars of a mapping", values(Scalars(Get(root, "base"))...), []string{}},
		{"line of the value", Get(root, "alias").Line, 2},
	}
	for _, test := range tests {
		if !reflect.DeepEqual(test.got, test.want) {
			t.Errorf("%s: got %#v, want %#v", test.name, test.got, test.want)
		}
	}
}

// Anchors that point back into themselves: every walk ends, and what leads back to
// where it started reads as absent.
//
// Verifies: REQ-LANG-032
func TestCycles(t *testing.T) {
	const cycles = `
selfMerge: &self
  <<: *self
  image: nginx
first: &first
  <<: &second
    <<: *first
    tag: "2"
  image: alpine
selfSequence: &sequence [a, *sequence]
selfMapping: &mapping {name: b, again: *mapping}
`
	root := Parse([]byte(cycles))
	chain := &yaml.Node{Kind: yaml.AliasNode}
	chain.Alias = &yaml.Node{Kind: yaml.AliasNode, Alias: chain}
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"self-referencing merge key", keys(Pairs(Merged(Get(root, "selfMerge")))), []string{"image=nginx"}},
		{"merge cycle through two anchors", keys(Pairs(Merged(Get(root, "first")))), []string{"image=alpine", "tag=2"}},
		{"merge cycle from the other anchor", keys(Pairs(Merged(Get(root, "first", "<<")))), []string{"tag=2", "image=alpine"}},
		{"sequence holding itself", values(Items(Get(root, "selfSequence"))...), []string{"a", "[]"}},
		{"scalars of a sequence holding itself", values(Scalars(Get(root, "selfSequence"))...), []string{"a"}},
		{"mapping holding itself", Scalar(root, "selfMapping", "again", "again", "again", "name"), "b"},
		{"alias chain built by hand", Resolve(chain) == nil, true},
		{"items of a hand-built chain", len(Items(&yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{chain}})), 0},
	}
	for _, test := range tests {
		if !reflect.DeepEqual(test.got, test.want) {
			t.Errorf("%s: got %#v, want %#v", test.name, test.got, test.want)
		}
	}
}

// Aliases nested so that their expansion has 10^9 nodes, and merge keys nested so
// that merging each source anew would take 2^40 steps, are read at once.
//
// Verifies: REQ-LANG-032
func TestBillionLaughs(t *testing.T) {
	var laughs, merges strings.Builder
	laughs.WriteString("l0: &l0 [lol, lol, lol, lol, lol, lol, lol, lol, lol, lol]\n")
	merges.WriteString("m0: &m0 {key: value}\n")
	for i := 1; i <= 40; i++ {
		if i <= 9 {
			fmt.Fprintf(&laughs, "l%d: &l%d [%s]\n", i, i, strings.TrimSuffix(strings.Repeat(fmt.Sprintf("*l%d, ", i-1), 10), ", "))
		}
		fmt.Fprintf(&merges, "m%d: &m%d {<<: [*m%d, *m%d], key%d: value}\n", i, i, i-1, i-1, i)
	}
	start := time.Now()
	root := Parse([]byte(laughs.String()))
	count := 0
	for _, item := range Items(Get(root, "l9")) {
		count += len(Scalars(item)) + len(Items(item))
	}
	merged := Merged(Get(Parse([]byte(merges.String())), "m40"))
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("took %v", elapsed)
	}
	if count != 100 || merged == nil || len(merged.Content) != 2*41 {
		t.Errorf("count %d, merged %v", count, merged)
	}
}

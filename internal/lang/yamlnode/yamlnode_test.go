package yamlnode

import (
	"reflect"
	"testing"

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
	aliased := Reader{Aliases: true}
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"document unwraps to its mapping", Mapping(root) == root && root.Kind == yaml.MappingNode, true},
		{"empty source", Parse(nil) == nil, true},
		{"not YAML", Parse([]byte("a: [")) == nil, true},
		{"document node", values(aliased.Get(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}, "name")), []string{"x"}},
		{"empty document node", Mapping(&yaml.Node{Kind: yaml.DocumentNode}) == nil, true},
		{"nil", values(Get(nil, "a"), Mapping(nil)), []string{"nil", "nil"}},
		{"scalar is no mapping", values(Get(root, "name", "x")), []string{"nil"}},
		{"path", aliased.Scalar(root, "alias", "image"), "alpine"},
		{"plain reader leaves an alias", values(Get(root, "alias"), Get(root, "alias", "image")), []string{"base", "nil"}},
		{"alias to mapping", keys(aliased.Pairs(aliased.Get(root, "alias"))), []string{"image=alpine", "tag= 1.0 "}},
		{"alias to scalar", aliased.Scalar(root, "aliasName"), "x"},
		{"plain scalar of an alias", Scalar(root, "aliasName"), ""},
		{"Text of a raw alias", aliased.Text(Get(root, "aliasName")), ""},
		{"alias to sequence", values(aliased.Items(aliased.Get(root, "aliasList"))...), []string{"a", "{}"}},
		{"alias item", values(aliased.Items(Get(root, "list"))...), []string{"a", "{}"}},
		{"plain alias item", values(Items(Get(root, "list"))...), []string{"a", "base"}},
		{"null", []string{Scalar(root, "null"), Scalar(root, "tilde"), Get(root, "null").Tag}, []string{"", "~", "!!null"}},
		{"tagged scalars", []string{Scalar(root, "tagged"), Scalar(root, "custom")}, []string{"1.0", "value"}},
		{"Text trims, Scalar does not", []string{Text(root, "base", "tag"), Scalar(root, "base", "tag")}, []string{"1.0", " 1.0 "}},
		{"Text of a mapping", Text(root, "base"), ""},
		{"first duplicate wins", Scalar(root, "duplicate"), "first"},
		{"pairs keep duplicates", keys(Pairs(root))[10:12], []string{"duplicate=first", "duplicate=second"}},
		{"merge key is an ordinary key", keys(aliased.Pairs(Get(root, "merged"))), []string{"<<={}", "image=debian"}},
		{"merged", keys(aliased.Pairs(aliased.Merged(Get(root, "merged")))), []string{"image=debian", "tag= 1.0 "}},
		{"merged list", keys(aliased.Pairs(aliased.Merged(Get(root, "mergedList")))), []string{"image=alpine", "tag= 1.0 ", "extra=y"}},
		{"quoted << is no merge key", keys(aliased.Pairs(aliased.Merged(Get(root, "quotedMerge")))), []string{"<<={}"}},
		{"merged without merge keys", aliased.Merged(Get(root, "base")) == Get(root, "base"), true},
		{"merged scalar", aliased.Merged(Get(root, "name")) == nil, true},
		{"items of a scalar", values(Items(Get(root, "lone"))...), []string{}},
		{"list of a scalar", values(List(Get(root, "lone"))...), []string{"one"}},
		{"list of nil", values(List(nil)...), []string{}},
		{"scalars of a scalar", values(Scalars(Get(root, "lone"))...), []string{"one"}},
		{"scalars of a sequence", values(Scalars(Get(root, "list"))...), []string{"a"}},
		{"scalars of a mapping", values(Scalars(Get(root, "base"))...), []string{}},
		{"line of the value", aliased.Get(root, "alias").Line, 2},
	}
	for _, test := range tests {
		if !reflect.DeepEqual(test.got, test.want) {
			t.Errorf("%s: got %#v, want %#v", test.name, test.got, test.want)
		}
	}
}

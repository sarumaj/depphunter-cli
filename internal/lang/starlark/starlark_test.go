package starlark

import (
	"reflect"
	"strings"
	"testing"
)

// Verifies: REQ-BAZEL-012
func TestStrings(t *testing.T) {
	source := "a = \"x\\\"y\"\nb = 'it''s'\nc = r\"\\d+\"\nd = \"\"\"multi\nline # not a comment\"\"\"\ne = rb'''raw'''  # comment\nf = \"unterminated\ng = \"ok\"\n"
	want := map[string]string{"a": `x"y`, "b": "its", "c": `\d+`, "d": "multi\nline # not a comment", "e": "raw", "f": "unterminated", "g": "ok"}
	got := map[string]string{}
	for _, statement := range Parse([]byte(source)).Statements {
		if statement.Kind == 'a' {
			got[statement.Targets[0]], _ = statement.X.StringValue()
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Verifies: REQ-BAZEL-012
func TestStatements(t *testing.T) {
	source := `load("//a:b.bzl", "x", y = "z")

# a comment
cc_library(
    name = "lib",  # trailing
    srcs = glob(["*.cc"], exclude = ["x.cc"]) + [":gen"],
    deps = select({"//c:linux": [":l"], "//conditions:default": []}),
    **kwargs
)

X, Y = 1, \
    2

def macro(name, **kwargs):
    """Doc."""
    if name:
        native.genrule(name = name, cmd = "echo {}".format(name))
    return [x for x in kwargs]

def inline(): return 1

after = struct(a = [1, 2,], b = {"k": "v"}, c = -1, d = not True, e = a if b else c, f = lambda x: x)
`
	f := Parse([]byte(source))
	var got []string
	for _, statement := range f.Statements {
		got = append(got, string(statement.Kind)+":"+statement.Definition+":"+statement.Name+":"+strings.Join(statement.Targets, ",")+":"+statement.X.Callee())
	}
	want := []string{
		"e:::" + ":load", "e::::cc_library", "a:::X,Y:", "d::macro::", "e:macro:::", "o:macro:::",
		"e:macro:::native.genrule", "o:macro:::", "d::inline::", "o:inline:::", "a:::after:struct",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("statements\n got %q\nwant %q", got, want)
	}
	library := f.Statements[1].X
	if s := library.KeywordString("name"); s != "lib" {
		t.Errorf("name = %q", s)
	}
	sources := library.Keyword("srcs")
	if sources.Kind != Binary || sources.Text != "+" || sources.Items[0].Callee() != "glob" ||
		!reflect.DeepEqual(sources.Items[0].Keyword("exclude").Strings(), []string{"x.cc"}) ||
		!reflect.DeepEqual(sources.Items[1].Strings(), []string{":gen"}) {
		t.Errorf("srcs read as %+v", sources)
	}
	dependency := library.Keyword("deps").Position(0)
	if dependency.Kind != Dictionary || len(dependency.Items) != 4 || !reflect.DeepEqual(dependency.Items[1].Strings(), []string{":l"}) {
		t.Errorf("select read as %+v", dependency)
	}
	if library.Arguments[len(library.Arguments)-1].Star != "**" {
		t.Error("**kwargs not read")
	}
	after := f.Statements[len(f.Statements)-1].X
	if a := after.Keyword("a"); a.Kind != List || len(a.Items) != 2 {
		t.Errorf("a = %+v", a)
	}
	if e := after.Keyword("e"); e.Kind != Condition {
		t.Errorf("e = %+v", e)
	}
	if l := f.Statements[0].X; l.Position(0).Text != "//a:b.bzl" || l.Arguments[2].Name != "y" {
		t.Errorf("load read as %+v", l)
	}
}

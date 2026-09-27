package starlark

import (
	"reflect"
	"strings"
	"testing"
)

// Verifies: REQ-BAZEL-012
func TestStrings(t *testing.T) {
	src := "a = \"x\\\"y\"\nb = 'it''s'\nc = r\"\\d+\"\nd = \"\"\"multi\nline # not a comment\"\"\"\ne = rb'''raw'''  # comment\nf = \"unterminated\ng = \"ok\"\n"
	want := map[string]string{"a": `x"y`, "b": "its", "c": `\d+`, "d": "multi\nline # not a comment", "e": "raw", "f": "unterminated", "g": "ok"}
	got := map[string]string{}
	for _, st := range Parse([]byte(src)).Stmts {
		if st.Kind == 'a' {
			got[st.Targets[0]], _ = st.X.Str()
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Verifies: REQ-BAZEL-012
func TestStatements(t *testing.T) {
	src := `load("//a:b.bzl", "x", y = "z")

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
	f := Parse([]byte(src))
	var got []string
	for _, st := range f.Stmts {
		got = append(got, string(st.Kind)+":"+st.Def+":"+st.Name+":"+strings.Join(st.Targets, ",")+":"+st.X.Callee())
	}
	want := []string{
		"e:::" + ":load", "e::::cc_library", "a:::X,Y:", "d::macro::", "e:macro:::", "o:macro:::",
		"e:macro:::native.genrule", "o:macro:::", "d::inline::", "o:inline:::", "a:::after:struct",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("statements\n got %q\nwant %q", got, want)
	}
	lib := f.Stmts[1].X
	if s := lib.KwStr("name"); s != "lib" {
		t.Errorf("name = %q", s)
	}
	srcs := lib.Kw("srcs")
	if srcs.Kind != Binary || srcs.Text != "+" || srcs.Items[0].Callee() != "glob" ||
		!reflect.DeepEqual(srcs.Items[0].Kw("exclude").Strings(), []string{"x.cc"}) ||
		!reflect.DeepEqual(srcs.Items[1].Strings(), []string{":gen"}) {
		t.Errorf("srcs read as %+v", srcs)
	}
	sel := lib.Kw("deps").Pos(0)
	if sel.Kind != Dict || len(sel.Items) != 4 || !reflect.DeepEqual(sel.Items[1].Strings(), []string{":l"}) {
		t.Errorf("select read as %+v", sel)
	}
	if lib.Args[len(lib.Args)-1].Star != "**" {
		t.Error("**kwargs not read")
	}
	after := f.Stmts[len(f.Stmts)-1].X
	if a := after.Kw("a"); a.Kind != List || len(a.Items) != 2 {
		t.Errorf("a = %+v", a)
	}
	if e := after.Kw("e"); e.Kind != Cond {
		t.Errorf("e = %+v", e)
	}
	if l := f.Stmts[0].X; l.Pos(0).Text != "//a:b.bzl" || l.Args[2].Name != "y" {
		t.Errorf("load read as %+v", l)
	}
}

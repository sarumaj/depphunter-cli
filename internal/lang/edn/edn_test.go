package edn

import (
	"strings"
	"testing"
)

func texts(nodes []*Node) string {
	var parts []string
	for _, n := range nodes {
		parts = append(parts, Render(n, 200))
	}
	return strings.Join(parts, " ")
}

// Verifies: REQ-CLOJURE-011
func TestReader(t *testing.T) {
	for src, want := range map[string]string{
		`(a b) [1 2] {:k "v"} #{x}`:     `(a b) [1 2] {:k "v"} #{x}`,
		`(a #_ b c)`:                    `(a c)`,
		`(a #_ #_ b c d)`:               `(a d)`,
		`(a #_(b (c)) d)`:               `(a d)`,
		`(defn ^:private ^String f [])`: `(defn f [])`,
		`(def ^{:doc "x"} y 1)`:         `(def y 1)`,
		`'x '[a b]`:                     `(quote x) (quote [a b])`,
		"`(a ~b ~@c)":                   `(syntax-quote (a (unquote b) (unquote-splicing c)))`,
		`@a #'b`:                        `(deref a) (var b)`,
		`(r #?(:clj a :cljs b))`:        `(r a b)`,
		`(r #?@(:clj [a b] :cljs [c]))`: `(r a b c)`,
		`(r #_ #?(:clj a :cljs b) c)`:   `(r c)`,
		`(r #?(:cljs b))`:               `(r b)`,
		`#:acme{:a 1 :b/c 2}`:           `{:acme/a 1 :b/c 2}`,
		`#::{:a 1}`:                     `{:a 1}`,
		`[\a \( \) \newline \u0041 \"]`: `[\a \( \) \newline \u0041 \"]`,
		`["a\"b" "c\\" "(" ";"]`:        `["a"b" "c\" "(" ";"]`,
		`#"[(]" #inst "2024" ##Inf`:     `#"[(]" "2024" ##Inf`,
		`#(inc %) #=(+ 1 2)`:            `(inc %) (+ 1 2)`,
		"a ; comment (\nb":              `a b`,
		"#!/usr/bin/env bb\n(x)":        `(x)`,
		`(a]) b`:                        `(a) b`, // a stray closer is ignored
		`(a [b) c`:                      `(a [b]) c`,
		`(a (b`:                         `(a (b))`, // input ends inside forms
		`-1 +2 -x 1.5e3 3/4 0x1F`:       `-1 +2 -x 1.5e3 3/4 0x1F`,
		`{:a 1, :b 2}`:                  `{:a 1 :b 2}`,
		`(a b#)`:                        `(a b#)`,
	} {
		if got := texts(Read([]byte(src))); got != want {
			t.Errorf("Read(%q) = %s, want %s", src, got, want)
		}
	}
	nodes := Read([]byte("(a)\n\n(b\n c)"))
	if nodes[0].Line != 1 || nodes[1].Line != 3 || nodes[1].Kids[1].Line != 4 {
		t.Errorf("lines %d %d %d", nodes[0].Line, nodes[1].Line, nodes[1].Kids[1].Line)
	}
	kinds := Read([]byte(`a :k "s" 1 \c #"r" [] () {} #{}`))
	for i, want := range []Kind{Symbol, Keyword, String, Number, Char, Regex, Vector, List, Map, Set} {
		if kinds[i].Kind != want {
			t.Errorf("form %d kind %d, want %d", i, kinds[i].Kind, want)
		}
	}
	if tagged := Read([]byte(`#inst "x"`)); tagged[0].Tag != "inst" {
		t.Errorf("tag %q", tagged[0].Tag)
	}
	n := 0
	ReadTop([]byte("(a) (b) (c)"), func(*Node) bool { n++; return n < 2 })
	if n != 2 {
		t.Errorf("ReadTop went on after false: %d forms", n)
	}
	m := Read([]byte(`{:paths ["src" "test" x] :deps {}}`))[0]
	if got := m.Get("paths").Strings(); len(got) != 2 || got[1] != "test" {
		t.Errorf("paths %v", got)
	}
	if Unquote(Read([]byte(`'[a]`))[0]).Kind != Vector || !Unquoted(Read([]byte(`~x`))[0]) {
		t.Error("Unquote/Unquoted")
	}
}

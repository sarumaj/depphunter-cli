package haskell

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// An extra-dep's commit given by an anchor is read as the commit.
//
// Verifies: REQ-LANG-032
func TestStackAnchors(t *testing.T) {
	p := readStack([]byte("resolver: lts-22.0\nx-commit: &commit 0123abc\nextra-deps:\n  - git: https://github.com/acme/pay.git\n    commit: *commit\n"))
	if p == nil || len(p.extras) != 1 || p.extras[0].version != "0123abc" {
		t.Errorf("extras %+v", p)
	}
}

// Anchors that make a when: contain itself, directly or through another anchor,
// end the walk; a self-referencing merge key is an ordinary key here.
//
// Verifies: REQ-LANG-032
func TestHpackCycles(t *testing.T) {
	const hpack = `name: app
<<: &top {name: other}
dependencies: [base]
library:
  source-dirs: src
  when: &self
    condition: flag(dev)
    dependencies: [text]
    then: {when: *self}
executables:
  app:
    main: Main.hs
    when: &first
      condition: os(linux)
      then: &second
        dependencies: [unix]
        when: [*first, *second]
tests:
  spec:
    self: &loop [*loop]
    dependencies: *loop
`
	p := readHpack([]byte(hpack), "package.yaml")
	if p == nil || p.name != "app" || len(p.comps) != 3 {
		t.Fatalf("readHpack: %+v", p)
	}
	names := func(c *component) string {
		var out []string
		for _, d := range c.dependencies {
			out = append(out, d.name)
		}
		return strings.Join(out, " ")
	}
	for i, want := range []string{"text base", "unix base", "base"} {
		if got := names(p.comps[i]); got != want {
			t.Errorf("%s: dependencies %q, want %q", p.comps[i].kind, got, want)
		}
	}
}

// A when: whose nested aliases would expand to 2^30 nodes is read at once.
//
// Verifies: REQ-LANG-032
func TestHpackLaughs(t *testing.T) {
	var source strings.Builder
	source.WriteString("name: app\nx0: &w0 {dependencies: [base]}\n")
	for i := 1; i <= 30; i++ {
		source.WriteString("x" + strconv.Itoa(i) + ": &w" + strconv.Itoa(i) + " {when: [*w" + strconv.Itoa(i-1) + ", *w" + strconv.Itoa(i-1) + "]}\n")
	}
	source.WriteString("library:\n  when: *w30\n")
	start := time.Now()
	p := readHpack([]byte(source.String()), "package.yaml")
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("took %v", elapsed)
	}
	if p == nil || len(p.comps) != 1 || len(p.comps[0].dependencies) != 1 {
		t.Errorf("readHpack: %+v", p)
	}
}

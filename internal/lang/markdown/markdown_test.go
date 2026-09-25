package markdown

import (
	"os"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// testdata/repo is a small documented project: a README linking to a file, a heading
// in it, a directory, a source file and itself, with the forms that are not paths -
// a URL, a mail address, a fragment - alongside, and a fenced block and a code span
// holding links that must not be followed.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-MD-001, REQ-MD-003, REQ-MD-004, REQ-MD-005
func TestLinksBecomeDependencies(t *testing.T) {
	got := langtest.Imports(t, analyze(t)["README.md"])
	for spec, want := range map[string]lang.Target{
		"[spec](docs/SPEC.md)":                       {Local: "docs/SPEC.md"},
		"[section of it](docs/SPEC.md#how-it-works)": {Local: "docs/SPEC.md"},
		"[directory](src)":                           {Local: "src"},
		"[source file](src/main.go)":                 {Local: "src/main.go"},
		// Rooted at the repository, which is what a renderer of a repository's
		// Markdown means by it.
		"[from the root](/docs/SPEC.md)": {Local: "docs/SPEC.md"},
		// Raw HTML carries a repository's badges, and a tag is as much a link as a
		// bracket is.
		`<img src="docs/badge.svg"`: {Local: "docs/badge.svg"},
		`<a href="docs/SPEC.md"`:    {Local: "docs/SPEC.md"},
		// A reference's definition is where the destination is written.
		"[spec]: docs/SPEC.md":     {Local: "docs/SPEC.md"},
		"[collapsed]: src/main.go": {Local: "src/main.go"},
	} {
		if g, ok := got[spec]; !ok {
			t.Errorf("%s: not captured", spec)
		} else if g != want {
			t.Errorf("%s: got %+v, want %+v", spec, g, want)
		}
	}
	// What is not a dependency of this repository resolves to nothing: a URL, a mail
	// address, a fragment of this same file, and a path that climbs out.
	for _, spec := range []string{
		"<https://example.test/page>", "[mail](mailto:nobody@example.test)",
		"[this page](#install)", "[out](../elsewhere.md)",
	} {
		if g, ok := got[spec]; !ok {
			t.Errorf("%s: not captured at all", spec)
		} else if g != (lang.Target{}) {
			t.Errorf("%s: resolved to %+v, want nothing", spec, g)
		}
	}
	for spec := range got {
		if strings.Contains(spec, "NOT-THERE") {
			t.Errorf("a link inside code was followed: %s", spec)
		}
	}
}

// Verifies: REQ-MD-002
func TestHeadingsBecomeSymbols(t *testing.T) {
	got := langtest.Symbols(t, analyze(t)["README.md"])
	for name, kind := range map[string]string{
		"The Project":    "title",
		"Install":        "heading 2",
		"Setext heading": "heading 2",
		// A document may say the same thing twice, and a symbol is named within its
		// file, so the repeat is numbered the way a renderer numbers its anchor.
		"Install (2)": "heading 2",
	} {
		if g, ok := got[name]; !ok {
			t.Errorf("%q: not a symbol (have %v)", name, got)
		} else if g != kind {
			t.Errorf("%q: kind %q, want %q", name, g, kind)
		}
	}
}

// Verifies: REQ-MD-008
func TestAnchors(t *testing.T) {
	src, err := os.ReadFile("testdata/repo/docs/SPEC.md")
	if err != nil {
		t.Fatal(err)
	}
	got := Anchors(src)
	for _, want := range []string{"spec", "how-it-works", "custom", "the-custom-one", "html-anchor"} {
		if !got[want] {
			t.Errorf("#%s is not an anchor of the spec (have %v)", want, keys(got))
		}
	}
	if got["nowhere"] {
		t.Error("a name nothing declares is an anchor")
	}
}

// Verifies: REQ-MD-008
func TestSlug(t *testing.T) {
	for _, c := range []struct{ heading, want string }{
		{"How it works", "how-it-works"},
		{"Package indexes", "package-indexes"},
		// The formatting comes off and the text of a link stays, because that is
		// what a reader sees and what a renderer slugs.
		{"`--online` and the rest", "--online-and-the-rest"},
		{"**Bold** and _thin_", "bold-and-thin"},
		{"See [the spec](docs/SPEC.md)", "see-the-spec"},
		// Punctuation goes; hyphens and underscores stay.
		{"What's new: a lot!", "whats-new-a-lot"},
		{"snake_case and kebab-case", "snake_case-and-kebab-case"},
		{"###", ""},
	} {
		if got := Slug(c.heading); got != c.want {
			t.Errorf("Slug(%q) = %q, want %q", c.heading, got, c.want)
		}
	}
}

// A reference link whose definition is missing renders as the literal text it was
// written as, which is a break that reading the rendered page does not reveal.
//
// Verifies: REQ-MD-009
func TestUndefinedReferenceIsReported(t *testing.T) {
	src, err := os.ReadFile("testdata/repo/README.md")
	if err != nil {
		t.Fatal(err)
	}
	var refs []string
	for _, l := range Links(src) {
		if l.Ref != "" {
			refs = append(refs, l.Ref)
		}
	}
	if len(refs) != 1 || refs[0] != "nowhere" {
		t.Errorf("undefined references %v, want [nowhere]", refs)
	}
}

func TestTarget(t *testing.T) {
	for _, c := range []struct {
		from, dest, want string
		ok               bool
	}{
		{"README.md", "docs/SPEC.md", "docs/SPEC.md", true},
		{"docs/SPEC.md", "../README.md", "README.md", true},
		{"docs/SPEC.md", "../README.md#install", "README.md", true},
		{"README.md", "docs/SPEC.md?plain=1", "docs/SPEC.md", true},
		{"README.md", "/docs/SPEC.md", "docs/SPEC.md", true},
		// A destination is a URL, so a space arrives escaped.
		{"README.md", "docs/my%20file.md", "docs/my file.md", true},
		// Not a path in this repository.
		{"README.md", "https://example.test", "", false},
		{"README.md", "mailto:nobody@example.test", "", false},
		{"README.md", "#install", "", false},
		{"README.md", "../../outside.md", "", false},
		{"README.md", "", "", false},
	} {
		got, ok := Target(c.from, c.dest)
		if got != c.want || ok != c.ok {
			t.Errorf("Target(%q, %q) = %q, %v; want %q, %v", c.from, c.dest, got, ok, c.want, c.ok)
		}
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

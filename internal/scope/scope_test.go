package scope

import (
	"reflect"
	"sort"
	"testing"
)

func TestMatchesTheWayGoprivateDoes(t *testing.T) {
	p := New([]string{"corp.example/*", "@acme/*", "com.acme.*"})
	for name, want := range map[string]bool{
		// A pattern matches leading path elements, so it covers what is under it.
		"corp.example/team/billing": true,
		"corp.example/lib":          true,
		"corp.example":              false, // one element short of the pattern
		"notcorp.example/team":      false,
		// A scope, a group: the same rule with no slashes to speak of.
		"@acme/widgets":       true,
		"@acmeish/widgets":    false,
		"com.acme.billing":    true,
		"com.acmeish.billing": false,
		// And the world stays public.
		"github.com/spf13/cobra": false,
		"react":                  false,
	} {
		if got := p.Match("go", name); got != want {
			t.Errorf("Match(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestAPatternMayNameItsEcosystem(t *testing.T) {
	p := New([]string{"npm:@acme/*", "oci:harbor.corp/*"})
	if !p.Match("npm", "@acme/widgets") {
		t.Error("the npm pattern did not match an npm package")
	}
	// The same name in another ecosystem is somebody else's.
	if p.Match("pypi", "@acme/widgets") {
		t.Error("an npm-scoped pattern matched a PyPI package")
	}
	if !p.Match("oci", "harbor.corp/team/app") {
		t.Error("the oci pattern did not match an image")
	}
}

func TestAHostAndPortIsNotAnEcosystem(t *testing.T) {
	// "localhost:5000/*" is a registry, not the ecosystem "localhost". Cutting at the
	// colon regardless would have filed it under an ecosystem nothing ever names, and
	// the pattern would have matched nothing at all.
	p := New([]string{"localhost:5000/*"})
	if !p.Match("oci", "localhost:5000/app") {
		t.Error("a registry with a port was read as an ecosystem prefix")
	}
}

func TestOneEntryMayHoldSeveralPatterns(t *testing.T) {
	// One flag, one environment variable and one config entry all say the same thing,
	// because GOPRIVATE itself is a comma-separated list.
	p := New([]string{"corp.example/*, @acme/*", "", "  "})
	if !p.Match("go", "corp.example/x") || !p.Match("npm", "@acme/y") {
		t.Errorf("a comma-separated entry did not split: %v", p.Patterns())
	}
}

func TestNothingDeclaredMatchesNothing(t *testing.T) {
	for _, p := range []*Private{nil, New(nil), New([]string{"", ","})} {
		if !p.Empty() {
			t.Errorf("%v is not empty", p.Patterns())
		}
		if p.Match("go", "corp.example/x") {
			t.Error("an empty matcher claimed a package")
		}
	}
}

func TestReadsWhatTheMachineAlreadySaysAboutGo(t *testing.T) {
	env := map[string]string{
		"GOPRIVATE": "corp.example/*,other.example/x",
		"GONOPROXY": "corp.example/*", // usually a copy of GOPRIVATE; a repeat is harmless
		"GONOSUMDB": "none",           // means "nothing", not a module called none
	}
	got := FromGoEnv(func(k string) string { return env[k] })
	sort.Strings(got)
	want := []string{"go:corp.example/*", "go:corp.example/*", "go:other.example/x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	// And they apply to Go and to nothing else, since that is what they were set for.
	p := New(got)
	if !p.Match("go", "corp.example/lib") {
		t.Error("GOPRIVATE did not reach the Go ecosystem")
	}
	if p.Match("npm", "corp.example/lib") {
		t.Error("GOPRIVATE reached beyond Go")
	}
}

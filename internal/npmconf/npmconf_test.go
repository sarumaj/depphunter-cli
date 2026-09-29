package npmconf

import (
	"reflect"
	"testing"
)

// Yarn's ${NAME}, ${NAME-x} and ${NAME:-x}, embedded or whole; an unset variable
// without a fallback makes the value unusable.
//
// Verifies: REQ-AUTH-024
func TestInterpolate(t *testing.T) {
	environment := func(k string) string { return map[string]string{"SET": "v"}[k] }
	for in, want := range map[string]string{
		"${SET}":             "v",
		"user:${SET}":        "user:v",
		"${UNSET:-fallback}": "fallback",
		"${UNSET-fb}":        "fb",
		"${SET:-fb}":         "v",
		"plain":              "plain",
	} {
		if got, ok := Interpolate(in, environment); !ok || got != want {
			t.Errorf("%s: %q %v, want %q", in, got, ok, want)
		}
	}
	if _, ok := Interpolate("x${UNSET}", environment); ok {
		t.Error("an unset variable without a fallback was accepted")
	}
}

// A value that is a variable and nothing else names it; anything else names none.
//
// Verifies: REQ-AUTH-023
func TestReference(t *testing.T) {
	for in, want := range map[string]string{
		"${NPM_TOKEN}": "NPM_TOKEN", "$NPM_TOKEN": "NPM_TOKEN", "${T:-x}": "T",
		"written": "", "x${T}": "", "$": "", "${T}x": "", "$A-B": "",
	} {
		if got, _ := Reference(in); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
	if _, fb := Reference("${T:-x}"); !fb {
		t.Error("a fallback went unreported")
	}
}

// Yarn 1's .yarnrc names a registry and scope registries, quoted or not.
//
// Verifies: REQ-SUP-015
func TestParseYarnClassic(t *testing.T) {
	s := ParseYarnClassic([]byte("# yarn lockfile v1\nregistry \"https://a.corp\"\n\"@s:registry\" \"https://s.corp\"\nstrict-ssl false\n"))
	if s.Registry.URL != "https://a.corp" || !reflect.DeepEqual(s.Scopes, map[string]Entry{"@s": {URL: "https://s.corp"}}) {
		t.Errorf("got %+v", s)
	}
}

// A bunfig URL's user:password moves out of the URL; a bare :token is a token.
//
// Verifies: REQ-AUTH-024
func TestBunfigURLCredentials(t *testing.T) {
	s, ok := ParseBunfig([]byte("[install]\nregistry = \"https://u:p@a.corp/npm/\"\n[install.scopes]\nt = \"https://:tok@t.corp/\"\n"))
	if !ok {
		t.Fatal("not parsed")
	}
	if s.Registry != (Entry{URL: "https://a.corp/npm/", Username: "u", Password: "p"}) {
		t.Errorf("registry %+v", s.Registry)
	}
	if s.Scopes["@t"] != (Entry{URL: "https://t.corp/", Token: "tok"}) {
		t.Errorf("scope %+v", s.Scopes["@t"])
	}
}

// Yarn's files merge key by key at every depth, the closest winning: a closer scope
// entry that sets only a token keeps a farther file's registry for the scope, a
// closer scalar replaces a farther mapping, and a file that is not YAML is skipped.
//
// Verifies: REQ-AUTH-024, REQ-SUP-079
func TestMergeYarnrc(t *testing.T) {
	closest := []byte("npmScopes:\n  acme:\n    npmAuthToken: near\nnpmRegistries: dropped\n")
	broken := []byte("npmScopes: [\n")
	farthest := []byte("npmRegistryServer: https://far.corp\nnpmAlwaysAuth: true\nnpmScopes:\n  acme:\n    npmRegistryServer: https://acme.corp\n    npmAuthToken: far\n  other:\n    npmRegistryServer: https://other.corp\nnpmRegistries:\n  https://reg.corp:\n    npmAuthToken: r\n")
	s, ok := MergeYarnrc([][]byte{closest, broken, farthest})
	if !ok {
		t.Fatal("nothing read")
	}
	want := Settings{
		Registry: Entry{URL: "https://far.corp", AlwaysAuth: "true"},
		Scopes: map[string]Entry{
			"@acme":  {URL: "https://acme.corp", Token: "near"},
			"@other": {URL: "https://other.corp"},
		},
	}
	if !reflect.DeepEqual(s, want) {
		t.Errorf("got  %+v\nwant %+v", s, want)
	}
	if _, ok := MergeYarnrc([][]byte{broken}); ok {
		t.Error("a file that is not YAML was read")
	}
}

// Yarn sends a registry's credential with every package's metadata request only
// under npmAlwaysAuth (true or 1, variables resolved); otherwise with the scoped
// packages', and a scope's credential only with its own packages'. An npmRegistries
// entry for the default registry takes the place of the top level's credential.
// YARN_NPM_* replace the top level, YARN_NPM_SCOPES is not read.
//
// Verifies: REQ-AUTH-024
func TestYarnCredentials(t *testing.T) {
	s, _ := ParseYarnrc([]byte(`
npmRegistryServer: https://top.corp
npmAuthToken: top
npmAlwaysAuth: "${ALWAYS}"
npmScopes:
  acme:
    npmRegistryServer: https://acme.corp
    npmAuthToken: acme
    npmAlwaysAuth: true
npmRegistries:
  https://reg.corp:
    npmAuthToken: reg
  //one.corp:
    npmAuthIdent: "u:p"
    npmAlwaysAuth: 1
`))
	summary := func(entries []Entry) map[string]string {
		out := map[string]string{}
		for _, e := range entries {
			out[e.URL] += e.Packages + "|"
		}
		return out
	}
	resolve := func(v string) string {
		out, _ := Interpolate(v, func(string) string { return "true" })
		return out
	}
	want := map[string]string{"https://top.corp": "|", "https://acme.corp": "@acme|", "https://reg.corp": "@|", "https://one.corp": "|"}
	if got := summary(s.YarnCredentials(resolve)); !reflect.DeepEqual(got, want) {
		t.Errorf("resolved: %v, want %v", got, want)
	}
	want["https://top.corp"] = "@|"
	if got := summary(s.YarnCredentials(nil)); !reflect.DeepEqual(got, want) {
		t.Errorf("unresolved: %v, want %v", got, want)
	}

	// An npmRegistries entry for the default registry, even one without a
	// credential, is what Yarn takes for it.
	s, _ = ParseYarnrc([]byte("npmRegistryServer: https://top.corp/\nnpmAuthToken: top\nnpmRegistries:\n  //top.corp:\n    npmAlwaysAuth: true\n"))
	if got := s.YarnCredentials(nil); len(got) != 0 {
		t.Errorf("the top level's credential was kept: %+v", got)
	}

	s = Settings{}
	s.YarnEnvironment(func(k string) string {
		return map[string]string{"YARN_NPM_AUTH_TOKEN": "env", "YARN_NPM_ALWAYS_AUTH": "1", "YARN_NPM_SCOPES": `{"x":{}}`}[k]
	})
	if got := s.YarnCredentials(nil); len(got) != 1 || got[0].URL != YarnDefault || got[0].Token != "env" || got[0].Packages != "" || s.Scopes != nil {
		t.Errorf("environment: %+v, scopes %v", got, s.Scopes)
	}
}

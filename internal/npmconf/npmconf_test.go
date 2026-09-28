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
	env := func(k string) string { return map[string]string{"SET": "v"}[k] }
	for in, want := range map[string]string{
		"${SET}":             "v",
		"user:${SET}":        "user:v",
		"${UNSET:-fallback}": "fallback",
		"${UNSET-fb}":        "fb",
		"${SET:-fb}":         "v",
		"plain":              "plain",
	} {
		if got, ok := Interpolate(in, env); !ok || got != want {
			t.Errorf("%s: %q %v, want %q", in, got, ok, want)
		}
	}
	if _, ok := Interpolate("x${UNSET}", env); ok {
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

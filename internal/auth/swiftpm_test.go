package auth

import (
	"path/filepath"
	"testing"
)

// `swift package-registry login` keeps the credential in the netrc. The user's
// registries.json says how SwiftPM sends it: "token" as a Bearer token to the
// registry's path (the host's other paths keep the netrc pair), "basic" as the
// netrc pair; with no entry, a netrc login of "token" is a token. A registry the
// netrc has nothing for gets nothing, and so does a host only a repository's
// registries.json would name.
//
// Verifies: REQ-AUTH-034
func TestSwiftPMRegistryLogins(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".netrc"), "machine tokens.example login token password tok1\n"+
		"machine basic.example login mona password secret\n"+
		"machine implicit.example login token password tok2\n"+
		"machine shared.example login mona password pair\n")
	writeFile(t, filepath.Join(home, ".swiftpm", "configuration", "registries.json"), `{
  "registries": {
    "[default]": {"url": "https://tokens.example/swift"},
    "mona": {"url": "https://basic.example"},
    "acme": {"url": "https://implicit.example/"},
    "other": {"url": "https://none.example"},
    "shared": {"url": "https://shared.example:8443/api/swift"}
  },
  "authentication": {
    "tokens.example": {"type": "token"},
    "basic.example": {"type": "basic"},
    "shared.example:8443": {"type": "token"}
  },
  "version": 1
}`)
	c := onMachine(t, home, "linux", nil)
	for raw, want := range map[string]string{
		"https://tokens.example/swift/mona/LinkedList":         bearer("tok1"),
		"https://tokens.example/other/path":                    basicHeader("token:tok1"),
		"https://basic.example/mona/LinkedList":                basicHeader("mona:secret"),
		"https://implicit.example/acme/Kit":                    bearer("tok2"),
		"https://none.example/other/Kit":                       "",
		"https://shared.example:8443/api/swift/shared/Package": bearer("pair"),
		"https://shared.example/npm/left-pad":                  basicHeader("mona:pair"),
	} {
		if got := authorization(t, c, raw); got != want {
			t.Errorf("%s: %q, want %q", raw, got, want)
		}
	}
}

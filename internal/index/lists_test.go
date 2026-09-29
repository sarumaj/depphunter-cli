package index

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Paket asks only the sources paket.dependencies lists: without nuget.org among
// them (in any group), nuget.org is not asked. paket.lock names where packages came
// from, not what may be asked, and switches nothing off.
//
// Verifies: REQ-FSHARP-010, REQ-SUP-063
func TestPaketSourcesLeaveNuGetOrgOut(t *testing.T) {
	for _, test := range []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"private feed only", map[string]string{"paket.dependencies": "source https://paket.corp/v3/index.json\nnuget Argu\n"},
			[]string{"https://paket.corp/v3/index.json?"}},
		{"a directory only", map[string]string{"paket.dependencies": "source ./local-packages\nnuget Argu\n"}, nil},
		{"nuget.org in another group", map[string]string{"paket.dependencies": "source https://paket.corp/v3/index.json\nnuget Argu\n\ngroup Build\n  source https://www.nuget.org/api/v2\n  nuget FAKE\n"},
			[]string{"https://paket.corp/v3/index.json?", "https://api.nuget.org/v3/index.json"}},
		{"no source line", map[string]string{"paket.dependencies": "github fsharp/FAKE src/app/FakeLib/Globbing/Globbing.fs\n"},
			[]string{"https://api.nuget.org/v3/index.json"}},
		{"a lock alone", map[string]string{"paket.lock": "NUGET\n  remote: https://feed.internal/v3/index.json\n    Argu (6.1.1)\n"},
			[]string{"https://feed.internal/v3/index.json?", "https://api.nuget.org/v3/index.json"}},
	} {
		c := Discover(write(t, test.files), environment(nil), "")
		if got := order(c, NuGet, "Argu", ""); !slices.Equal(got, test.want) {
			t.Errorf("%s: asked %v, want %v", test.name, got, test.want)
		}
	}
}

// R asks every repository of its repos option, in order: a package the first
// CRAN-like repository's PACKAGES does not list is found on the next, and one the
// first lists is never named to the next.
//
// Verifies: REQ-SUP-048, REQ-SUP-063
func TestRRepositoriesFallBack(t *testing.T) {
	cran := newFeed(t, map[string]string{"/src/contrib/PACKAGES": "Package: dplyr\nVersion: 1.1.4\nImports: cli\n"})
	corp := newFeed(t, map[string]string{"/src/contrib/PACKAGES": "Package: acmeR\nVersion: 0.3.0\nImports: jsonlite\n\nPackage: dplyr\nVersion: 9.9.9\nImports: evil\n"})
	asPublic(t, CRAN, cran)
	profile := filepath.Join(t.TempDir(), "profile.R")
	os.WriteFile(profile, []byte(`options(repos = c(CRAN = "@CRAN@", corp = "`+corp.URL+`"))`), 0o644)
	c := newClient(t, Discover(nil, environment(map[string]string{"R_PROFILE_USER": profile}), ""))
	if got, l := ask(t, c, lang.Target{Ecosystem: CRAN, Package: "acmeR", Version: "0.3.0"}); !slices.Equal(got, []string{"jsonlite"}) || l.Index != corp.URL {
		t.Errorf("acmeR: %v from %s", got, l.Index)
	}
	if got, l := ask(t, c, lang.Target{Ecosystem: CRAN, Package: "dplyr", Version: "1.1.4"}); !slices.Equal(got, []string{"cli"}) || l.Index != cran.URL {
		t.Errorf("dplyr: %v from %s", got, l.Index)
	}
}

// cabal combines its repositories: a package the private repository lacks is found
// on Hackage, after it.
//
// Verifies: REQ-SUP-049, REQ-SUP-063
func TestCabalRepositoriesFallBack(t *testing.T) {
	hackage := newFeed(t, map[string]string{
		"/package/aeson/preferred":                 `{"normal-version": ["2.2.1.0"]}`,
		"/package/aeson-2.2.1.0/aeson.cabal":       "name: aeson\nlibrary\n  build-depends: base, text\n",
		"/package/acme-json/preferred":             `{"normal-version": ["1.0.0"]}`,
		"/package/acme-json-1.0.0/acme-json.cabal": "name: acme-json\nlibrary\n  build-depends: evil\n",
		"/package/legacy/preferred":                `{"normal-version": ["0.1"]}`,
		"/package/legacy-0.1/legacy.cabal":         "name: legacy\nlibrary\n  build-depends: base, mtl\n",
	})
	corp := newFeed(t, map[string]string{
		"/package/acme-json/preferred":             `{"normal-version": ["1.0.0"]}`,
		"/package/acme-json-1.0.0/acme-json.cabal": "name: acme-json\nlibrary\n  build-depends: aeson\n",
		"/package/legacy/preferred":                `{"normal-version": [], "deprecated-version": ["0.1"]}`,
	})
	asPublic(t, Hackage, hackage)
	config := filepath.Join(t.TempDir(), "config")
	os.WriteFile(config, []byte("repository hackage.haskell.org\n  url: "+hackage.URL+"\n\nrepository corp\n  url: "+corp.URL+"\n"), 0o644)
	c := newClient(t, Discover(nil, environment(map[string]string{"CABAL_CONFIG": config}), ""))
	if got, l := ask(t, c, lang.Target{Ecosystem: Hackage, Package: "acme-json", Version: "1.0.0", Pinned: true}); !slices.Equal(got, []string{"aeson"}) || l.Index != corp.URL {
		t.Errorf("acme-json: %v from %s", got, l.Index)
	}
	if got, l := ask(t, c, lang.Target{Ecosystem: Hackage, Package: "aeson", Version: "2.2.1.0", Pinned: true}); !slices.Equal(got, []string{"text"}) || l.Index != hackage.URL {
		t.Errorf("aeson: %v from %s", got, l.Index)
	}
	// Every release the private repository has is deprecated: Hackage's normal
	// one answers.
	if got, l := ask(t, c, lang.Target{Ecosystem: Hackage, Package: "legacy"}); !slices.Equal(got, []string{"mtl"}) || l.Index != hackage.URL {
		t.Errorf("legacy: %v from %s", got, l.Index)
	}
	if hackage.askedFor("acme-json") {
		t.Error("a package the private repository has was named to Hackage")
	}
}

// LuaRocks searches every rocks server: a rock the first server's manifest does not
// list (or lists with no version the constraint admits) is found on the next.
//
// Verifies: REQ-SUP-052, REQ-SUP-063
func TestRocksServersFallBack(t *testing.T) {
	first := newFeed(t, map[string]string{
		"/manifest-5.1": `repository = { ["acme-http"] = { ["1.0.0-1"] = { { arch = "rockspec" } } } }`,
	})
	second := newFeed(t, map[string]string{
		"/manifest-5.1":               `repository = { ["acme-http"] = { ["2.1.0-1"] = { { arch = "rockspec" } } }, penlight = { ["1.14.0-3"] = { { arch = "rockspec" } } } }`,
		"/acme-http-2.1.0-1.rockspec": `package = "acme-http" dependencies = { "penlight >= 1.5" }`,
		"/penlight-1.14.0-3.rockspec": `package = "penlight" dependencies = { "luafilesystem" }`,
	})
	config := filepath.Join(t.TempDir(), "config.lua")
	os.WriteFile(config, []byte(`rocks_servers = { "`+first.URL+`", "`+second.URL+`" }`), 0o644)
	c := newClient(t, Discover(nil, environment(map[string]string{"LUAROCKS_CONFIG": config}), ""))
	if got, l := ask(t, c, lang.Target{Ecosystem: LuaRocks, Package: "penlight"}); !slices.Equal(got, []string{"luafilesystem"}) || l.Index != second.URL {
		t.Errorf("penlight: %v from %s", got, l.Index)
	}
	if got, l := ask(t, c, lang.Target{Ecosystem: LuaRocks, Package: "acme-http", Version: ">= 2"}); !slices.Equal(got, []string{"penlight"}) || l.Index != second.URL {
		t.Errorf("acme-http >= 2: %v from %s", got, l.Index)
	}
}

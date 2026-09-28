package nuget

import (
	"reflect"
	"testing"
)

func configs(t *testing.T, docs ...string) []ConfigFile {
	t.Helper()
	var out []ConfigFile
	for _, d := range docs {
		f, ok := ParseConfig([]byte(d))
		if !ok {
			t.Fatalf("not parsed: %s", d)
		}
		out = append(out, f)
	}
	return out
}

// Files merge from the farthest to the closest: a closer key replaces a farther
// one's URL in place, a <clear/> drops what the section held before it (in its own
// file too), and disabledPackageSources, packageSourceMapping and credentials merge
// the same way.
//
// Verifies: REQ-SUP-065
func TestMerge(t *testing.T) {
	s := Merge(configs(t,
		// closest
		`<configuration>
  <packageSources><add key="Corp" value="https://new.corp/v3/index.json"/><add key="repo" value="https://repo/v3/index.json"/></packageSources>
  <disabledPackageSources><add key="vendor" value="false"/><add key="old" value="true"/></disabledPackageSources>
  <packageSourceMapping><packageSource key="corp"><package pattern="Corp.*"/></packageSource></packageSourceMapping>
</configuration>`,
		// farthest
		`<configuration>
  <packageSources><add key="gone" value="https://gone/v3/index.json"/><clear/>
    <add key="corp" value="https://old.corp/v3/index.json"/><add key="vendor" value="https://vendor/v3/index.json"/></packageSources>
  <disabledPackageSources><add key="vendor" value="true"/></disabledPackageSources>
  <packageSourceMapping><packageSource key="corp"><package pattern="Old.*"/></packageSource></packageSourceMapping>
  <packageSourceCredentials><Corp_x0020_Feed><add key="Username" value="u"/><add key="Password" value="AQAAANCMnd8"/></Corp_x0020_Feed></packageSourceCredentials>
</configuration>`))
	want := []Feed{{"corp", "https://new.corp/v3/index.json", 0}, {"vendor", "https://vendor/v3/index.json", 1}, {"repo", "https://repo/v3/index.json", 0}}
	if !reflect.DeepEqual(s.Feeds, want) {
		t.Errorf("feeds %v", s.Feeds)
	}
	if !s.Cleared {
		t.Error("the <clear/> was not recorded")
	}
	if !s.Enabled("VENDOR") || s.Enabled("old") {
		t.Error("disabledPackageSources not merged")
	}
	if keys, ok := s.Route("Old.Thing"); ok {
		t.Errorf("a closer packageSource replaces the farther one's patterns: %v", keys)
	}
	if c := s.Credentials["corp feed"]; c.Username != "u" || !c.Encrypted || c.ClearTextPassword != "" || c.Layer != 1 {
		t.Errorf("credential %+v", c)
	}
	s = Merge(configs(t, `<configuration><packageSourceMapping><clear/></packageSourceMapping></configuration>`,
		`<configuration><packageSourceMapping><packageSource key="a"><package pattern="*"/></packageSource></packageSourceMapping></configuration>`))
	if _, ok := s.Route("X"); ok {
		t.Error("a closer <clear/> left the farther mapping")
	}
}

// The most specific pattern wins: an exact id over any prefix, a longer prefix over
// a shorter, `*` last; ties keep every key, in the order of the sources.
//
// Verifies: REQ-SUP-065
func TestRoute(t *testing.T) {
	s := Merge(configs(t, `<configuration>
  <packageSources><add key="b" value="https://b"/><add key="a" value="https://a"/></packageSources>
  <packageSourceMapping>
    <packageSource key="a"><package pattern="*"/><package pattern="Contoso.*"/><package pattern="bad*pattern"/></packageSource>
    <packageSource key="b"><package pattern="Contoso.*"/><package pattern="Contoso.Exact"/></packageSource>
    <packageSource key="c"><package pattern="Contoso.Deeper.*"/></packageSource>
  </packageSourceMapping>
</configuration>`))
	for id, want := range map[string][]string{
		"Anything":           {"a"},
		"contoso.lib":        {"b", "a"},
		"Contoso.Exact":      {"b"},
		"Contoso.Deeper.Lib": {"c"},
		"badXpattern":        {"a"}, // only by *
	} {
		if got, ok := s.Route(id); !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %v %v, want %v", id, got, ok, want)
		}
	}
	if _, ok := Merge(nil).Route("X"); ok {
		t.Error("no mapping routed a package")
	}
}

// Verifies: REQ-AUTH-022
func TestEnvCredential(t *testing.T) {
	for v, want := range map[string][3]string{
		"Username=ci;Password=p@ss=word":                                    {"ci", "p@ss=word", "ok"},
		"username=ci; password=x; ValidAuthenticationTypes=basic,negotiate": {"ci", "x", "ok"},
		"Username=ci;Password=x;ValidAuthenticationTypes=negotiate":         {"ci", "x", ""},
		"Username=ci": {"ci", "", ""},
		"":            {"", "", ""},
	} {
		u, p, ok := EnvCredential(v)
		if u != want[0] || p != want[1] || ok != (want[2] == "ok") {
			t.Errorf("%q: %q %q %v", v, u, p, ok)
		}
	}
	if got := DecodeName("My_x0020_Feed_x0040_corp_x00zz_"); got != "My Feed@corp_x00zz_" {
		t.Errorf("DecodeName: %q", got)
	}
}

// A Paket source line's username:, password: and authtype: options are read, quoted
// or not.
//
// Verifies: REQ-FSHARP-010
func TestPaketSourceOptions(t *testing.T) {
	_, sources := ParseDependencies([]byte("source https://feed/v3/index.json username: \"ci user\" password: \"%PAT%\" authtype: basic\nsource ./local\n"))
	want := []Source{
		{Group: MainGroup, URL: "https://feed/v3/index.json", Username: "ci user", Password: "%PAT%", AuthType: "basic"},
		{Group: MainGroup, URL: "./local"},
	}
	if !reflect.DeepEqual(sources, want) {
		t.Errorf("%+v", sources)
	}
}

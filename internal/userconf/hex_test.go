package userconf

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// pub-tokens.json is in Dart's configuration directory: %APPDATA%\dart on Windows
// (the XDG one without %APPDATA%), ~/Library/Application Support/dart on macOS,
// $XDG_CONFIG_HOME/dart (~/.config/dart) elsewhere. hex.config is in HEX_HOME, the
// XDG directory under MIX_XDG, else ~/.hex.
//
// Verifies: REQ-SUP-064, REQ-AUTH-027, REQ-AUTH-028
func TestDartAndHexLocations(t *testing.T) {
	home, xdg, other := t.TempDir(), t.TempDir(), t.TempDir()
	for _, tc := range []struct {
		goos      string
		vars      map[string]string
		pub, hexd string
	}{
		{"linux", nil, filepath.Join(home, ".config", "dart", "pub-tokens.json"), filepath.Join(home, ".hex")},
		{"linux", map[string]string{"XDG_CONFIG_HOME": xdg, "MIX_XDG": "1"},
			filepath.Join(xdg, "dart", "pub-tokens.json"), filepath.Join(xdg, "hex")},
		{"linux", map[string]string{"MIX_XDG": "true"}, filepath.Join(home, ".config", "dart", "pub-tokens.json"),
			filepath.Join(home, ".config", "hex")},
		{"linux", map[string]string{"MIX_XDG": "0", "HEX_HOME": other}, filepath.Join(home, ".config", "dart", "pub-tokens.json"), other},
		{"darwin", map[string]string{"XDG_CONFIG_HOME": xdg}, filepath.Join(home, "Library", "Application Support", "dart", "pub-tokens.json"),
			filepath.Join(home, ".hex")},
		{"windows", map[string]string{"APPDATA": other}, filepath.Join(other, "dart", "pub-tokens.json"), filepath.Join(home, ".hex")},
		{"windows", nil, filepath.Join(home, ".config", "dart", "pub-tokens.json"), filepath.Join(home, ".hex")},
	} {
		m := machine(t, home, tc.goos, tc.vars)
		if got := m.PubTokens(); got != tc.pub {
			t.Errorf("%s %v: pub %s", tc.goos, tc.vars, got)
		}
		if got := m.HexHome(); got != tc.hexd {
			t.Errorf("%s %v: hex %s", tc.goos, tc.vars, got)
		}
	}
	if got := machine(t, "", "linux", nil).PubTokens(); got != "" {
		t.Errorf("no home: %q", got)
	}
}

// hex.config as Hex writes it with io_lib:print: the API URL, a plain api_key, the
// OAuth token of `mix hex.user auth` (string keys from older versions too), and
// $repos with an organization's auth_key; comments, binaries with /utf8 and byte
// lists read, encrypted keys left out.
//
// Verifies: REQ-AUTH-028
func TestParseHexConfig(t *testing.T) {
	got := ParseHexConfig([]byte(`% written by Hex
{api_url,<<"https://hex.corp/api">>}.
{api_key,<<"user-key">>}.
{'$encrypted_key',<<1,2,3>>}.
{'$oauth_token',#{access_token => <<"oauth-access">>,refresh_token => <<"r">>,
                  expires_at => 1893456000}}.
{'$repos',#{<<"hexpm:acme">> =>
                #{auth_key => <<"acme-key"/utf8>>,url => <<"https://repo.hex.pm/repos/acme">>,
                  public_key => <<"-----BEGIN PUBLIC KEY-----\nMIIB\n-----END PUBLIC KEY-----\n">>},
            <<"hexpm:beta">> => #{"oauth_token" => #{"access_token" => "beta-token","expires_at" => 1}},
            <<"bytes">> => #{auth_key => <<107,101,121>>},
            <<"hexpm">> => #{url => <<"https://repo.hex.pm">>}}}.
{unsafe_https,false}.
`))
	want := HexConfig{
		APIURL: "https://hex.corp/api",
		APIKey: "user-key",
		OAuth:  HexToken{Access: "oauth-access", Expires: 1893456000},
		Repos: map[string]HexRepo{
			"hexpm:acme": {AuthKey: "acme-key"},
			"hexpm:beta": {OAuth: HexToken{Access: "beta-token", Expires: 1}},
			"bytes":      {AuthKey: "key"},
			"hexpm":      {},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	// A term that does not read ends the file; what came before is kept.
	got = ParseHexConfig([]byte("{api_key,<<\"k\">>}.\n{broken,\n{api_url,<<\"x\">>}.\n"))
	if got.APIKey != "k" || got.APIURL != "" {
		t.Errorf("broken file: %+v", got)
	}
}

// The Hex API is HEX_API_URL, else HEX_API, else hex.config's api_url.
//
// Verifies: REQ-SUP-015, REQ-SUP-064
func TestHexAPIURL(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".hex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".hex", "hex.config"), []byte(`{api_url,<<"https://file.corp/api">>}.`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		vars map[string]string
		want string
	}{
		{nil, "https://file.corp/api"},
		{map[string]string{"HEX_API": "https://api.corp/api"}, "https://api.corp/api"},
		{map[string]string{"HEX_API": "https://api.corp/api", "HEX_API_URL": "https://url.corp/api"}, "https://url.corp/api"},
		{map[string]string{"HEX_HOME": t.TempDir()}, ""},
	} {
		if got := machine(t, home, "linux", tc.vars).HexAPIURL(); got != tc.want {
			t.Errorf("%v: %q, want %q", tc.vars, got, tc.want)
		}
	}
}

// rebar3's global rebar.config is in .config/rebar3 under REBAR_GLOBAL_CONFIG_DIR,
// else the home directory; its hex.config there too, under REBAR_CACHE_DIR when
// that is set and REBAR_GLOBAL_CONFIG_DIR is not, as rebar3 finds it once the
// project is loaded.
//
// Verifies: REQ-SUP-064, REQ-AUTH-028, REQ-BEAM-013
func TestRebar3Locations(t *testing.T) {
	home, global, cache := t.TempDir(), t.TempDir(), t.TempDir()
	for _, tc := range []struct {
		vars          map[string]string
		config, hexed string
	}{
		{nil, filepath.Join(home, ".config", "rebar3", "rebar.config"), filepath.Join(home, ".config", "rebar3", "hex.config")},
		{map[string]string{"REBAR_CACHE_DIR": cache}, filepath.Join(home, ".config", "rebar3", "rebar.config"),
			filepath.Join(cache, ".config", "rebar3", "hex.config")},
		{map[string]string{"REBAR_CACHE_DIR": cache, "REBAR_GLOBAL_CONFIG_DIR": global},
			filepath.Join(global, ".config", "rebar3", "rebar.config"), filepath.Join(global, ".config", "rebar3", "hex.config")},
	} {
		m := machine(t, home, "linux", tc.vars)
		if got := m.Rebar3GlobalConfig(); got != tc.config {
			t.Errorf("%v: rebar.config %s", tc.vars, got)
		}
		if got := m.Rebar3HexConfig(); got != tc.hexed {
			t.Errorf("%v: hex.config %s", tc.vars, got)
		}
	}
	if got := machine(t, "", "linux", nil).Rebar3HexConfig(); got != "" {
		t.Errorf("no home: %q", got)
	}
}

// The repos of a rebar.config's {hex, ...}, in order across its entries; the first
// entry says whether they replace hex.pm's. Names may be binaries or strings; a
// map without one and the other options are passed over.
//
// Verifies: REQ-BEAM-013
func TestParseRebar3HexRepos(t *testing.T) {
	for src, want := range map[string]struct {
		repos   []string
		replace bool
	}{
		`{hex, [{repos, [#{name => <<"hexpm:acme">>, repo_key => <<"k">>}]}]}.`: {[]string{"hexpm:acme"}, false},
		`%% global
{plugins, [rebar3_hex]}.
{hex, [{doc, #{provider => ex_doc}},
       {repos, replace, [#{name => <<"hexpm:acme">>}, #{name => "hexpm"}]},
       {repos, [#{repo_url => <<"x">>}, #{name => <<"hexpm:beta">>}]}]}.`: {[]string{"hexpm:acme", "hexpm", "hexpm:beta"}, true},
		`{hex, [{repos, [#{name => <<"a">>}]}, {repos, replace, [#{name => <<"b">>}]}]}.`: {[]string{"a", "b"}, false},
		`{deps, []}.`: {nil, false},
	} {
		repos, replace := ParseRebar3HexRepos([]byte(src))
		if !reflect.DeepEqual(repos, want.repos) || replace != want.replace {
			t.Errorf("%s: %v %v", src, repos, replace)
		}
	}
}

// rebar3's hex.config, as rebar3 writes it with io_lib:print: one map from a
// repository name to its keys. hexpm's api_key is the user's key, $oauth the
// user's token; each repository's api_key, repo_key (else auth_key) and
// oauth_token are its own; the atom undefined is no key.
//
// Verifies: REQ-AUTH-028
func TestParseRebar3HexConfig(t *testing.T) {
	got := ParseRebar3HexConfig([]byte(`%% coding: utf-8
#{<<"$oauth">> =>
      #{access_token => <<"user-token">>,expires_at => 1893456000,
        refresh_token => undefined},
  <<"hexpm">> => #{api_key => <<"user-key">>,repo_key => undefined},
  <<"hexpm:acme">> => #{name => <<"hexpm:acme">>,repo_key => <<"acme-key">>},
  <<"hexpm:beta">> => #{api_key => <<"beta-api">>,auth_key => <<"beta-auth">>,
                        oauth_token => #{access_token => <<"beta-token">>,expires_at => 1}},
  <<"hexpm:gamma">> => #{api_key => undefined, oauth_token => #{access_token => undefined}}}.
`))
	want := HexConfig{
		APIKey: "user-key",
		OAuth:  HexToken{Access: "user-token", Expires: 1893456000},
		Repos: map[string]HexRepo{
			"hexpm":       {APIKey: "user-key"},
			"hexpm:acme":  {AuthKey: "acme-key"},
			"hexpm:beta":  {APIKey: "beta-api", AuthKey: "beta-auth", OAuth: HexToken{Access: "beta-token", Expires: 1}},
			"hexpm:gamma": {},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	if got := ParseRebar3HexConfig([]byte("{api_key, <<\"k\">>}.\n")); got.APIKey != "" || len(got.Repos) != 0 {
		t.Errorf("Mix's hex.config read as rebar3's: %+v", got)
	}
}

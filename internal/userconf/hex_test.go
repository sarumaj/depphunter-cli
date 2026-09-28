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

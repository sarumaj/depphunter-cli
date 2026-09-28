package auth

import (
	"path/filepath"
	"testing"
	"time"
)

// pub-tokens.json: each token is a Bearer token for the URLs under its hosted URL -
// the whole host for a server at its root, its path only for one under a path - and
// an "env" entry takes the variable it names. An unset variable, a token pub would
// refuse and another host get nothing; the file is found in Dart's configuration
// directory of the platform.
//
// Verifies: REQ-AUTH-027, REQ-AUTH-020
func TestPubTokens(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".config", "dart", "pub-tokens.json"), `{"version":1,"hosted":[
  {"url":"https://pub.corp","token":"root-token"},
  {"url":"https://dart.cloudsmith.io/acme/flutter/","env":"CLOUDSMITH_TOKEN"},
  {"url":"https://dart.cloudsmith.io/acme/other","env":"UNSET_TOKEN"},
  {"url":"https://bad.corp","token":"two words"},
  {"url":"not a url","token":"x"}
]}`)
	writeFile(t, filepath.Join(home, ".netrc"), "machine dart.cloudsmith.io login n password n\n")
	c := onMachine(t, home, "linux", map[string]string{"CLOUDSMITH_TOKEN": "cs-token"})
	for u, want := range map[string]string{
		"https://pub.corp/api/packages/billing":                            bearer("root-token"),
		"https://dart.cloudsmith.io/acme/flutter/api/packages/billing":     bearer("cs-token"),
		"https://dart.cloudsmith.io/acme/flutter-other/api/packages/x":     basicHeader("n:n"),
		"https://dart.cloudsmith.io/acme/other/api/packages/x":             basicHeader("n:n"),
		"https://bad.corp/api/packages/x":                                  "",
		"https://pub.dev/api/packages/http":                                "",
		"http://pub.corp/api/packages/billing":                             "",
		"https://dart.cloudsmith.io/acme/flutter/api/packages/billing?x=1": bearer("cs-token"),
	} {
		if got := authorization(t, c, u); got != want {
			t.Errorf("%s: %q, want %q", u, got, want)
		}
	}
	mac := t.TempDir()
	writeFile(t, filepath.Join(mac, "Library", "Application Support", "dart", "pub-tokens.json"),
		`{"version":1,"hosted":[{"url":"https://pub.corp","token":"mac-token"}]}`)
	if got := authorization(t, onMachine(t, mac, "darwin", nil), "https://pub.corp/api/packages/x"); got != bearer("mac-token") {
		t.Errorf("macOS: %q", got)
	}
}

// Hex: the user's key (HEX_API_KEY over hex.config's api_key over an unexpired
// OAuth token) goes, as the whole Authorization header, to every organization's
// part of the API (<api>/repos/) and nowhere else; without one, each hexpm:<org>
// auth_key of hex.config serves that organization alone and HEX_REPOS_KEY the
// others. The API is HEX_API_URL's host when set.
//
// Verifies: REQ-AUTH-028, REQ-AUTH-020
func TestHexKeys(t *testing.T) {
	saved := now
	now = func() time.Time { return time.Unix(2000, 0) }
	t.Cleanup(func() { now = saved })
	config := `{'$repos',#{<<"hexpm:acme">> => #{auth_key => <<"acme-key">>},
  <<"hexpm:beta">> => #{oauth_token => #{access_token => <<"beta-live">>, expires_at => 3000}},
  <<"hexpm:gone">> => #{oauth_token => #{access_token => <<"gone">>, expires_at => 1000}},
  <<"mini">> => #{auth_key => <<"mini-key">>, url => <<"https://mini.corp/repo">>}}}.
`
	for _, tc := range []struct {
		name   string
		config string
		vars   map[string]string
		want   map[string]string
	}{
		{"organization keys", config, nil, map[string]string{
			"https://hex.pm/api/repos/acme/packages/billing":          "acme-key",
			"https://hex.pm/api/repos/beta/packages/x":                bearer("beta-live"),
			"https://hex.pm/api/repos/gone/packages/x":                "",
			"https://hex.pm/api/repos/acmeother/packages/x":           "",
			"https://hex.pm/api/packages/jason":                       "",
			"https://mini.corp/repo/packages/x":                       "",
			"https://hex.pm/api/repos/acme/packages/billing/releases": "acme-key",
		}},
		{"repos key for the others", config, map[string]string{"HEX_REPOS_KEY": "repos-key"}, map[string]string{
			"https://hex.pm/api/repos/acme/packages/billing": "acme-key",
			"https://hex.pm/api/repos/other/packages/x":      "repos-key",
			"https://hex.pm/api/packages/jason":              "",
		}},
		{"user key over them", config + "{api_key,<<\"file-key\">>}.\n", nil, map[string]string{
			"https://hex.pm/api/repos/acme/packages/billing": "file-key",
			"https://hex.pm/api/repos/other/packages/x":      "file-key",
			"https://hex.pm/api/packages/jason":              "",
		}},
		{"HEX_API_KEY over the file", config + "{api_key,<<\"file-key\">>}.\n", map[string]string{"HEX_API_KEY": "env-key"}, map[string]string{
			"https://hex.pm/api/repos/acme/packages/billing": "env-key",
		}},
		{"OAuth token of the user", config + "{'$oauth_token',#{access_token => <<\"user-live\">>, expires_at => 3000}}.\n", nil, map[string]string{
			"https://hex.pm/api/repos/acme/packages/billing": bearer("user-live"),
		}},
		{"expired OAuth token", config + "{'$oauth_token',#{access_token => <<\"user-old\">>, expires_at => 1000}}.\n", nil, map[string]string{
			"https://hex.pm/api/repos/acme/packages/billing": "acme-key",
		}},
		{"another API", config, map[string]string{"HEX_API_KEY": "env-key", "HEX_API_URL": "https://hex.corp/api/"}, map[string]string{
			"https://hex.corp/api/repos/acme/packages/billing": "env-key",
			"https://hex.pm/api/repos/acme/packages/billing":   "",
		}},
		{"a key that is no key", "", map[string]string{"HEX_API_KEY": "a\r\nX-Evil: 1"}, map[string]string{
			"https://hex.pm/api/repos/acme/packages/billing": "",
		}},
	} {
		home := t.TempDir()
		writeFile(t, filepath.Join(home, ".hex", "hex.config"), tc.config)
		c := onMachine(t, home, "linux", tc.vars)
		for u, want := range tc.want {
			if got := authorization(t, c, u); got != want {
				t.Errorf("%s: %s: %q, want %q", tc.name, u, got, want)
			}
		}
	}
}

// rebar3's hex.config (.config/rebar3/hex.config): an organization's own api_key
// comes first for it; the user's key is Mix's, else Mix's token, else rebar3's
// hexpm api_key, else its unexpired $oauth token; only without one does each
// hexpm:<org> repo_key (Mix's auth_key first) serve its organization, and
// HEX_REPOS_KEY the others. HEX_API_KEY is over all of them.
//
// Verifies: REQ-AUTH-028, REQ-AUTH-020
func TestRebar3HexKeys(t *testing.T) {
	saved := now
	now = func() time.Time { return time.Unix(2000, 0) }
	t.Cleanup(func() { now = saved })
	repos := `<<"hexpm:acme">> => #{name => <<"hexpm:acme">>, repo_key => <<"acme-key">>},
  <<"hexpm:beta">> => #{api_key => <<"beta-api">>},
  <<"hexpm:gone">> => #{oauth_token => #{access_token => <<"gone">>, expires_at => 1000}},
  <<"mine">> => #{repo_key => <<"mine-key">>}`
	for _, tc := range []struct {
		name, rebar3, mix string
		vars              map[string]string
		want              map[string]string
	}{
		{"organization keys", "#{" + repos + "}.", "", nil, map[string]string{
			"https://hex.pm/api/repos/acme/packages/billing": "acme-key",
			"https://hex.pm/api/repos/beta/packages/x":       "beta-api",
			"https://hex.pm/api/repos/gone/packages/x":       "",
			"https://hex.pm/api/repos/other/packages/x":      "",
			"https://hex.pm/api/packages/jason":              "",
		}},
		{"repos key for the others", "#{" + repos + "}.", "", map[string]string{"HEX_REPOS_KEY": "repos-key"}, map[string]string{
			"https://hex.pm/api/repos/acme/packages/billing": "acme-key",
			"https://hex.pm/api/repos/other/packages/x":      "repos-key",
		}},
		{"hexpm api_key of the user", "#{" + repos + `, <<"hexpm">> => #{api_key => <<"user-key">>}}.`, "", nil, map[string]string{
			"https://hex.pm/api/repos/acme/packages/billing": "user-key",
			"https://hex.pm/api/repos/beta/packages/x":       "beta-api",
			"https://hex.pm/api/repos/other/packages/x":      "user-key",
			"https://hex.pm/api/packages/jason":              "",
		}},
		{"OAuth token of the user", "#{" + repos + `, <<"$oauth">> => #{access_token => <<"user-live">>, expires_at => 3000}}.`, "", nil, map[string]string{
			"https://hex.pm/api/repos/acme/packages/billing": bearer("user-live"),
		}},
		{"hexpm api_key over the OAuth token", "#{" + repos + `, <<"hexpm">> => #{api_key => <<"user-key">>}, <<"$oauth">> => #{access_token => <<"user-live">>, expires_at => 3000}}.`, "", nil, map[string]string{
			"https://hex.pm/api/repos/acme/packages/billing": "user-key",
		}},
		{"expired OAuth token", "#{" + repos + `, <<"$oauth">> => #{access_token => <<"user-old">>, expires_at => 1000}}.`, "", nil, map[string]string{
			"https://hex.pm/api/repos/acme/packages/billing": "acme-key",
		}},
		{"Mix's key first", "#{" + repos + `, <<"hexpm">> => #{api_key => <<"rebar3-key">>}}.`, `{api_key,<<"mix-key">>}.`, nil, map[string]string{
			"https://hex.pm/api/repos/other/packages/x": "mix-key",
		}},
		{"Mix's organization key first", "#{" + repos + "}.", `{'$repos',#{<<"hexpm:acme">> => #{auth_key => <<"mix-acme">>}}}.`, nil, map[string]string{
			"https://hex.pm/api/repos/acme/packages/billing": "mix-acme",
		}},
		{"HEX_API_KEY over all", "#{" + repos + "}.", "", map[string]string{"HEX_API_KEY": "env-key"}, map[string]string{
			"https://hex.pm/api/repos/beta/packages/x": "env-key",
			"https://hex.pm/api/repos/acme/packages/x": "env-key",
		}},
	} {
		home := t.TempDir()
		writeFile(t, filepath.Join(home, ".config", "rebar3", "hex.config"), "%% coding: utf-8\n"+tc.rebar3+"\n")
		writeFile(t, filepath.Join(home, ".hex", "hex.config"), tc.mix)
		c := onMachine(t, home, "linux", tc.vars)
		for u, want := range tc.want {
			if got := authorization(t, c, u); got != want {
				t.Errorf("%s: %s: %q, want %q", tc.name, u, got, want)
			}
		}
	}
	global := t.TempDir()
	writeFile(t, filepath.Join(global, ".config", "rebar3", "hex.config"), `#{<<"hexpm:acme">> => #{repo_key => <<"moved">>}}.`)
	c := onMachine(t, t.TempDir(), "linux", map[string]string{"REBAR_GLOBAL_CONFIG_DIR": global})
	if got := authorization(t, c, "https://hex.pm/api/repos/acme/packages/billing"); got != "moved" {
		t.Errorf("REBAR_GLOBAL_CONFIG_DIR: %q", got)
	}
}

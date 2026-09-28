package auth

import (
	"cmp"
	"encoding/json"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// bearerToken is a token pub accepts from `dart pub token add` (RFC 6750's
// b64token), and the shape of a Hex key or token: nothing that could end a header or
// start another one.
var bearerToken = regexp.MustCompile(`^[A-Za-z0-9._~+/=-]+$`)

// readPub takes the tokens `dart pub token add` keeps in pub-tokens.json, each for
// the hosted-repository URL it was added for: a "token" as written, an "env" entry
// from the variable it names. Each is sent as a Bearer token to the URLs under that
// one, as pub sends it - the whole host for a server at its root, only its path for
// one under a path, so two repositories on one host keep their own tokens.
//
// Implements: REQ-AUTH-027, REQ-AUTH-020
func (c *Store) readPub(m userconf.Machine) {
	name := m.PubTokens()
	if name == "" {
		return
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return
	}
	var doc struct {
		Hosted []struct {
			URL   string `json:"url"`
			Token string `json:"token"`
			Env   string `json:"env"`
		} `json:"hosted"`
	}
	if json.Unmarshal(data, &doc) != nil {
		return
	}
	for _, h := range doc.Hosted {
		token := h.Token
		if h.Env != "" {
			token = m.Env(h.Env)
		}
		u, err := url.Parse(strings.TrimSpace(h.URL))
		if err != nil || u.Host == "" || u.User != nil || !bearerToken.MatchString(token) {
			continue
		}
		// pub refuses to add a token for plain http other than to this machine, so
		// nothing here marks a host as one to reach in the clear.
		c.file(u.Host, rootPrefix(u.Path), secret{bearer: true, value: token}, true)
	}
}

// rootPrefix is pathPrefix, with "/" for a root: a credential filed for a tool's own
// URL serves its path even when another source holds one for the whole host.
func rootPrefix(p string) string { return cmp.Or(pathPrefix(p), "/") }

// hexPublicAPI is hex.pm's API, which Mix talks to unless told otherwise.
const hexPublicAPI = "https://hex.pm/api"

// now is the clock an OAuth token's expiry is read against; tests set it.
var now = time.Now

// readHex takes what Mix's Hex and rebar3 authenticate to the Hex API with, for the
// private organizations' packages (<api>/repos/<organization>/...), the only part of
// the API that needs it. Sent as they send them - a key as the whole Authorization
// header, an OAuth token as Bearer - in their own order:
//
//   - HEX_API_KEY, which both put before anything a file holds, for every
//     organization;
//   - an organization's own api_key in rebar3's hex.config (hexpm:<organization>),
//     which rebar3 takes before the user's, for that organization's path;
//   - the user's: Mix's hex.config api_key, else the unexpired OAuth token `mix
//     hex.user auth` stores, else rebar3's hexpm api_key, else the unexpired token
//     `rebar3 hex user auth` stores ($oauth) - it reaches every organization the
//     user belongs to (<api>/repos/);
//   - only when there is none of those: each hexpm:<organization> repository's
//     key (Mix's auth_key, else its unexpired OAuth token; then rebar3's repo_key,
//     as `mix hex.organization auth <organization> --key KEY` and `rebar3 hex
//     organization auth hexpm:<organization>` write them) for that organization's
//     path, and HEX_REPOS_KEY for the others.
//
// The API is HEX_API_URL, HEX_API or hex.config's api_url, else hex.pm's. A repository
// with a URL of its own (a mini_repo, HEX_MIRROR) serves Hex's protobuf registry, not
// this API, and is not read; nor are the encrypted keys of older Hex versions.
//
// Implements: REQ-AUTH-028, REQ-AUTH-020
func (c *Store) readHex(m userconf.Machine) {
	mix, rebar3 := m.ReadHexConfig(), m.ReadRebar3HexConfig()
	api := cmp.Or(m.HexAPIURL(), hexPublicAPI)
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(api), "/"))
	if err != nil || u.Host == "" {
		return
	}
	repos := u.Path + "/repos/"
	filed := map[string]bool{}
	// put files the first of secrets that is a key or a token under prefix, unless
	// one is there already; it reports whether one is there now.
	put := func(prefix string, secrets ...secret) bool {
		for _, s := range secrets {
			if !filed[prefix] && bearerToken.MatchString(s.value) {
				c.file(u.Host, prefix, s, true)
				filed[prefix] = true
			}
		}
		return filed[prefix]
	}
	token := func(t userconf.HexToken) string {
		if t.Access != "" && (t.Expires == 0 || t.Expires > now().Unix()) {
			return t.Access
		}
		return ""
	}
	// organizations lists the organizations of a hex.config's repositories.
	organizations := func(cfg userconf.HexConfig) []string {
		var out []string
		for name := range cfg.Repos {
			if org, ok := strings.CutPrefix(name, "hexpm:"); ok && HexOrganization(org) {
				out = append(out, org)
			}
		}
		sort.Strings(out)
		return out
	}
	if put(repos, secret{verbatim: true, value: m.Env("HEX_API_KEY")}) {
		c.notePlain(u)
		return
	}
	for _, org := range organizations(rebar3) {
		put(repos+org+"/", secret{verbatim: true, value: rebar3.Repos["hexpm:"+org].APIKey})
	}
	user := put(repos, secret{verbatim: true, value: mix.APIKey}, secret{bearer: true, value: token(mix.OAuth)},
		secret{verbatim: true, value: rebar3.APIKey}, secret{bearer: true, value: token(rebar3.OAuth)})
	if !user {
		put(repos, secret{verbatim: true, value: m.Env("HEX_REPOS_KEY")})
		for _, cfg := range []userconf.HexConfig{mix, rebar3} {
			for _, org := range organizations(cfg) {
				r := cfg.Repos["hexpm:"+org]
				put(repos+org+"/", secret{verbatim: true, value: r.AuthKey}, secret{bearer: true, value: token(r.OAuth)})
			}
		}
	}
	if len(filed) > 0 {
		c.notePlain(u)
	}
}

// hexOrganization is what Hex accepts as an organization name.
var hexOrganization = regexp.MustCompile(`^[a-z0-9_]+$`)

// HexOrganization reports whether name is a Hex organization name, which is all
// that may go into an API path.
func HexOrganization(name string) bool { return hexOrganization.MatchString(name) }

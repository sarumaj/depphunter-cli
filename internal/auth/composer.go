package auth

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// ---------------------------------------------------------------- Composer

// Composer keeps the credentials for Private Packagist, a Satis, a GitLab or a
// GitHub in auth.json in its home directory, and a pipeline hands them over in
// COMPOSER_AUTH, a JSON document of the same shape. The global config.json may
// hold the same keys under "config".
//
// A repository's own auth.json, beside its composer.json, is not read, and neither
// is the "config" of a composer.json: a repository that could supply a credential
// could also choose where it is sent (REQ-AUTH-012). Read is given the home
// directory only, so nothing under the repository reaches this file.

// composerKinds are the credential kinds, in the order Composer loads them: for a
// host named under several, the later one is the one sent. bitbucket-oauth is
// missing on purpose - it is an OAuth consumer key and secret, which Composer
// exchanges for a token at bitbucket.org before each run.
var composerKinds = []string{"github-oauth", "gitlab-oauth", "gitlab-token", "http-basic", "bearer"}

// readComposer merges config.json's "config", auth.json and COMPOSER_AUTH, host by
// host within each kind and each over the one before, as Composer does, and files
// what results under its hosts.
//
// Implements: REQ-AUTH-016, REQ-AUTH-017
func (c *Store) readComposer(m userconf.Machine) {
	merged := map[string]map[string]json.RawMessage{}
	merge := func(doc map[string]json.RawMessage) {
		for _, kind := range composerKinds {
			var hosts map[string]json.RawMessage
			if json.Unmarshal(doc[kind], &hosts) != nil {
				continue
			}
			if merged[kind] == nil {
				merged[kind] = map[string]json.RawMessage{}
			}
			for host, v := range hosts {
				merged[kind][host] = v
			}
		}
	}
	if directory := m.ComposerHome(); directory != "" {
		if data, err := os.ReadFile(filepath.Join(directory, "config.json")); err == nil {
			var doc struct {
				Config       map[string]json.RawMessage `json:"config"`
				Repositories json.RawMessage            `json:"repositories"`
			}
			if json.Unmarshal(data, &doc) == nil {
				merge(doc.Config)
				c.composerPlain(doc.Repositories)
			}
		}
		if data, err := os.ReadFile(filepath.Join(directory, "auth.json")); err == nil {
			var doc map[string]json.RawMessage
			if json.Unmarshal(data, &doc) == nil {
				merge(doc)
			}
		}
	}
	if v := m.Environment("COMPOSER_AUTH"); v != "" {
		var doc map[string]json.RawMessage
		if json.Unmarshal([]byte(v), &doc) == nil {
			merge(doc)
		}
	}
	// Resolved per host first, so that a host's last kind wins over its others here
	// without touching what another package manager filed for it.
	type credential struct{ bearer, basic string }
	resolved := map[string]credential{}
	for _, kind := range composerKinds {
		for key, raw := range merged[kind] {
			host := composerHost(key)
			if host == "" {
				continue
			}
			var parsed credential
			switch kind {
			case "http-basic":
				var pair struct{ Username, Password string }
				if json.Unmarshal(raw, &pair) == nil && pair.Username != "" {
					parsed.basic = pair.Username + ":" + pair.Password
				}
			case "gitlab-token":
				// A personal, project or group access token alone is taken as a
				// Bearer token, which GitLab's API - its Composer registry included -
				// accepts. With a user name (a deploy token, gitlab-ci-token and a
				// job token) it is a Basic pair.
				var token string
				var pair struct{ Username, Token string }
				if json.Unmarshal(raw, &token) == nil {
					parsed.bearer = token
				} else if json.Unmarshal(raw, &pair) == nil && pair.Token != "" {
					if pair.Username != "" {
						parsed.basic = pair.Username + ":" + pair.Token
					} else {
						parsed.bearer = pair.Token
					}
				}
			default: // bearer, gitlab-oauth, github-oauth: one token per host
				var token string
				if json.Unmarshal(raw, &token) == nil {
					parsed.bearer = token
				}
			}
			if parsed.bearer == "" && parsed.basic == "" {
				continue
			}
			resolved[host] = parsed
			// Composer sends a github.com token to the API as well, which is where
			// the dist archives and the rate limit are.
			if kind == "github-oauth" && host == "github.com" {
				resolved["api.github.com"] = parsed
			}
		}
	}
	for host, credential := range resolved {
		if credential.bearer != "" {
			c.bearer[host] = credential.bearer
		} else {
			c.basic[host] = credential.basic
		}
	}
}

// composerHost is the host an auth.json key names: a host, optionally with a port.
// A URL is tolerated; anything with a path or a credential in it is not a key.
func composerHost(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	if strings.Contains(key, "://") {
		u, err := url.Parse(key)
		if err != nil || u.User != nil {
			return ""
		}
		key = u.Host
	}
	if key == "" || strings.ContainsAny(key, "/@ \t") {
		return ""
	}
	return key
}

// composerPlain notes the repositories of the global config.json that are reached
// over plain http - Composer's secure-http has to be switched off for those - so
// that their credentials may be sent there (REQ-AUTH-011).
func (c *Store) composerPlain(raw json.RawMessage) {
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		var named map[string]json.RawMessage
		if json.Unmarshal(raw, &named) != nil {
			return
		}
		for _, r := range named {
			list = append(list, r)
		}
	}
	for _, r := range list {
		var repository struct {
			URL string `json:"url"`
		}
		if json.Unmarshal(r, &repository) == nil {
			if u, err := url.Parse(strings.TrimSpace(repository.URL)); err == nil && u.Host != "" {
				c.notePlain(u)
			}
		}
	}
}

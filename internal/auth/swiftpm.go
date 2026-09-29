package auth

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// readSwiftPM decides how the credential `swift package-registry login` left in
// the netrc is sent to each registry the user's registries.json names
// (userconf.SwiftPMRegistries), as SwiftPM sends it: a registry whose
// `authentication` entry (by host, with its port) says "token", or that has no
// entry while the netrc's login is "token", gets the netrc's password as a
// Bearer token, limited to the registry's path; any other gets the netrc pair
// as Basic, which the netrc has filed already. What SwiftPM keeps in the macOS
// keychain instead (its default there) cannot be read. A repository's
// registries.json is not read here: it would be choosing how a secret is sent.
//
// Implements: REQ-AUTH-034
func (c *Store) readSwiftPM(m userconf.Machine) {
	name := m.SwiftPMRegistries()
	if name == "" || len(c.netrc) == 0 {
		return
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return
	}
	var doc struct {
		Registries map[string]struct {
			URL string `json:"url"`
		} `json:"registries"`
		Authentication map[string]struct {
			Type string `json:"type"`
		} `json:"authentication"`
		Version int `json:"version"`
	}
	if json.Unmarshal(data, &doc) != nil || doc.Version != 1 {
		return
	}
	kinds := map[string]string{}
	for key, a := range doc.Authentication {
		kinds[strings.ToLower(key)] = a.Type
	}
	for _, r := range doc.Registries {
		u, err := url.Parse(strings.TrimSpace(r.URL))
		if err != nil || u.Host == "" {
			continue
		}
		host := strings.ToLower(u.Host)
		pair, ok := c.netrc[strings.ToLower(u.Hostname())]
		if !ok {
			pair, ok = c.netrc[host]
		}
		if !ok {
			continue
		}
		login, password, _ := strings.Cut(pair, ":")
		kind, configured := kinds[host]
		if kind == "token" || !configured && login == "token" {
			c.file(host, pathPrefix(u.Path), secret{bearer: true, value: password}, true)
		}
	}
}

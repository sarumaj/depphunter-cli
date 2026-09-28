package auth

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang/nuget"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// readNuGet takes the credentials this machine keeps for its NuGet feeds: those of
// its NuGet.Config files, the NuGetPackageSourceCredentials_<source> variables a
// pipeline sets, and the Azure Artifacts credential provider's
// VSS_NUGET_EXTERNAL_FEED_ENDPOINTS.
//
// Implements: REQ-AUTH-004, REQ-AUTH-022
func (c *Store) readNuGet(m userconf.Machine) {
	var files [][]byte
	for _, name := range append(m.NuGetConfigs(), m.NuGetMachineConfigs()...) {
		if data, err := os.ReadFile(name); err == nil {
			files = append(files, data)
		}
	}
	c.readNuGetConfigs(files, m.NuGetCredentialVariables())
	c.readVSSEndpoints(m.Environment("VSS_NUGET_EXTERNAL_FEED_ENDPOINTS"))
}

// readNuGetConfigs files the credential of each enabled source of this machine's
// NuGet configuration (files, closest first, merged the way NuGet merges them) under
// the host of that source: the source's NuGetPackageSourceCredentials_ variable
// (vars, by source name lower-cased) when there is one, as NuGet prefers it, else
// the Username and ClearTextPassword of its <packageSourceCredentials> entry.
//
// Only the sources this machine's files name are matched. A repository's
// nuget.config is never read here: a variable or entry for a source only the
// repository defines would let the repository choose where the password goes
// (internal/index lends one to such a source only when the user vouched for it).
//
// Implements: REQ-AUTH-004, REQ-AUTH-010, REQ-AUTH-022
func (c *Store) readNuGetConfigs(files [][]byte, variables map[string]string) {
	var layers []nuget.ConfigFile
	for _, data := range files {
		if f, ok := nuget.ParseConfig(data); ok {
			layers = append(layers, f)
		}
	}
	s := nuget.Merge(layers)
	for _, f := range s.Feeds {
		if !s.Enabled(f.Key) {
			continue
		}
		key := strings.ToLower(f.Key)
		user, pass, ok := nuget.EnvironmentCredential(variables[key])
		if !ok {
			// A password stored encrypted is encrypted with a key only Windows
			// holds; only the cleartext form is any use here.
			credential := s.Credentials[key]
			user, pass = expand(credential.Username), expand(credential.ClearTextPassword)
			ok = credential.Basic() && user != "" && pass != ""
		}
		if ok {
			c.nugetFeed(f.URL, user+":"+pass)
		}
	}
}

// readVSSEndpoints reads the JSON the Azure Artifacts credential provider takes from
// VSS_NUGET_EXTERNAL_FEED_ENDPOINTS - {"endpointCredentials": [{"endpoint": ...,
// "username": ..., "password": ...}]} - and files each password under its
// endpoint. It is how a pipeline hands a feed's personal access token to NuGet;
// the user name is optional, as Azure Artifacts ignores it.
//
// Implements: REQ-AUTH-022
func (c *Store) readVSSEndpoints(value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	var doc struct {
		EndpointCredentials []struct {
			Endpoint string `json:"endpoint"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"endpointCredentials"`
	}
	if json.Unmarshal([]byte(value), &doc) != nil {
		return
	}
	for _, e := range doc.EndpointCredentials {
		if e.Password != "" {
			c.nugetFeed(e.Endpoint, e.Username+":"+e.Password)
		}
	}
}

// nugetFeed files a feed's credential under the feed's host (and port, when it names
// one) - on pkgs.dev.azure.com,
// which every Azure DevOps organization shares, under the organization's path of it
// only (/<organization>/), which also holds the feed's content addresses.
//
// Implements: REQ-AUTH-004, REQ-AUTH-022
func (c *Store) nugetFeed(feed, pair string) {
	u, err := url.Parse(strings.TrimSpace(feed))
	if err != nil || u.Host == "" {
		return
	}
	c.notePlain(u)
	if prefix := sharedHostPrefix(u); prefix != "" {
		c.scope(u.Host, prefix, pair)
		return
	}
	c.basic[u.Host] = pair
}

// sharedHostPrefix is the path a credential for u is limited to on a host several
// tenants share: /<organization>/ on pkgs.dev.azure.com. It is "" for any other
// host, whose credential is the whole host's.
func sharedHostPrefix(u *url.URL) string {
	if !strings.EqualFold(u.Hostname(), "pkgs.dev.azure.com") {
		return ""
	}
	org, _, _ := strings.Cut(strings.TrimPrefix(u.Path, "/"), "/")
	if org == "" {
		return "/-/" // no organization: a path nothing is served under
	}
	return "/" + org + "/"
}

// scope files a Basic credential for one path prefix of a host.
func (c *Store) scope(host, prefix, pair string) { c.file(host, prefix, secret{value: pair}, true) }

// Lend files a credential for a feed that the repository names but this machine's
// environment supplies the secret for - a paket.dependencies `password: "%PAT%"`, a
// NuGetPackageSourceCredentials_ variable for a source of the repository's
// nuget.config - once the caller has established that the feed is one this machine
// vouches for (internal/index). A host that already has a credential keeps it:
// what this machine's own configuration says comes first.
//
// Implements: REQ-AUTH-023
func (c *Store) Lend(feed, user, password string) {
	if c == nil || password == "" {
		return
	}
	u, err := url.Parse(strings.TrimSpace(feed))
	if err != nil || u.Host == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if prefix := sharedHostPrefix(u); prefix != "" {
		if _, taken := c.scoped[u.Host][prefix]; !taken {
			c.scope(u.Host, prefix, user+":"+password)
		}
		return
	}
	for _, h := range [...]string{u.Host, u.Hostname()} {
		if _, taken := c.basic[h]; taken {
			return
		}
		if _, taken := c.bearer[h]; taken {
			return
		}
	}
	c.basic[u.Host] = user + ":" + password
}

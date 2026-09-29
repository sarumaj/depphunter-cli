package auth

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// ---------------------------------------------------------------- Buf Schema Registry

// bufPublic is the Buf Schema Registry's public host, the one a BUF_TOKEN without a
// host is for.
const bufPublic = "buf.build"

// readBuf takes the tokens buf sends to a Buf Schema Registry from BUF_TOKEN:
// "<token>@<host>" entries, comma-separated, each for its host; a token without a
// host is for buf.build alone here (buf would send it to any registry, and a
// registry is named by the repository's module names). The netrc entries
// `buf registry login` writes are read by BufToken.
//
// Implements: REQ-AUTH-031
func (c *Store) readBuf(m userconf.Machine) {
	value := strings.TrimSpace(m.Environment("BUF_TOKEN"))
	if value == "" {
		return
	}
	c.buf = map[string]string{}
	if !strings.Contains(value, "@") {
		c.buf[bufPublic] = value
		return
	}
	for _, entry := range strings.Split(value, ",") {
		if token, host, ok := strings.Cut(strings.TrimSpace(entry), "@"); ok && token != "" && host != "" {
			c.buf[strings.ToLower(host)] = token
		}
	}
}

// BufToken is the token buf would send to a Buf Schema Registry host: BUF_TOKEN's
// for it, else the password of the netrc's machine entry for it, which is where
// `buf registry login` keeps the token. "" when there is none.
//
// Implements: REQ-AUTH-031, REQ-SUP-072
func (c *Store) BufToken(host string) string {
	if c == nil {
		return ""
	}
	host = strings.ToLower(host)
	c.mu.RLock()
	defer c.mu.RUnlock()
	if token := c.buf[host]; token != "" {
		return token
	}
	if pair, ok := c.netrc[host]; ok {
		_, password, _ := strings.Cut(pair, ":")
		return password
	}
	return ""
}

// ---------------------------------------------------------------- CUE registries

// readCUE takes the tokens `cue login` keeps in logins.json (in cue's configuration
// directory, userconf.CUEConfigDirectory), each sent as a Bearer token to the
// registry host it was stored for, as cue sends it. A registry cue has no login for
// is reached with the container credentials read before (cue reads Docker's
// configuration for those). An expired token is sent as it is: cue would refresh
// it first, and the registry's 401 then says so.
//
// Implements: REQ-AUTH-032
func (c *Store) readCUE(m userconf.Machine) {
	name := m.CUEConfigDirectory()
	if name == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(name, "logins.json"))
	if err != nil {
		return
	}
	var doc struct {
		Registries map[string]struct {
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
		} `json:"registries"`
	}
	if json.Unmarshal(data, &doc) != nil {
		return
	}
	for host, login := range doc.Registries {
		if login.AccessToken != "" && (login.TokenType == "" || strings.EqualFold(login.TokenType, "bearer")) {
			c.file(strings.ToLower(host), "", secret{bearer: true, value: login.AccessToken}, true)
		}
	}
}

// ---------------------------------------------------------------- Puppet Forge

// readR10K takes the token r10k sends to the Forge its configuration names
// (`forge: authorization_token`, the whole Authorization header value, "Bearer
// <token>"), for the paths of `forge: baseurl` alone; without a baseurl it is for
// the public Forge.
//
// Implements: REQ-AUTH-033
func (c *Store) readR10K(m userconf.Machine) {
	for _, name := range m.R10KConfigs() {
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		var doc struct {
			Forge struct {
				BaseURL string `yaml:"baseurl"`
				Token   string `yaml:"authorization_token"`
			} `yaml:"forge"`
		}
		if yaml.Unmarshal(data, &doc) != nil {
			return
		}
		token := strings.TrimSpace(doc.Forge.Token)
		base := strings.TrimSpace(doc.Forge.BaseURL)
		if base == "" {
			base = "https://forgeapi.puppet.com"
		}
		u, err := url.Parse(base)
		if token == "" || err != nil || u.Host == "" || u.Scheme != "https" {
			return
		}
		s := secret{verbatim: true, value: token}
		if !strings.Contains(token, " ") {
			s = secret{bearer: true, value: token}
		}
		c.file(u.Host, rootPrefix(u.Path), s, true)
		return // the first file r10k finds is the one it reads
	}
}

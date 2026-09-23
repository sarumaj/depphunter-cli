// Package auth holds the credentials this machine already has for the registries and
// package indexes it uses, so that a private feed answers instead of returning 401.
//
// Everything here is read from the user's own files and environment, never from the
// repository: a repository that could supply a credential could also choose where it
// is sent. Each credential is filed under the host it was written for and is sent to
// that host and no other.
package auth

import (
	"encoding/base64"
	"encoding/xml"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Store is what this machine holds, by host.
//
// It is filled by Read before it is shared, and afterwards only by FromURL, which
// index discovery calls on every analysis - under --watch, while the previous
// cycle's link checker may still be calling Apply. mu guards those two.
type Store struct {
	mu     sync.RWMutex
	bearer map[string]string // host -> token
	basic  map[string]string // host -> "user:password"
	// plain holds the hosts this machine's own configuration reaches over http://,
	// the only hosts a credential is sent to unencrypted; see Apply.
	plain map[string]bool
}

// Read collects the credentials from the files and variables the package managers of
// this machine keep them in. env is the environment to read; nil reads none.
func Read(home string, env func(string) string) *Store {
	c := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	if env == nil {
		env = func(string) string { return "" }
	}
	if home == "" {
		return c
	}
	if data, err := os.ReadFile(filepath.Join(home, ".npmrc")); err == nil {
		c.readNpmrc(data)
	}
	for _, name := range []string{".netrc", "_netrc"} {
		if data, err := os.ReadFile(filepath.Join(home, name)); err == nil {
			c.readNetrc(data)
		}
	}
	// The two an enterprise actually keeps its feeds behind. Both name a credential
	// by the id of a source declared in the same file, so both have to read the
	// sources as well to know which host it is for - which is the whole reason they
	// are read here rather than fished out of what discover.go already parsed.
	if data, err := os.ReadFile(filepath.Join(home, ".m2", "settings.xml")); err == nil {
		c.readMavenSettings(data)
	}
	for _, name := range []string{
		filepath.Join(home, ".nuget", "NuGet", "NuGet.Config"),
		filepath.Join(home, ".config", "NuGet", "NuGet.Config"),
	} {
		if data, err := os.ReadFile(name); err == nil {
			c.readNuGetConfig(data)
		}
	}
	c.readMachineSources(home, env, exec.LookPath)
	return c
}

// readMavenSettings takes the username and password of every <server> whose id names
// a <mirror> or a <profile>'s <repository>, and files it under that URL's host. It is
// where a developer's Nexus or Artifactory password lives; without it the company
// repository answers 401 and half the dependency tree goes quiet.
func (c *Store) readMavenSettings(data []byte) {
	var doc struct {
		Servers struct {
			Server []struct {
				ID       string `xml:"id"`
				Username string `xml:"username"`
				Password string `xml:"password"`
			} `xml:"server"`
		} `xml:"servers"`
		Mirrors struct {
			Mirror []struct {
				ID  string `xml:"id"`
				URL string `xml:"url"`
			} `xml:"mirror"`
		} `xml:"mirrors"`
		Profiles struct {
			Profile []struct {
				Repositories struct {
					Repository []struct {
						ID  string `xml:"id"`
						URL string `xml:"url"`
					} `xml:"repository"`
				} `xml:"repositories"`
			} `xml:"profile"`
		} `xml:"profiles"`
	}
	if xml.Unmarshal(data, &doc) != nil {
		return
	}
	urls := map[string]string{}
	for _, m := range doc.Mirrors.Mirror {
		urls[m.ID] = m.URL
	}
	for _, p := range doc.Profiles.Profile {
		for _, r := range p.Repositories.Repository {
			if _, ok := urls[r.ID]; !ok {
				urls[r.ID] = r.URL
			}
		}
	}
	for _, s := range doc.Servers.Server {
		user, pass := expand(s.Username), expand(s.Password)
		if user == "" || pass == "" {
			// An encrypted password ({...}) needs the master password from
			// settings-security.xml to be of any use, and guessing is worse than
			// going without: a wrong Authorization header is a 401 either way.
			continue
		}
		if host := c.hostOf(urls[s.ID]); host != "" {
			c.basic[host] = user + ":" + pass
		}
	}
}

// readNuGetConfig takes the credentials a NuGet configuration keeps for its own
// package sources - Azure Artifacts, a Nexus feed, ProGet - and files them under the
// host of the source they name.
func (c *Store) readNuGetConfig(data []byte) {
	type entry struct {
		Key   string `xml:"key,attr"`
		Value string `xml:"value,attr"`
	}
	var doc struct {
		PackageSources struct {
			Add []entry `xml:"add"`
		} `xml:"packageSources"`
		Credentials struct {
			// One element per source, named after it, so the shape is not known in
			// advance: it is read as a list of whatever elements are there.
			Sources []struct {
				XMLName xml.Name
				Add     []entry `xml:"add"`
			} `xml:",any"`
		} `xml:"packageSourceCredentials"`
	}
	if xml.Unmarshal(data, &doc) != nil {
		return
	}
	urls := map[string]string{}
	for _, s := range doc.PackageSources.Add {
		// A source's key is written into the credentials element's tag name, where a
		// space is not allowed, so NuGet replaces each with "_x0020_".
		urls[strings.ReplaceAll(s.Key, " ", "_x0020_")] = s.Value
	}
	for _, src := range doc.Credentials.Sources {
		user, pass := "", ""
		for _, kv := range src.Add {
			switch strings.ToLower(kv.Key) {
			case "username":
				user = expand(kv.Value)
			// A password stored encrypted is encrypted with a key only Windows holds;
			// only the cleartext form is any use here.
			case "cleartextpassword":
				pass = expand(kv.Value)
			}
		}
		if user == "" || pass == "" {
			continue
		}
		if host := c.hostOf(urls[src.XMLName.Local]); host != "" {
			c.basic[host] = user + ":" + pass
		}
	}
}

// expand resolves the environment references these files are allowed to hold, so a
// password can live in the environment rather than on disk: Maven writes ${env.NAME}
// and NuGet %NAME%.
func expand(v string) string {
	v = strings.TrimSpace(v)
	if inner, ok := strings.CutPrefix(v, "${env."); ok {
		if name, ok := strings.CutSuffix(inner, "}"); ok {
			return os.Getenv(name)
		}
	}
	// npm's own form, and how every pipeline writes a token into an .npmrc.
	if inner, ok := strings.CutPrefix(v, "${"); ok {
		if name, ok := strings.CutSuffix(inner, "}"); ok && name != "" && !strings.ContainsAny(name, "${}") {
			return os.Getenv(name)
		}
	}
	if name, ok := strings.CutPrefix(v, "%"); ok {
		if name, ok := strings.CutSuffix(name, "%"); ok && name != "" && !strings.Contains(name, "%") {
			return os.Getenv(name)
		}
	}
	return v
}

// hostOf is the host a source URL names, "" for anything that is not one. A source
// this machine configured over plain http is noted as one to be reached that way.
func (c *Store) hostOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return ""
	}
	c.notePlain(u)
	return u.Hostname()
}

// notePlain records that this machine's own configuration names u, so that if u is a
// plain http:// address a credential for its host may be sent there.
func (c *Store) notePlain(u *url.URL) {
	if u.Scheme != "http" {
		return
	}
	if c.plain == nil {
		c.plain = map[string]bool{}
	}
	c.plain[u.Host] = true
	c.plain[u.Hostname()] = true
}

// readNpmrc reads the per-registry credentials of an npm configuration, which is
// "//registry.example/:<field>=<value>" for each of the four fields npm accepts:
// a bearer token, a base64 "user:password", or the two halves of that pair written
// separately, the password itself base64.
func (c *Store) readNpmrc(data []byte) {
	user, password := map[string]string{}, map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || !strings.HasPrefix(key, "//") {
			continue
		}
		key = strings.TrimSpace(key)
		field := key[strings.LastIndex(key, ":")+1:]
		hostPath := strings.TrimSuffix(strings.TrimPrefix(key, "//"), ":"+field)
		host, _, _ := strings.Cut(hostPath, "/")
		value = expand(strings.Trim(strings.TrimSpace(value), `"`))
		if host == "" || value == "" {
			continue
		}
		switch field {
		case "_authToken":
			c.bearer[host] = value
		case "_auth":
			if pair, err := base64.StdEncoding.DecodeString(value); err == nil && strings.Contains(string(pair), ":") {
				c.basic[host] = string(pair)
			}
		case "username":
			user[host] = value
		case "_password":
			if plain, err := base64.StdEncoding.DecodeString(value); err == nil {
				password[host] = string(plain)
			}
		}
	}
	for host, name := range user {
		if _, taken := c.basic[host]; !taken {
			c.basic[host] = name + ":" + password[host]
		}
	}
}

// readNetrc reads the machine/login/password triples git and curl use.
func (c *Store) readNetrc(data []byte) {
	fields := strings.Fields(string(data))
	machine, login, password := "", "", ""
	flush := func() {
		if machine != "" && login != "" {
			c.basic[machine] = login + ":" + password
		}
		machine, login, password = "", "", ""
	}
	for i := 0; i+1 < len(fields); i += 2 {
		switch fields[i] {
		case "machine":
			flush()
			machine = fields[i+1]
		case "login":
			login = fields[i+1]
		case "password":
			password = fields[i+1]
		}
	}
	flush()
}

// Apply adds the credential for the request's host, if there is one.
//
// Each key is tried with the port and then without it, since a registry reached on a
// port - a Harbor, a Nexus behind one - has a credential of its own, while a netrc
// names a machine and nothing more.
//
// Over plain http a credential is sent only to this machine itself or to a host its
// own configuration names with an http:// address. The request's address may come
// from the repository - the link checker follows every link in its Markdown - and
// a link to http://nexus.corp/ must not put the password for nexus.corp on the wire
// in the clear.
func (c *Store) Apply(req *http.Request) {
	if c == nil {
		return
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if req.URL.Scheme != "https" && !c.plain[req.URL.Host] && !c.plain[req.URL.Hostname()] && !loopback(req.URL.Hostname()) {
		return
	}
	for _, host := range [...]string{req.URL.Host, req.URL.Hostname()} {
		if token, ok := c.bearer[host]; ok {
			req.Header.Set("Authorization", "Bearer "+token)
			return
		}
	}
	for _, host := range [...]string{req.URL.Host, req.URL.Hostname()} {
		if pair, ok := c.basic[host]; ok {
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(pair)))
			return
		}
	}
}

// loopback reports whether host is this machine, where plain http leaves nothing on
// the network: a registry run locally for development is usually reached that way.
func loopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

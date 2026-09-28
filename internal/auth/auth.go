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
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
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
	// registries holds the container registries this machine's own configuration
	// names, with or without a credential; see Registry.
	registries map[string]bool
	// terraform holds the Terraform registry hosts this machine's CLI configuration
	// names; see TerraformHost.
	terraform map[string]bool
	// scoped holds the credentials that serve one path of a host and no other:
	// host -> path prefix (ending in "/") -> credential. A host many
	// organizations share - Azure Artifacts' pkgs.dev.azure.com - gets each
	// organization's credential this way, and so does an npm registry that
	// lives under a path, as GitLab's per-project registries do.
	scoped map[string]map[string]secret
}

// secret is a credential of scoped: a Bearer token, a Basic "user:password", or
// (verbatim) the whole Authorization header value, as Cargo sends its tokens.
type secret struct {
	bearer   bool
	verbatim bool
	value    string
}

// header is the Authorization header that sends it.
func (s secret) header() string {
	if s.verbatim {
		return s.value
	}
	if s.bearer {
		return "Bearer " + s.value
	}
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(s.value))
}

// Read collects the credentials from the files and variables the package managers of
// this machine keep them in. env is the environment to read; nil reads none.
//
// Implements: REQ-AUTH-014
func Read(home string, env func(string) string) *Store {
	m := userconf.New(home, env)
	m.Environ = environ
	return readMachine(m)
}

// readMachine is Read for a given machine, whose platform a test may choose. Every
// file is found through m, the way the tool that writes it finds it
// (internal/userconf).
//
// Implements: REQ-AUTH-020
func readMachine(m userconf.Machine) *Store {
	c := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	if m.Home == "" {
		return c
	}
	c.readNpm(m)
	c.readYarn(m)
	c.readBun(m)
	if data, err := os.ReadFile(m.Netrc()); err == nil {
		c.readNetrc(data)
	}
	// The two an enterprise actually keeps its feeds behind. Both name a credential
	// by the id of a source declared in the same file, so both have to read the
	// sources as well to know which host it is for - which is the whole reason they
	// are read here rather than fished out of what discover.go already parsed.
	c.readMaven(m)
	c.readNuGet(m)
	c.readMachineSources(m, exec.LookPath)
	return c
}

// readMavenSettings takes the username and password of every <server> whose id names
// a <mirror> or a <profile>'s <repository> - active or not, as Maven matches a server
// to any repository by id - or a repository of the Clojure CLI's user deps.edn
// (repos, by name), and files it under that URL's host. It is where a developer's
// Nexus or Artifactory password lives; without it the company repository answers
// 401 and half the dependency tree goes quiet. The files are the user's settings
// first, then the installation's; the first to define an id is the one used.
//
// A repository a project declares (a POM's, a project deps.edn's) is not matched: the
// repository would then be choosing where the password is sent.
//
// Implements: REQ-AUTH-003, REQ-AUTH-010
func (c *Store) readMavenSettings(files [][]byte, repos map[string]string) {
	type doc struct {
		Servers []struct {
			ID       string `xml:"id"`
			Username string `xml:"username"`
			Password string `xml:"password"`
		} `xml:"servers>server"`
		Mirrors []struct {
			ID  string `xml:"id"`
			URL string `xml:"url"`
		} `xml:"mirrors>mirror"`
		Profiles []struct {
			Repositories []struct {
				ID  string `xml:"id"`
				URL string `xml:"url"`
			} `xml:"repositories>repository"`
		} `xml:"profiles>profile"`
	}
	var docs []doc
	for _, data := range files {
		var d doc
		if xml.Unmarshal(data, &d) == nil {
			docs = append(docs, d)
		}
	}
	urls := map[string]string{}
	name := func(id, u string) {
		if _, ok := urls[id]; !ok && id != "" {
			urls[id] = u
		}
	}
	for _, d := range docs {
		for _, m := range d.Mirrors {
			name(strings.TrimSpace(m.ID), m.URL)
		}
	}
	for _, d := range docs {
		for _, p := range d.Profiles {
			for _, r := range p.Repositories {
				name(strings.TrimSpace(r.ID), r.URL)
			}
		}
	}
	for _, id := range slices.Sorted(maps.Keys(repos)) {
		name(id, repos[id])
	}
	done := map[string]bool{}
	for _, d := range docs {
		for _, s := range d.Servers {
			id := strings.TrimSpace(s.ID)
			user, pass := expand(s.Username), expand(s.Password)
			if done[id] {
				continue
			}
			done[id] = true
			if user == "" || pass == "" || mavenEncrypted(pass) {
				// An encrypted password ({...}) needs the master password from
				// settings-security.xml to be of any use, and guessing is worse
				// than going without: a wrong Authorization header is a 401
				// either way.
				continue
			}
			if host := c.hostOf(urls[id]); host != "" {
				c.basic[host] = user + ":" + pass
			}
		}
	}
}

// mavenEncrypted reports whether a Maven password is in the encrypted form: a
// {...} with something inside, which Maven finds anywhere in the value so that a
// note may sit around it.
func mavenEncrypted(pass string) bool {
	i := strings.Index(pass, "{")
	return i >= 0 && strings.LastIndex(pass, "}") > i+1
}

// expand resolves the environment references these files are allowed to hold, so a
// password can live in the environment rather than on disk: Maven writes ${env.NAME}
// and NuGet %NAME%.
//
// Implements: REQ-AUTH-009
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

// readNpm reads the per-registry credentials npm itself would use: those of the
// global npmrc, then the user's, then the environment's npm_config_//host/:field
// variables, each key replacing the same key of the one before.
//
// Implements: REQ-AUTH-001, REQ-AUTH-020
func (c *Store) readNpm(m userconf.Machine) {
	keys := map[string]string{}
	for _, name := range []string{m.NpmGlobalConfig(), m.NpmUserConfig()} {
		if name == "" {
			continue
		}
		if data, err := os.ReadFile(name); err == nil {
			for k, v := range npmKeys(data) {
				keys[k] = v
			}
		}
	}
	for k, v := range m.NpmEnv() {
		if strings.HasPrefix(k, "//") {
			keys[k] = v
		}
	}
	c.applyNpm(keys)
}

// readNpmrc reads the per-registry credentials of one npm configuration.
//
// Implements: REQ-AUTH-001
func (c *Store) readNpmrc(data []byte) { c.applyNpm(npmKeys(data)) }

// npmKeys are the "//registry.example/:<field>=<value>" lines of an npm
// configuration, by key.
func npmKeys(data []byte) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if key = strings.TrimSpace(key); ok && strings.HasPrefix(key, "//") {
			out[key] = value
		}
	}
	return out
}

// applyNpm files the credentials of npm's per-registry keys, for each of the four
// fields npm accepts: a bearer token, a base64 "user:password", or the two halves of
// that pair written separately, the password itself base64.
//
// A key names a registry by host and path, as npm does: //host/:field serves the
// whole host, and //host/some/path/:field only the URLs under that path, the
// longest such path winning (see Apply) - two GitLab projects' registries on one
// host keep their own tokens, and neither goes to anything else on the host.
//
// Implements: REQ-AUTH-001, REQ-AUTH-025
func (c *Store) applyNpm(keys map[string]string) {
	user, password := map[string]string{}, map[string]string{}
	for _, key := range slices.Sorted(maps.Keys(keys)) {
		value := keys[key]
		field := key[strings.LastIndex(key, ":")+1:]
		hostPath := strings.TrimSuffix(strings.TrimPrefix(key, "//"), ":"+field)
		host, _, _ := strings.Cut(hostPath, "/")
		value = expand(strings.Trim(strings.TrimSpace(value), `"`))
		if host == "" || value == "" {
			continue
		}
		prefix := pathPrefix(hostPath[len(host):])
		switch field {
		case "_authToken":
			c.file(host, prefix, secret{bearer: true, value: value}, true)
		case "_auth":
			if pair, err := base64.StdEncoding.DecodeString(value); err == nil && strings.Contains(string(pair), ":") {
				c.file(host, prefix, secret{value: string(pair)}, true)
			}
		case "username":
			user[hostPath] = value
		case "_password":
			if plain, err := base64.StdEncoding.DecodeString(value); err == nil {
				password[hostPath] = string(plain)
			}
		}
	}
	for hostPath, name := range user {
		host, _, _ := strings.Cut(hostPath, "/")
		c.file(host, pathPrefix(hostPath[len(host):]), secret{value: name + ":" + password[hostPath]}, false)
	}
}

// pathPrefix is the path a registry's credential is limited to: "" for a registry
// at the root of its host, whose credential is the host's, else the path ending in
// "/", so that /npm does not also cover /npm-private.
func pathPrefix(p string) string {
	p = strings.TrimRight(p, "/")
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p + "/"
}

// file files a credential for a host, or for one path prefix of it. Without
// replace, a credential already filed for the same host and prefix is kept: the
// bearer and basic kinds of a host count as one.
func (c *Store) file(host, prefix string, s secret, replace bool) {
	if prefix != "" {
		if _, taken := c.scoped[host][prefix]; taken && !replace {
			return
		}
		if c.scoped == nil {
			c.scoped = map[string]map[string]secret{}
		}
		if c.scoped[host] == nil {
			c.scoped[host] = map[string]secret{}
		}
		c.scoped[host][prefix] = s
		return
	}
	_, hasBearer := c.bearer[host]
	_, hasBasic := c.basic[host]
	if (hasBearer || hasBasic) && !replace {
		return
	}
	if s.bearer {
		c.bearer[host] = s.value
	} else {
		c.basic[host] = s.value
	}
}

// readNetrc reads the machine/login/password triples git and curl use.
//
// Implements: REQ-AUTH-002
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
//
// Implements: REQ-AUTH-011
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
		if s, ok := scopedSecret(c.scoped[host], req.URL.Path); ok {
			req.Header.Set("Authorization", s.header())
			return
		}
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

// Authorizes reports whether Apply would send a credential with a request for raw.
//
// Implements: REQ-SUP-047
func (c *Store) Authorizes(raw string) bool {
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return false
	}
	c.Apply(req)
	return req.Header.Get("Authorization") != ""
}

// scopedSecret is the credential of the longest prefix of path among a host's
// scoped ones, as npm takes the registry key with the longest matching path.
func scopedSecret(prefixes map[string]secret, path string) (s secret, ok bool) {
	best := -1
	for prefix, p := range prefixes {
		if strings.HasPrefix(path, prefix) && len(prefix) > best {
			best, s, ok = len(prefix), p, true
		}
	}
	return s, ok
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

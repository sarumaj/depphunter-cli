package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// ---------------------------------------------------------------- container registries

// readDockerConfig takes the credentials a container registry configuration holds, and
// the names of the helpers it keeps the rest in.
//
// A stored credential is base64 "user:password" under the registry it belongs to. Most
// installations no longer store one: the config names a helper instead, per registry
// (credHelpers) or for all of them (credsStore), and the credential has to be asked
// for. runHelper does that.
//
// An entry may hold an identity token instead (`identitytoken`, as `docker login`
// stores what a registry such as Azure Container Registry hands out for an Entra
// login): a refresh token, not a password, filed apart (see IdentityToken). A helper
// answering with the user name `<token>` hands one back the same way.
//
// Implements: REQ-AUTH-005, REQ-AUTH-006, REQ-AUTH-030
func (c *Store) readDockerConfig(data []byte, lookPath func(string) (string, error)) {
	var doc struct {
		Auths map[string]struct {
			Auth     string `json:"auth"`
			Username string `json:"username"`
			Password string `json:"password"`
			Identity string `json:"identitytoken"`
		} `json:"auths"`
		CredentialsStore  string            `json:"credsStore"`
		CredentialHelpers map[string]string `json:"credHelpers"`
	}
	if json.Unmarshal(data, &doc) != nil {
		return
	}
	if c.registries == nil {
		c.registries = map[string]bool{}
	}
	for registry := range doc.CredentialHelpers {
		if host := registryHost(registry); host != "" {
			c.registries[host] = true
		}
	}
	// What this file holds, filed over what an earlier file held only once it is
	// complete: the files are read from the last to be consulted to the first, and a
	// helper this file names must run even when a later file stored a credential.
	got, identity := map[string]string{}, map[string]string{}
	for registry, entry := range doc.Auths {
		host := registryHost(registry)
		if host == "" {
			continue
		}
		c.registries[host] = true
		switch {
		case entry.Identity != "":
			identity[host] = entry.Identity
		case entry.Auth != "":
			if pair, err := base64.StdEncoding.DecodeString(entry.Auth); err == nil && strings.Contains(string(pair), ":") {
				got[host] = string(pair)
			}
		case entry.Username != "":
			got[host] = entry.Username + ":" + entry.Password
		}
	}
	// A helper is asked only for a registry the configuration actually names, so the
	// set of programs that can run is the set the user's own file lists.
	helpers := map[string]string{}
	for registry, helper := range doc.CredentialHelpers {
		helpers[registry] = helper
	}
	if doc.CredentialsStore != "" {
		for registry := range doc.Auths {
			if _, ok := helpers[registry]; !ok {
				helpers[registry] = doc.CredentialsStore
			}
		}
	}
	for registry, helper := range helpers {
		host := registryHost(registry)
		if host == "" || got[host] != "" || identity[host] != "" {
			continue
		}
		user, secret, ok := runHelper(helper, registry, lookPath)
		switch {
		case ok && user == identityUser:
			identity[host] = secret
		case ok:
			got[host] = user + ":" + secret
		}
	}
	for host, pair := range got {
		c.basic[host] = pair
		delete(c.identity, host)
	}
	for host, token := range identity {
		if c.identity == nil {
			c.identity = map[string]string{}
		}
		c.identity[host] = token
	}
}

// identityUser is the user name under which a credential helper hands back an
// identity token rather than a password.
const identityUser = "<token>"

// IdentityToken is the identity token this machine's container configuration holds
// for a registry host ("" for none): an OAuth2 refresh token, which is sent to
// nothing but that registry's own token endpoint, over https, to be exchanged for a
// pull token (internal/index, ociToken). It is never an Authorization header.
//
// Implements: REQ-AUTH-030
func (c *Store) IdentityToken(host string) string {
	if c == nil {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.identity[host]
}

// Registry reports whether this machine's container configuration names a registry
// host, whether or not it holds a credential for it: a registry somebody here logged
// in to, or set a helper up for, is one this machine knows.
//
// Implements: REQ-SUP-026
func (c *Store) Registry(host string) bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.registries[host]
}

// helperName is what may follow "docker-credential-". A helper is named by a
// configuration file, and a name is all it may contribute: anything that could reach
// outside PATH - a separator, a relative segment, an extension - is not a name.
//
// Implements: REQ-AUTH-007
var helperName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// helperTimeout bounds one helper. Some of them talk to a cloud service, and none of
// them is worth waiting on: a credential that does not arrive means an anonymous
// request, which is what would have happened anyway.
//
// Implements: REQ-AUTH-007
const helperTimeout = 10 * time.Second

// runHelper asks a credential helper for one registry, the way docker login does:
// "docker-credential-<name> get" with the registry on stdin and a JSON credential on
// stdout.
//
// This is the one place depphunter runs a program it was not told to run on the
// command line, so the name is checked against helperName and resolved on PATH only -
// a configuration naming "../../evil" or an absolute path gets nothing.
//
// Implements: REQ-AUTH-006, REQ-AUTH-007
func runHelper(name, registry string, lookPath func(string) (string, error)) (user, secret string, ok bool) {
	if !helperName.MatchString(name) {
		return "", "", false
	}
	path, err := lookPath("docker-credential-" + name)
	if err != nil {
		return "", "", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), helperTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, path, "get")
	command.Stdin = strings.NewReader(registry)
	out, err := command.Output()
	if err != nil {
		return "", "", false
	}
	var answer struct{ Username, Secret string }
	if json.Unmarshal(out, &answer) != nil || answer.Secret == "" {
		return "", "", false
	}
	// A refresh token comes back under the user name identityUser and is not a
	// password: the caller files it as an identity token.
	return answer.Username, answer.Secret, true
}

// registryHost is the host a registry key names. The keys are historical: Docker Hub
// is written as a v1 API URL, and the rest are bare hosts or URLs.
//
// Implements: REQ-AUTH-005
func registryHost(registry string) string {
	registry = strings.TrimSpace(registry)
	if registry == "" {
		return ""
	}
	if strings.Contains(registry, "://") {
		if u, err := url.Parse(registry); err == nil && u.Host != "" {
			registry = u.Host
		}
	}
	registry, _, _ = strings.Cut(registry, "/")
	if registry == "index.docker.io" || registry == "docker.io" {
		return "registry-1.docker.io"
	}
	return registry
}

// ---------------------------------------------------------------- crates.io

// cargoFile is what Cargo's config.toml and credentials.toml say about registries.
// Cargo reads the two as one configuration, credentials.toml over config.toml, so
// either may hold any of these keys. A credential-provider is a string or a list
// (a command and its arguments), hence any.
type cargoFile struct {
	Registry struct {
		Token     string
		Provider  any   `toml:"credential-provider"`
		Providers []any `toml:"global-credential-providers"`
	}
	Registries map[string]struct {
		Index    string
		Token    string
		Provider any `toml:"credential-provider"`
	}
}

// cargoRegistry is one registry as Cargo would authenticate to it.
type cargoRegistry struct {
	index, token string
	provider     []string // the registry's own credential-provider, if set
}

// cargoRegistries is every registry Cargo knows, by name in the form
// userconf.CargoRegistryName gives, with "" for crates.io: its index from
// config.toml or CARGO_REGISTRIES_<NAME>_INDEX (indexes), and its token from
// config.toml, then credentials.toml, then CARGO_REGISTRIES_<NAME>_TOKEN
// (CARGO_REGISTRY_TOKEN for crates.io), each over the one before, as Cargo takes
// them. global is registry.global-credential-providers, nil when unset.
func cargoRegistries(files [][]byte, indexes map[string]string, environment func(string) string) (out map[string]*cargoRegistry, global []string) {
	out = map[string]*cargoRegistry{"": {}}
	get := func(name string) *cargoRegistry {
		if out[name] == nil {
			out[name] = &cargoRegistry{}
		}
		return out[name]
	}
	for _, data := range files {
		var doc cargoFile
		if _, err := toml.Decode(string(data), &doc); err != nil {
			continue
		}
		crates := out[""]
		if doc.Registry.Token != "" {
			crates.token = doc.Registry.Token
		}
		if p := cargoProvider(doc.Registry.Provider); p != nil {
			crates.provider = p
		}
		if doc.Registry.Providers != nil {
			global = nil
			for _, p := range doc.Registry.Providers {
				if s, ok := p.(string); ok {
					global = append(global, s)
				}
			}
		}
		for name, r := range doc.Registries {
			registry := get(userconf.CargoRegistryName(name))
			if r.Index != "" {
				registry.index = r.Index
			}
			if r.Token != "" {
				registry.token = r.Token
			}
			if p := cargoProvider(r.Provider); p != nil {
				registry.provider = p
			}
		}
	}
	for name, index := range indexes {
		get(name).index = index
	}
	if environment == nil {
		return out, global
	}
	for name, registry := range out {
		prefix := "CARGO_REGISTRIES_" + name + "_"
		if name == "" {
			prefix = "CARGO_REGISTRY_"
		}
		if token := environment(prefix + "TOKEN"); token != "" {
			registry.token = token
		}
		if p := strings.Fields(environment(prefix + "CREDENTIAL_PROVIDER")); len(p) > 0 {
			registry.provider = p
		}
	}
	if list := environment("CARGO_REGISTRY_GLOBAL_CREDENTIAL_PROVIDERS"); list != "" {
		global = strings.Fields(list)
	}
	return out, global
}

// cargoProvider is a credential-provider value as a command line: a string is split
// on whitespace, a list is taken as it is.
func cargoProvider(v any) []string {
	switch v := v.(type) {
	case string:
		if f := strings.Fields(v); len(f) > 0 {
			return f
		}
	case []any:
		var out []string
		for _, s := range v {
			if s, ok := s.(string); ok {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// plaintext reports whether Cargo would send the token it has in its files or
// environment. Only the cargo:token provider does: a registry whose
// credential-provider is another one - an OS keychain, cargo:token-from-stdout,
// cargo:paseto, a plugin - gets its credential from that provider, which
// depphunter does not run. Without a provider of its own, a registry uses the
// global list, cargo:token alone unless configured.
func (r *cargoRegistry) plaintext(global []string) bool {
	if r.provider != nil {
		return len(r.provider) == 1 && r.provider[0] == "cargo:token"
	}
	return global == nil || slices.Contains(global, "cargo:token")
}

// cargoToken is a token Cargo would accept: printable ASCII and tabs, anything else
// being refused by Cargo and unsendable in a header.
func cargoToken(token string) bool {
	for _, r := range token {
		if r != '\t' && (r < ' ' || r > '~') {
			return false
		}
	}
	return token != ""
}

// readCargo takes the tokens Cargo keeps for its registries (cargoRegistries): the
// named registries' are filed for the path of each one's index, since a host such as
// Artifactory serves several registries, each with its own token; crates.io's for
// crates.io alone - never for its index, which Cargo reads anonymously, and never
// for a registry replacing it, which has its own.
//
// Cargo puts the token into the Authorization header as it is written. A registry
// that wants a scheme has it written into the token (Artifactory documents
// "Bearer <token>"); crates.io and most others take the token bare. For an
// alternative registry Cargo sends it to the index only when the registry's
// config.json says auth-required; depphunter sends it to the index whenever there
// is one, which such a registry needs and any other ignores.
//
// Implements: REQ-AUTH-008
func (c *Store) readCargo(files [][]byte, indexes map[string]string, environment func(string) string) {
	registries, global := cargoRegistries(files, indexes, environment)
	for name, registry := range registries {
		if !cargoToken(registry.token) || !registry.plaintext(global) {
			continue
		}
		if name == "" {
			c.file("crates.io", "/", secret{verbatim: true, value: registry.token}, true)
			continue
		}
		u, err := url.Parse(strings.TrimPrefix(strings.TrimPrefix(registry.index, "sparse+"), "registry+"))
		if err != nil || u.Host == "" {
			continue
		}
		prefix := pathPrefix(u.Path)
		if prefix == "" {
			prefix = "/"
		}
		c.file(u.Host, prefix, secret{verbatim: true, value: registry.token}, true)
		c.notePlain(u)
	}
}

// ---------------------------------------------------------------- URL credentials

// FromURL takes a credential written into an index URL - the form pip and Cargo
// mirrors are usually configured with - and files it under that URL's host. It answers
// with the URL as it should be shown and stored: without the credential.
//
// Stripping is not optional. The index a package resolves from is drawn on the map,
// named in the side panel and written into every export, and an export is a file the
// documentation suggests sharing.
//
// Implements: REQ-AUTH-012, REQ-AUTH-013
func (c *Store) FromURL(raw string, trusted bool) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	if trusted && c != nil {
		// Only this machine's own configuration may supply one. A repository that
		// could would be choosing both the credential and where it is sent.
		password, _ := u.User.Password()
		c.mu.Lock()
		defer c.mu.Unlock()
		if user := u.User.Username(); user != "" {
			c.basic[u.Host] = user + ":" + password
			c.notePlain(u)
		}
	}
	u.User = nil
	return u.String()
}

// ---------------------------------------------------------------- reading them all

// readMachineSources adds the credential files that are read wholesale.
//
// Implements: REQ-AUTH-020
func (c *Store) readMachineSources(m userconf.Machine, lookPath func(string) (string, error)) {
	var cargo [][]byte
	for _, name := range [...]string{"config", "credentials"} {
		if data, err := os.ReadFile(m.CargoFile(name)); err == nil {
			cargo = append(cargo, data)
		}
	}
	c.readCargo(cargo, m.CargoRegistries(), m.Environment)
	c.readTerraform(m.Home, m.Environment)
	c.readComposer(m)
	c.readBundler(m)
	c.readJVM(m)
	c.readPython(m)
	c.readPub(m)
	c.readHex(m)
	// The first file to hold a registry's credential is the one used, so they are
	// read from the last to the first, each over the one before.
	files := m.ContainerAuthFiles()
	for i := len(files) - 1; i >= 0; i-- {
		if data, err := os.ReadFile(files[i]); err == nil {
			c.readDockerConfig(data, lookPath)
		}
	}
}

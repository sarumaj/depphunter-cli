package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"regexp"
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
// Implements: REQ-AUTH-005, REQ-AUTH-006
func (c *Store) readDockerConfig(data []byte, lookPath func(string) (string, error)) {
	var doc struct {
		Auths map[string]struct {
			Auth     string `json:"auth"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"auths"`
		CredsStore  string            `json:"credsStore"`
		CredHelpers map[string]string `json:"credHelpers"`
	}
	if json.Unmarshal(data, &doc) != nil {
		return
	}
	if c.registries == nil {
		c.registries = map[string]bool{}
	}
	for registry := range doc.CredHelpers {
		if host := registryHost(registry); host != "" {
			c.registries[host] = true
		}
	}
	// What this file holds, filed over what an earlier file held only once it is
	// complete: the files are read from the last to be consulted to the first, and a
	// helper this file names must run even when a later file stored a credential.
	got := map[string]string{}
	for registry, entry := range doc.Auths {
		host := registryHost(registry)
		if host == "" {
			continue
		}
		c.registries[host] = true
		switch {
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
	for registry, helper := range doc.CredHelpers {
		helpers[registry] = helper
	}
	if doc.CredsStore != "" {
		for registry := range doc.Auths {
			if _, ok := helpers[registry]; !ok {
				helpers[registry] = doc.CredsStore
			}
		}
	}
	for registry, helper := range helpers {
		host := registryHost(registry)
		if host == "" || got[host] != "" {
			continue
		}
		if user, secret, ok := runHelper(helper, registry, lookPath); ok {
			got[host] = user + ":" + secret
		}
	}
	for host, pair := range got {
		c.basic[host] = pair
	}
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
	cmd := exec.CommandContext(ctx, path, "get")
	cmd.Stdin = strings.NewReader(registry)
	out, err := cmd.Output()
	if err != nil {
		return "", "", false
	}
	var answer struct{ Username, Secret string }
	if json.Unmarshal(out, &answer) != nil || answer.Secret == "" {
		return "", "", false
	}
	// A refresh token comes back under this user name and is not a password; the
	// registry takes it as an identity token instead, which is beyond what a link
	// check or a manifest read needs.
	if answer.Username == "<token>" {
		return "", "", false
	}
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

// cargoIndexes is the index URL of each registry Cargo knows, by name in the form
// userconf.CargoRegistryName gives: the [registries] of config.toml, and over them
// the CARGO_REGISTRIES_<NAME>_INDEX variables.
func cargoIndexes(config []byte, env map[string]string) map[string]string {
	var doc struct {
		Registries map[string]struct{ Index string }
	}
	out := map[string]string{}
	if _, err := toml.Decode(string(config), &doc); err == nil {
		for name, r := range doc.Registries {
			if r.Index != "" {
				out[userconf.CargoRegistryName(name)] = r.Index
			}
		}
	}
	for name, index := range env {
		out[name] = index
	}
	return out
}

// readCargoCredentials takes the tokens Cargo keeps for its registries, which are
// named rather than addressed: the index URL for each name is in config.toml beside
// them, or in the environment (cargoIndexes).
//
// Implements: REQ-AUTH-008
func (c *Store) readCargoCredentials(credentials []byte, indexes map[string]string) {
	var creds struct {
		Registry   struct{ Token string }
		Registries map[string]struct{ Token string }
	}
	if _, err := toml.Decode(string(credentials), &creds); err != nil {
		return
	}
	for name, entry := range creds.Registries {
		if entry.Token != "" {
			c.cargo(indexes[userconf.CargoRegistryName(name)], entry.Token)
		}
	}
	if creds.Registry.Token != "" {
		c.bearer["crates.io"] = creds.Registry.Token
		c.bearer["index.crates.io"] = creds.Registry.Token
	}
}

// readCargoEnv takes the tokens Cargo reads from the environment, which is how a
// pipeline supplies them. The name is upper-cased with hyphens turned into
// underscores, and there is no way back from the variable to the name, so the
// environment is asked for the registries config.toml and the environment define.
//
// Implements: REQ-AUTH-008
func (c *Store) readCargoEnv(env func(string) string, indexes map[string]string) {
	for name, index := range indexes {
		if token := env("CARGO_REGISTRIES_" + name + "_TOKEN"); token != "" {
			c.cargo(index, token)
		}
	}
	if token := env("CARGO_REGISTRY_TOKEN"); token != "" {
		c.bearer["crates.io"] = token
		c.bearer["index.crates.io"] = token
	}
}

// cargo files a registry's token under the host of its index.
func (c *Store) cargo(index, token string) {
	if index == "" {
		return
	}
	if u, err := url.Parse(strings.TrimPrefix(index, "sparse+")); err == nil && u.Host != "" {
		c.bearer[u.Host] = token
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
	cargoConfig, _ := os.ReadFile(m.CargoFile("config"))
	indexes := cargoIndexes(cargoConfig, m.CargoRegistries())
	if data, err := os.ReadFile(m.CargoFile("credentials")); err == nil {
		c.readCargoCredentials(data, indexes)
	}
	c.readCargoEnv(m.Env, indexes)
	c.readTerraform(m.Home, m.Env)
	c.readComposer(m)
	c.readBundler(m)
	c.readJVM(m)
	c.readPython(m)
	// The first file to hold a registry's credential is the one used, so they are
	// read from the last to the first, each over the one before.
	files := m.ContainerAuthFiles()
	for i := len(files) - 1; i >= 0; i-- {
		if data, err := os.ReadFile(files[i]); err == nil {
			c.readDockerConfig(data, lookPath)
		}
	}
}

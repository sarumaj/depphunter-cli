package auth

import (
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// ---------------------------------------------------------------- Bundler

// Bundler keeps a gem server's credential as a setting named after the server:
// `bundle config set --global gems.example.com user:password` writes
// BUNDLE_GEMS__EXAMPLE__COM to the user's config file, and a pipeline sets the
// same name in the environment. The name is the host upper-cased with "." as
// "__" and "-" as "___"; a source URL may stand for the host
// (BUNDLE_HTTPS://GEMS__EXAMPLE__COM/PRIVATE/), which only the YAML file can hold.
//
// The application's config (.bundle/config beside the Gemfile, or the directory
// BUNDLE_APP_CONFIG names) is not read: it belongs to the repository, and a
// repository that could supply a credential could also choose where it is sent
// (REQ-AUTH-012). Read is given the home directory only.

// environ lists the process environment. The names of Bundler's variables cannot be
// derived from anything else, so they are listed from here; their values are still
// read through the environment Read was given, and a nil environment reads none.
var environ = os.Environ

// bundlerHostName is what a decoded host key may be: a host name, a port after it
// only in a source URL.
var bundlerHostName = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]+)?$`)

// bundlerSetting reports whether a decoded key is one of Bundler's own dotted
// settings rather than a server: build.<gem> and local.<gem>, the gem.* defaults of
// bundle gem, and github.https. They encode like a two-label host.
func bundlerSetting(host string) bool {
	label, rest, _ := strings.Cut(host, ".")
	if strings.Contains(rest, ".") {
		return false
	}
	switch label {
	case "build", "local":
		return true
	case "gem":
		return strings.Contains(" bundle changelog ci coc linter mit rubocop test ", " "+rest+" ")
	}
	return host == "github.https"
}

// readBundler files the credentials of the user's Bundler config and of the
// BUNDLE_<HOST> variables, the environment over the file as in Bundler. A host key
// is sent in preference to a source URL key on the same host, since the store
// answers by host and the host key is the one Bundler sends to any source there; of
// several URL keys on one host, the first in sorted order. A URL key with a port is
// filed under host:port, which Apply tries before the host alone.
//
// Implements: REQ-AUTH-018, REQ-AUTH-019, REQ-AUTH-011
func (c *Store) readBundler(m userconf.Machine) {
	environment := m.Environment
	settings := map[string]string{}
	if data, err := os.ReadFile(m.BundlerConfig()); err == nil {
		var doc map[string]any
		if yaml.Unmarshal(data, &doc) == nil {
			for k, v := range doc {
				if s, ok := v.(string); ok {
					settings[strings.ToUpper(k)] = s
				}
			}
		}
	}
	for _, keyValue := range environ() {
		name, _, _ := strings.Cut(keyValue, "=")
		if !strings.HasPrefix(name, "BUNDLE_") {
			continue
		}
		if v := environment(name); v != "" {
			settings[strings.ToUpper(name)] = v
		}
	}
	hostKey, urlKey := map[string]string{}, map[string]string{}
	keys := make([]string, 0, len(settings))
	for k := range settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := strings.TrimSpace(settings[key])
		name := strings.TrimPrefix(key, "BUNDLE_")
		if strings.Contains(value, "://") {
			// A mirror (BUNDLE_MIRROR__RUBYGEMS__ORG, mirror.https://...), never a
			// credential. A mirror this machine reaches over plain http may be sent
			// its host's credential there.
			if strings.HasPrefix(name, "MIRROR__") {
				if u, err := url.Parse(value); err == nil && u.Host != "" {
					c.notePlain(u)
				}
			}
			continue
		}
		pair := bundlerPair(value)
		if pair == "" {
			continue
		}
		if scheme, rest, ok := strings.Cut(name, "://"); ok {
			scheme = strings.ToLower(scheme)
			encoded, _, _ := strings.Cut(rest, "/")
			host := bundlerHost(encoded)
			if (scheme != "https" && scheme != "http") || !bundlerHostName.MatchString(host) {
				continue
			}
			if _, taken := urlKey[host]; !taken {
				urlKey[host] = pair
			}
			c.notePlain(&url.URL{Scheme: scheme, Host: host})
			continue
		}
		host := bundlerHost(name)
		if !strings.Contains(host, ".") || strings.Contains(host, ":") || !bundlerHostName.MatchString(host) || bundlerSetting(host) {
			continue
		}
		hostKey[host] = pair
	}
	for host, pair := range urlKey {
		c.basic[host] = pair
	}
	for host, pair := range hostKey {
		c.basic[host] = pair
	}
}

// bundlerHost decodes the host part of a setting name: "___" is "-", "__" is ".".
func bundlerHost(encoded string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(encoded, "___", "-"), "__", "."))
}

// bundlerPair is a setting's value as the user:password Bundler sends. Bundler puts
// the value into the source URL's user information and sends it as Basic
// credentials, unescaping both halves; a value without a colon is a token alone,
// sent as the user name with an empty password.
func bundlerPair(value string) string {
	user, password, _ := strings.Cut(value, ":")
	if u, err := url.QueryUnescape(user); err == nil {
		user = u
	}
	if p, err := url.QueryUnescape(password); err == nil {
		password = p
	}
	if user == "" {
		return ""
	}
	return user + ":" + password
}

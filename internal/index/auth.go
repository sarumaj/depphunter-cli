package index

import (
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// credentials are the tokens this machine already holds for its package indexes. They
// are read from the user's own files only - never from the repository - and are sent
// to the host they were written for and to no other.
type credentials struct {
	bearer map[string]string // host -> npm token
	basic  map[string]string // host -> "user:password" from .netrc
}

func readCredentials(home string) *credentials {
	c := &credentials{bearer: map[string]string{}, basic: map[string]string{}}
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
	return c
}

// readNpmrc reads "//registry.example/:_authToken=…" lines.
func (c *credentials) readNpmrc(data []byte) {
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || !strings.HasPrefix(key, "//") {
			continue
		}
		key = strings.TrimSpace(key)
		if !strings.HasSuffix(key, ":_authToken") {
			continue
		}
		hostPath := strings.TrimSuffix(strings.TrimPrefix(key, "//"), ":_authToken")
		host, _, _ := strings.Cut(hostPath, "/")
		if token := strings.Trim(strings.TrimSpace(value), `"`); host != "" && token != "" {
			c.bearer[host] = token
		}
	}
}

// readNetrc reads the machine/login/password triples git and curl use.
func (c *credentials) readNetrc(data []byte) {
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

// apply adds the credentials for the request's host, if there are any.
func (c *credentials) apply(req *http.Request) {
	if c == nil {
		return
	}
	host := req.URL.Hostname()
	if token, ok := c.bearer[req.URL.Host]; ok {
		req.Header.Set("Authorization", "Bearer "+token)
		return
	}
	if token, ok := c.bearer[host]; ok {
		req.Header.Set("Authorization", "Bearer "+token)
		return
	}
	if pair, ok := c.basic[host]; ok {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(pair)))
	}
}

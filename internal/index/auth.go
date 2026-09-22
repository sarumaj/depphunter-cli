package index

import (
	"encoding/base64"
	"encoding/xml"
	"net/http"
	"net/url"
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
	return c
}

// readMavenSettings takes the username and password of every <server> whose id names
// a <mirror> or a <profile>'s <repository>, and files it under that URL's host.
//
// A settings.xml is where a developer's Nexus or Artifactory password lives, and
// without it every request to the company repository comes back 401 and the map goes
// quiet about half the dependency tree.
func (c *credentials) readMavenSettings(data []byte) {
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
		if host := hostOf(urls[s.ID]); host != "" {
			c.basic[host] = user + ":" + pass
		}
	}
}

// readNuGetConfig takes the credentials a NuGet configuration keeps for its own
// package sources - Azure Artifacts, a Nexus feed, ProGet - and files them under the
// host of the source they name.
func (c *credentials) readNuGetConfig(data []byte) {
	var doc struct {
		PackageSources struct {
			Add []struct {
				Key   string `xml:"key,attr"`
				Value string `xml:"value,attr"`
			} `xml:"add"`
		} `xml:"packageSources"`
		Credentials struct {
			// One element per source, named after it, so the shape is not known in
			// advance: it is read as a list of whatever elements are there.
			Sources []struct {
				XMLName xml.Name
				Add     []struct {
					Key   string `xml:"key,attr"`
					Value string `xml:"value,attr"`
				} `xml:"add"`
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
		if host := hostOf(urls[src.XMLName.Local]); host != "" {
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
	if name, ok := strings.CutPrefix(v, "%"); ok {
		if name, ok := strings.CutSuffix(name, "%"); ok && name != "" && !strings.Contains(name, "%") {
			return os.Getenv(name)
		}
	}
	return v
}

// hostOf is the host a source URL names, "" for anything that is not one.
func hostOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Hostname()
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

package index

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// ociConf is what this machine's container tools say about where an image is pulled
// from: the [[registry]] tables of registries.conf(5), and the registry-mirrors of
// the Docker daemon, which serve Docker Hub alone.
type ociConf struct {
	registries []ociRegistryConf
	hubMirrors []string // Docker daemon registry-mirrors, as URLs
}

// ociRegistryConf is one [[registry]] table of registries.conf.
type ociRegistryConf struct {
	prefix, location string
	blocked          bool
	mirrors          []ociMirror
}

// ociMirror is one [[registry.mirror]]; pull is its pull-from-mirror ("",
// "digest-only", "tag-only"), the registry's mirror-by-digest-only already folded in.
type ociMirror struct {
	location, pull string
}

// registriesFile is the part of a registries.conf (version 2) read here.
type registriesFile struct {
	Registries []struct {
		Prefix         string
		Location       string
		Blocked        bool
		DigestOnly     bool   `toml:"mirror-by-digest-only"`
		PullFromMirror string `toml:"pull-from-mirror"`
		Mirrors        []struct {
			Location       string
			PullFromMirror string `toml:"pull-from-mirror"`
		} `toml:"mirror"`
	} `toml:"registry"`
}

// readOCIConf reads the registries.conf files (userconf.RegistriesConf) as
// containers/image merges them: a drop-in's [[registry]] replaces the one with the
// same prefix, a prefix defaulting to the location; and the Docker daemon's
// registry-mirrors.
//
// unqualified-search-registries, short-name-mode and [aliases] are not read: the
// oci plugin names every Docker Hub image the short way ("nginx" for
// docker.io/library/nginx too), so a short name cannot be told from one written in
// full, and Docker itself, which builds most Dockerfiles, searches nothing. A
// version 1 file ([registries.search] and the like) is not read either.
//
// Implements: REQ-SUP-068
func readOCIConf(m userconf.Machine) ociConf {
	var out ociConf
	main, dropIns := m.RegistriesConf()
	byPrefix := map[string]int{}
	for _, name := range append([]string{main}, dropIns...) {
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		var doc registriesFile
		if _, err := toml.Decode(string(data), &doc); err != nil {
			continue
		}
		for _, r := range doc.Registries {
			registry := ociRegistryConf{
				prefix:   ociLocation(r.Prefix),
				location: ociLocation(r.Location),
				blocked:  r.Blocked,
			}
			if registry.prefix == "" {
				registry.prefix = registry.location
			}
			if registry.prefix == "" || strings.HasPrefix(registry.prefix, "*.") && registry.location != "" {
				continue // no prefix at all, or a wildcard with a location: invalid
			}
			for _, mr := range r.Mirrors {
				pull := mr.PullFromMirror
				if pull == "" && r.DigestOnly {
					pull = "digest-only"
				}
				if loc := ociLocation(mr.Location); loc != "" {
					registry.mirrors = append(registry.mirrors, ociMirror{location: loc, pull: pull})
				}
			}
			if i, ok := byPrefix[registry.prefix]; ok {
				out.registries[i] = registry
			} else {
				byPrefix[registry.prefix] = len(out.registries)
				out.registries = append(out.registries, registry)
			}
		}
	}
	if name := m.DockerDaemonConfig(); name != "" {
		if data, err := os.ReadFile(name); err == nil {
			var doc struct {
				Mirrors []string `json:"registry-mirrors"`
			}
			if json.Unmarshal(data, &doc) == nil {
				for _, raw := range doc.Mirrors {
					u, err := url.Parse(strings.TrimSpace(raw))
					if err != nil || u.Host == "" || u.Scheme != "https" && u.Scheme != "http" ||
						strings.Trim(u.Path, "/") != "" || u.RawQuery != "" {
						continue // what dockerd refuses to start with
					}
					out.hubMirrors = append(out.hubMirrors, u.Scheme+"://"+u.Host)
				}
			}
		}
	}
	return out
}

// ociLocation is a prefix or location as registries.conf compares it: without a
// trailing "/"; one with a scheme is not a location at all.
func ociLocation(s string) string {
	s = strings.TrimRight(strings.TrimSpace(s), "/")
	if strings.Contains(s, "://") {
		return ""
	}
	return s
}

// ociEndpoint is one place an image is pulled from: the registry's API (base), the
// repository there, and the URL that stands for the pair as a candidate index
// (ociEndpointAt).
type ociEndpoint struct {
	url, base, repository string
	mirror, known         bool
}

// ociName is an image reference's repository name in full, as registries.conf
// prefixes are matched against it: a Docker Hub image is docker.io/library/nginx
// however it was written.
func ociName(image string) string {
	first, _, ok := strings.Cut(image, "/")
	if ok && (strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost") {
		switch strings.ToLower(first) {
		case "index.docker.io", "registry-1.docker.io":
			return "docker.io" + image[len(first):]
		}
		return image
	}
	return "docker.io/" + ociRepository(image)
}

// ociMatch is how much of name a registries.conf prefix matches, 0 for none: all of
// the prefix, ending where name ends or at a "/"; a "*.example.com" prefix matches
// the host of name when it is a subdomain of example.com, and the match is that
// host.
func ociMatch(name, prefix string) int {
	if wild, ok := strings.CutPrefix(prefix, "*"); ok {
		host, _, _ := strings.Cut(name, "/")
		if strings.HasSuffix(host, wild) && len(host) > len(wild) {
			return len(host)
		}
		return 0
	}
	if name == prefix || strings.HasPrefix(name, prefix) && name[len(prefix)] == '/' {
		return len(prefix)
	}
	return 0
}

// ociEndpoints lists where an image is pulled from, in the order it is asked
// (candidates): blocked says a registries.conf forbids pulling it at all.
//
// The [[registry]] with the longest prefix matching the image's name rewrites it:
// each of its mirrors that serves this kind of reference (a digest, or a tag) first,
// in order, then its location, the matched prefix replaced by each one's location -
// as containers/image pulls. Without one, a Docker Hub image is asked of the Docker
// daemon's registry-mirrors first, and then of Docker Hub; any other image of the
// registry its name carries. What the machine's configuration names is known; the
// registry an image names alone is known as candidates says.
//
// Implements: REQ-SUP-068, REQ-SUP-017
func (c *Config) ociEndpoints(image, reference string) (out []ociEndpoint, blocked bool) {
	name := ociName(image)
	best, n := -1, 0
	for i, r := range c.oci.registries {
		if m := ociMatch(name, r.prefix); m > n || m > 0 && m == n && len(r.prefix) > len(c.oci.registries[best].prefix) {
			best, n = i, m
		}
	}
	if best < 0 {
		registry, repository := ociRegistry(image), ociRepository(image)
		if hub, ok := strings.CutPrefix(name, "docker.io/"); ok {
			// docker.io/library/debian, as a base image's annotation names it, is
			// Docker Hub's as much as debian is.
			registry, repository = public[OCI], hub
			for _, m := range c.oci.hubMirrors {
				out = append(out, ociEndpoint{url: m, base: m, repository: repository, mirror: true, known: true})
			}
		}
		host := Host(registry)
		return append(out, ociEndpoint{url: registry, base: registry, repository: repository,
			known: registry == public[OCI] || c.trusted[registry] || c.trusted[host] || c.credentials.Registry(host)}), false
	}
	r := c.oci.registries[best]
	if r.blocked {
		return nil, true
	}
	rest := name[n:]
	digest := strings.Contains(reference, ":") // sha256:..., as a tag holds no colon
	for _, m := range r.mirrors {
		if m.pull == "digest-only" && !digest || m.pull == "tag-only" && digest {
			continue
		}
		if e, ok := ociEndpointAt(m.location, rest, true); ok {
			e.mirror, e.known = true, true
			out = append(out, e)
		}
	}
	location := r.location
	if location == "" {
		location = name[:n] // a wildcard prefix: the matched host itself
	}
	if e, ok := ociEndpointAt(location, rest, false); ok {
		e.known = true
		out = append(out, e)
	}
	return out, false
}

// ociEndpointAt is the endpoint of a registries.conf location, with rest (what
// followed the matched prefix) after it. Docker Hub's names are asked of its API
// host, as they are without any configuration. A mirror's URL keeps the location's
// path, so that two mirrors on one host are two candidates; the registry's is its
// host alone, as it is without a registries.conf.
func ociEndpointAt(location, rest string, mirror bool) (ociEndpoint, bool) {
	host, path, _ := strings.Cut(location, "/")
	repository := strings.Trim(path+rest, "/")
	if host == "" || repository == "" {
		return ociEndpoint{}, false
	}
	api := host
	switch strings.ToLower(host) {
	case "docker.io", "index.docker.io":
		api = "registry-1.docker.io"
	}
	base := "https://" + api
	if api == "registry-1.docker.io" {
		base = public[OCI]
	}
	u := base
	if path != "" && mirror {
		u += "/" + path
	}
	return ociEndpoint{url: u, base: base, repository: repository}, true
}

// ociRoute is where a request for an image goes when it is asked of index (one of
// its candidates): the registry API and the repository there.
func (c *Config) ociRoute(image, reference, index string) (base, repository string) {
	eps, _ := c.ociEndpoints(image, reference)
	for _, e := range eps {
		if e.url == index {
			return e.base, e.repository
		}
	}
	return index, ociRepository(image)
}

// ociCandidates are an image's candidates (ociEndpoints). A container reference
// carries its registry: "ghcr.io/org/app" is not "app" from Docker Hub. Nothing needs
// to be configured to see that; to be asked, the registry has to be Docker Hub, one
// this machine's container configuration names, or one the user vouched for. A mirror
// is asked first and passed over on any failure, as containers/image and dockerd
// fall back from one; it is not where the map attributes the image to (primary),
// the registry is. reference is the reference's tag or digest: some mirrors serve digests
// alone, some tags alone.
//
// Implements: REQ-SUP-017, REQ-SUP-026, REQ-SUP-068
func (c *Config) ociCandidates(image, reference string) []candidate {
	eps, _ := c.ociEndpoints(image, reference)
	out := make([]candidate, 0, len(eps))
	for _, e := range eps {
		out = append(out, candidate{url: e.url, known: e.known, primary: !e.mirror, onError: e.mirror})
	}
	return out
}

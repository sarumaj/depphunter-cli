// Package oci is what every plugin that names a container image shares: the
// ecosystem the images land in, and how a reference is read. A CI job's image:, a
// Dockerfile's FROM and a Compose service's image: are the same kind of dependency,
// so they resolve to the same island, under the same names, by the same pinning rule -
// and whatever the index client does with an "oci" package under --online (following
// the base image, asking the registry the name carries, with this machine's
// credentials) applies to all of them alike.
package oci

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Ecosystem is the id every container image resolves under; it is also the prefix of
// a private pattern (oci:registry.example.com/*).
const Ecosystem = "oci"

// Island is the ecosystem as a plugin declares it. Plugins declaring the same id share
// one island.
var Island = lang.Ecosystem{ID: Ecosystem, Name: "Container images"}

// Image resolves a container reference, "[registry/]name[:tag][@digest]". A tag is
// republished whenever its owner likes, so only a digest pins an image.
//
// Implements: REQ-CI-012, REQ-CI-013, REQ-DOCKER-004
func Image(reference string) lang.Target {
	name, digest, hasDigest := strings.Cut(reference, "@")
	tag := ""
	// A colon in the registry part is a port, not a tag: "localhost:5000/img".
	if i := strings.LastIndex(name, ":"); i >= 0 && !strings.Contains(name[i:], "/") {
		name, tag = name[:i], name[i+1:]
	}
	name = canonical(name)
	if name == "" {
		return lang.Target{}
	}
	switch {
	case hasDigest:
		return lang.Target{Ecosystem: Ecosystem, Package: name, Version: digest, Requested: tag, Pinned: lang.Pinned(digest)}
	case tag == "":
		// No tag at all means whatever :latest is today, the loosest reference there
		// is; it names no version, so none is made up for it.
		return lang.Target{Ecosystem: Ecosystem, Package: name, Floating: true}
	}
	return lang.Target{Ecosystem: Ecosystem, Package: name, Version: tag}
}

// canonical names a Docker Hub image the short way whichever way it was written:
// "docker.io/library/nginx", "index.docker.io/library/nginx" and "library/nginx" all
// pull the same image as "nginx", so they are one package on the map - and the short
// name is also the one the index client reads as Docker Hub, which the long ones,
// with a host it does not know as the public registry, would not be. Any other
// registry's names are kept as written.
//
// Implements: REQ-DOCKER-004
func canonical(name string) string {
	first, rest, ok := strings.Cut(name, "/")
	if !ok {
		return name
	}
	switch strings.ToLower(first) {
	case "docker.io", "index.docker.io", "registry-1.docker.io":
		name = rest
	}
	if short, ok := strings.CutPrefix(name, "library/"); ok && short != "" && !strings.Contains(short, "/") {
		return short
	}
	return name
}

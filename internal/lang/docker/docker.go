// Package docker analyzes container build files as lists of dependencies: the images
// a Dockerfile builds on and copies from, and the images and build files a Compose
// file brings together. None of them appears in any package manifest, yet each is
// code that ends up in what the project ships or runs.
//
// Every image lands in the same "oci" island as the images CI jobs run in (package
// oci), named and pinned the same way: only a digest pins, a tag floats. A Compose
// service that is built rather than pulled becomes an edge to the Dockerfile inside
// the repository that builds it, so that file's own base images chain on.
package docker

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/oci"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Import kinds, carried in RawImport.Name so the resolver knows how to read Module.
const (
	kindImage = "image" // a container reference, possibly with a variable left in it
	kindBuild = "build" // a Dockerfile, relative to the Compose file's directory
)

// Import kinds of a Compose file whose Module is the text as written, interpolated by
// the resolver with the values of the .env file beside the Compose file.
const (
	kindCompose = "compose-image" // a container reference
	kindInclude = "include"       // a Compose file the top-level include: lists
	kindExtends = "extends"       // a service of this file extending one of another file
)

// The two kinds of file, as Class names them.
const (
	classDockerfile = "dockerfile"
	classCompose    = "compose"
)

type Plugin struct{}

func (Plugin) Name() string { return "docker" }
func (Plugin) Version() int { return 2 }

// Claims takes Dockerfiles under every name scan.Dockerfile knows, and the files
// Compose reads by default: compose.yaml, docker-compose.yml, their .yml/.yaml
// spellings and their override and per-environment variants.
//
// Implements: REQ-DOCKER-001
func (Plugin) Claims(f *scan.File) bool {
	return !f.Binary && (scan.Dockerfile(f.Path) || composeFile(f.Path))
}

// composeFile reports whether p is named as a Compose file.
func composeFile(p string) bool {
	base := strings.ToLower(path.Base(p))
	ext := path.Ext(base)
	if ext != ".yml" && ext != ".yaml" {
		return false
	}
	stem := strings.TrimSuffix(base, ext)
	return stem == "compose" || strings.HasPrefix(stem, "compose.") || strings.HasPrefix(stem, "docker-compose")
}

// Class is which of the two kinds of file f is (lang.Classifier). A Dockerfile.yml
// and a compose.yml share an extension but are read by different parsers, so the
// kind is part of the cache key.
//
// Implements: REQ-DOCKER-001
func (Plugin) Class(f *scan.File) string {
	if scan.Dockerfile(f.Path) {
		return classDockerfile
	}
	return classCompose
}

// Implements: REQ-DOCKER-006
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{oci.Island}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Implements: REQ-DOCKER-001
func (p Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	if p.Class(f) == classDockerfile {
		return extractDockerfile(src), nil
	}
	return extractCompose(src), nil
}

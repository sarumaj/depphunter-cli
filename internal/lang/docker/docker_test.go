package docker

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/cache"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const digest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// testdata/repo: a multi-stage Dockerfile using build arguments, a Compose file with
// every way a service names an image or a build, an override file in another
// directory, and the Dockerfile both Compose files build.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

func image(pkg, version string) lang.Target {
	return lang.Target{Ecosystem: "oci", Package: pkg, Version: version}
}

// Verifies: REQ-DOCKER-002, REQ-DOCKER-003, REQ-DOCKER-004, REQ-DOCKER-005, REQ-DOCKER-006
func TestDockerfile(t *testing.T) {
	res := analyze(t)["Dockerfile"]
	langtest.CheckImports(t, res, map[string]lang.Target{
		// The frontend BuildKit pulls to read the file is an image like any other.
		"# syntax=docker/dockerfile:1.7": image("docker/dockerfile", "1.7"),
		// Defaults of ARGs declared before the first FROM are expanded.
		"FROM golang:1.22":             image("golang", "1.22"),
		"FROM node:20-alpine":          image("node", "20-alpine"),
		"RUN --mount from=alpine:3.20": image("alpine", "3.20"),
		"COPY --from=nginx:1.27@" + digest: {
			Ecosystem: "oci", Package: "nginx", Version: digest, Requested: "1.27", Pinned: true,
		},
		"COPY --from=busybox:1.36": image("busybox", "1.36"),
		// A registry only --build-arg can name leaves the image unknown.
		"FROM ${REGISTRY}/tools:1": {Ecosystem: "oci", Package: "${REGISTRY}/tools:1", Unresolved: true},
		// A tag only --build-arg can name leaves the version unknown, and unpinned.
		"FROM python:${TAG}": image("python", "${TAG}"),
		// Docker Hub's long form is the short one.
		"FROM docker.io/library/debian:bookworm-slim": image("debian", "bookworm-slim"),
		// Not here: stage references (build, 0, Runtime, web), scratch, and the
		// heredoc's text.
	})
	langtest.CheckSymbols(t, res, map[string]string{
		"Build": "stage", "web": "stage", "tools": "stage", "runtime": "stage",
	})
	lines := map[string]int{}
	for _, im := range res.Imports {
		lines[im.Spec] = im.Line
	}
	if lines["RUN --mount from=alpine:3.20"] != 9 || lines["FROM docker.io/library/debian:bookworm-slim"] != 24 {
		t.Errorf("an instruction is reported on the line it starts on: %v", lines)
	}
}

// Verifies: REQ-DOCKER-003, REQ-DOCKER-004, REQ-DOCKER-005, REQ-DOCKER-006, REQ-DOCKER-007
func TestCompose(t *testing.T) {
	res := analyze(t)["compose.yaml"]
	langtest.CheckImports(t, res, map[string]lang.Target{
		// Built services point at their Dockerfile; web's image: is the tag it gets.
		"build: Dockerfile":                    {Local: "Dockerfile"},
		"build: api/api.Dockerfile":            {Local: "api/api.Dockerfile"},
		"additional context: alpine:3.20":      image("alpine", "3.20"),
		"FROM alpine:3.20":                     image("alpine", "3.20"), // dockerfile_inline
		"image: postgres:${PG_VERSION:-16}":    image("postgres", "16"),
		"image: redis@" + digest:               {Ecosystem: "oci", Package: "redis", Version: digest, Pinned: true},
		"image: ${PROXY_IMAGE}":                {Ecosystem: "oci", Package: "${PROXY_IMAGE}", Unresolved: true},
		"image: docker.io/library/python:3.12": image("python", "3.12"),
		// A context that is not in the repository is dropped; so is a missing file.
		"build: nowhere/Dockerfile": {},
	})
	langtest.CheckSymbols(t, res, map[string]string{
		"web": "service", "api": "service", "db": "service", "cache": "service", "proxy": "service",
		"worker": "service", "inline": "service", "remote": "service", "missing": "service",
	})

	// A context is read from the Compose file's directory, and a merge key shares an
	// image between services.
	langtest.CheckImports(t, analyze(t)["deploy/docker-compose.prod.yml"], map[string]lang.Target{
		"image: ghcr.io/acme/web:${TAG}": image("ghcr.io/acme/web", "${TAG}"),
		"build: ../api/api.Dockerfile":   {Local: "api/api.Dockerfile"},
	})
	langtest.CheckImports(t, analyze(t)["api/api.Dockerfile"], map[string]lang.Target{
		"FROM eclipse-temurin:21-jre": image("eclipse-temurin", "21-jre"),
	})
}

// Verifies: REQ-DOCKER-001
func TestClaims(t *testing.T) {
	for _, p := range []string{
		"Dockerfile", "Containerfile", "build/Dockerfile.dev", "api.Dockerfile", "worker.dockerfile",
		"compose.yaml", "compose.yml", "compose.override.yaml", "docker-compose.yml",
		"deploy/docker-compose.prod.yaml",
	} {
		if !(Plugin{}).Claims(&scan.File{Path: p}) {
			t.Errorf("%s: not claimed", p)
		}
	}
	for _, p := range []string{
		"Dockerfile.dockerignore", "composer.json", "composer.yml", "docker/config.yml", "README.md",
	} {
		if (Plugin{}).Claims(&scan.File{Path: p}) {
			t.Errorf("%s: claimed", p)
		}
	}
	if (Plugin{}).Claims(&scan.File{Path: "Dockerfile", Binary: true}) {
		t.Error("a binary file was claimed")
	}
}

// Verifies: REQ-DOCKER-001
func TestTheKindOfFileIsPartOfTheCacheKey(t *testing.T) {
	src := []byte("services: {}\n")
	key := func(p string) string {
		return cache.Key(Plugin{}.Name(), Plugin{}.Version(), lang.ClassOf(Plugin{}, &scan.File{Path: p}), src)
	}
	if key("Dockerfile.yml") == key("compose.yml") {
		t.Error("a Dockerfile and a Compose file share a cache entry")
	}
}

// Verifies: REQ-DOCKER-002
func TestInstructions(t *testing.T) {
	src := "# escape=`\r\n" +
		"from alpine:3 `\r\n" +
		"  as base\r\n" +
		"RUN <<-EOF cat > /a\n\tFROM inside:heredoc\n\tEOF\n" +
		"RUN echo $((1<<2)) \\\n" + // not a heredoc; and "\" does not continue here
		"FROM base\n" +
		"RUN cat <<NEVER\n" + // never terminated: not a heredoc after all
		"COPY --from=busybox / /\n"
	got, _ := instructions([]byte(src))
	var keywords []string
	for _, in := range got {
		keywords = append(keywords, in.keyword)
	}
	want := []string{"FROM", "RUN", "RUN", "FROM", "RUN", "COPY"}
	if !reflect.DeepEqual(keywords, want) {
		t.Fatalf("got %v, want %v", keywords, want)
	}
	if got[0].args != "alpine:3 as base" || got[0].line != 2 || got[3].line != 8 {
		t.Errorf("instructions: %+v", got)
	}
}

// Verifies: REQ-DOCKER-003
func TestExpand(t *testing.T) {
	vars := map[string]string{"A": "a", "EMPTY": ""}
	for _, c := range []struct {
		in, want string
		ok       bool
	}{
		{"$A/${A}", "a/a", true},
		{"${EMPTY:-x}${EMPTY-y}", "x", true},
		{"${UNSET:-${A}}", "a", true},
		{"${A:+set}${UNSET:+no}", "set", true},
		{"img:$UNSET", "img:$UNSET", false},
		{"${UNSET:?required}", "${UNSET:?required}", false},
		{`\$A`, "$A", true},
	} {
		if got, ok := expand(c.in, vars, false); got != c.want || ok != c.ok {
			t.Errorf("%s: got %q %v, want %q %v", c.in, got, ok, c.want, c.ok)
		}
	}
	if got, ok := expand("a$$b", nil, true); got != "a$b" || !ok {
		t.Errorf("compose $$: got %q %v", got, ok)
	}
}

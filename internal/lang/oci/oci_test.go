package oci

import (
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Verifies: REQ-CI-012, REQ-CI-013
func TestImageReferences(t *testing.T) {
	digest := "sha256:" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	for _, c := range []struct {
		reference string
		want      lang.Target
	}{
		// No tag floats, with no version made up for it.
		{"nginx", lang.Target{Ecosystem: "oci", Package: "nginx", Floating: true}},
		{"nginx:1.25.3", lang.Target{Ecosystem: "oci", Package: "nginx", Version: "1.25.3"}},
		{"ghcr.io/org/app:main", lang.Target{Ecosystem: "oci", Package: "ghcr.io/org/app", Version: "main"}},
		// A registry's port is not a tag.
		{"localhost:5000/app", lang.Target{Ecosystem: "oci", Package: "localhost:5000/app", Floating: true}},
		{"localhost:5000/app:2", lang.Target{Ecosystem: "oci", Package: "localhost:5000/app", Version: "2"}},
		// Only a digest pins, and the tag beside it is what was asked for.
		{"postgres:16@" + digest, lang.Target{Ecosystem: "oci", Package: "postgres", Version: digest, Requested: "16", Pinned: true}},
		{"", lang.Target{}},
	} {
		if got := Image(c.reference); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.reference, got, c.want)
		}
	}
}

// Verifies: REQ-DOCKER-004
func TestDockerHubNamesAreCanonical(t *testing.T) {
	for reference, want := range map[string]string{
		"docker.io/library/nginx:1.27":     "nginx",
		"index.docker.io/library/nginx":    "nginx",
		"registry-1.docker.io/nginx":       "nginx",
		"library/nginx":                    "nginx",
		"docker.io/bitnami/redis:7":        "bitnami/redis",
		"ghcr.io/library/tool":             "ghcr.io/library/tool", // another registry's names are its own
		"quay.io/prometheus/node-exporter": "quay.io/prometheus/node-exporter",
	} {
		if got := Image(reference).Package; got != want {
			t.Errorf("%s: got %q, want %q", reference, got, want)
		}
	}
}

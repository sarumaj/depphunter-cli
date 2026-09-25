package ci

import (
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/cache"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// testdata/repo holds a GitHub workflow (actions by tag and by commit, a local
// composite action, a container and a service, a reusable workflow in another
// repository and one in this one), that composite action, and a GitLab pipeline with
// every include form.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-CI-002, REQ-CI-003, REQ-CI-004, REQ-CI-009, REQ-CI-011, REQ-CI-012, REQ-CI-014
func TestWorkflow(t *testing.T) {
	// cSpell: disable
	langtest.CheckImports(t, analyze(t)[".github/workflows/ci.yml"], map[string]lang.Target{
		// A tag can be moved to other code, so pinning means a commit - and the
		// version a commit documents beside itself is kept as what was asked for.
		"uses: actions/checkout@v4": {Ecosystem: "actions", Package: "actions/checkout", Version: "v4"},
		"uses: actions/setup-go@3041bf56c941b39c61721a86cd11f3bb1338122a # v5.0.1": {
			Ecosystem: "actions", Package: "actions/setup-go",
			Version: "3041bf56c941b39c61721a86cd11f3bb1338122a", Requested: "v5.0.1", Pinned: true,
		},
		"uses: ./.github/actions/setup": {Local: ".github/actions/setup/action.yml"},
		"uses: docker://alpine:3.19":    {Ecosystem: "oci", Package: "alpine", Version: "3.19"},
		"container: golang:1.27-alpine": {Ecosystem: "oci", Package: "golang", Version: "1.27-alpine"},
		"service postgres: postgres:16@sha256:aa11bb22cc33dd44ee55ff66aa11bb22cc33dd44ee55ff66aa11bb22cc33dd44": {
			Ecosystem: "oci", Package: "postgres",
			Version:   "sha256:aa11bb22cc33dd44ee55ff66aa11bb22cc33dd44ee55ff66aa11bb22cc33dd44",
			Requested: "16", Pinned: true,
		},
		"uses: octo-org/shared/.github/workflows/release.yml@a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0": {
			Ecosystem: "actions", Package: "octo-org/shared",
			Version: "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0", Pinned: true,
		},
		"uses: ./.github/workflows/build.yml": {Local: ".github/workflows/build.yml"},
	})
	// cSpell: enable
}

// Verifies: REQ-CI-010
func TestWorkflowJobsAreSymbols(t *testing.T) {
	got := langtest.Symbols(t, analyze(t)[".github/workflows/ci.yml"])
	for _, job := range []string{"test", "release", "build"} {
		if got[job] != "job" {
			t.Errorf("job %s: kind %q", job, got[job])
		}
	}
	if len(got) != 3 {
		t.Errorf("symbols %v, want the three jobs", got)
	}
}

// Verifies: REQ-CI-002, REQ-CI-014
func TestCompositeAction(t *testing.T) {
	langtest.CheckImports(t, analyze(t)[".github/actions/setup/action.yml"], map[string]lang.Target{
		"uses: actions/cache@0c907a75c2c80ebcb7f088228285e798b750cf8f # v4.2.1": {
			Ecosystem: "actions", Package: "actions/cache",
			Version: "0c907a75c2c80ebcb7f088228285e798b750cf8f", Requested: "v4.2.1", Pinned: true,
		},
	})
}

// Verifies: REQ-CI-005, REQ-CI-007, REQ-CI-008, REQ-CI-009, REQ-CI-011, REQ-CI-013
func TestGitLabPipeline(t *testing.T) {
	langtest.CheckImports(t, analyze(t)[".gitlab-ci.yml"], map[string]lang.Target{
		"include local: /.gitlab/ci/build.yml": {Local: ".gitlab/ci/build.yml"},
		"include project: infra/pipelines@9f8e7d6c5b4a39281706f5e4d3c2b1a09f8e7d6c /templates/deploy.yml": {
			Ecosystem: "gitlab-ci", Package: "infra/pipelines",
			Version: "9f8e7d6c5b4a39281706f5e4d3c2b1a09f8e7d6c", Pinned: true,
		},
		// A GitLab-provided template moves with the instance that serves it.
		"include template: Security/SAST.gitlab-ci.yml":         {Ecosystem: "gitlab-ci", Package: "template: Security/SAST.gitlab-ci.yml", Floating: true},
		"include remote: https://example.com/ci/shared.yml?v=2": {Ecosystem: "gitlab-ci", Package: "example.com/ci/shared.yml", Floating: true},
		"include component: gitlab.com/components/sonar/scan@1.4.0": {
			Ecosystem: "gitlab-ci", Package: "gitlab.com/components/sonar/scan", Version: "1.4.0",
		},
		"image: node:20":                         {Ecosystem: "oci", Package: "node", Version: "20"},
		"default service: redis:7.2":             {Ecosystem: "oci", Package: "redis", Version: "7.2"},
		"unit image: python:3.12-slim":           {Ecosystem: "oci", Package: "python", Version: "3.12-slim"},
		".hidden-template image: busybox:latest": {Ecosystem: "oci", Package: "busybox", Version: "latest"},
	})
}

// Verifies: REQ-CI-010
func TestGitLabJobsAreSymbols(t *testing.T) {
	got := langtest.Symbols(t, analyze(t)[".gitlab-ci.yml"])
	for _, job := range []string{"unit", ".hidden-template"} {
		if got[job] != "job" {
			t.Errorf("job %s: kind %q", job, got[job])
		}
	}
	if len(got) != 2 {
		t.Errorf("symbols %v: the pipeline's own keys are not jobs", got)
	}
}

// Verifies: REQ-CI-001
func TestClaims(t *testing.T) {
	for _, p := range []string{
		".github/workflows/ci.yml", ".github/workflows/nightly.yaml", "action.yml",
		"tools/my-action/action.yaml", ".gitlab-ci.yml", "ci/build.gitlab-ci.yml", ".gitlab/ci/test.yml",
	} {
		if !(Plugin{}).Claims(&scan.File{Path: p}) {
			t.Errorf("%s: not claimed", p)
		}
	}
	for _, p := range []string{
		"docker-compose.yml", "config/app.yaml", ".github/dependabot.yml",
		"README.md", ".github/workflows/notes.txt",
	} {
		if (Plugin{}).Claims(&scan.File{Path: p}) {
			t.Errorf("%s: claimed, but it is not a pipeline", p)
		}
	}
	if (Plugin{}).Claims(&scan.File{Path: ".gitlab-ci.yml", Binary: true}) {
		t.Error("a binary file was claimed")
	}
}

// Verifies: REQ-CI-012
func TestImageReferences(t *testing.T) {
	for _, c := range []struct {
		ref  string
		want lang.Target
	}{
		{"nginx", lang.Target{Ecosystem: "oci", Package: "nginx", Version: "latest"}},
		{"nginx:1.25.3", lang.Target{Ecosystem: "oci", Package: "nginx", Version: "1.25.3"}},
		{"ghcr.io/org/app:main", lang.Target{Ecosystem: "oci", Package: "ghcr.io/org/app", Version: "main"}},
		// A registry's port is not a tag.
		{"localhost:5000/app", lang.Target{Ecosystem: "oci", Package: "localhost:5000/app", Version: "latest"}},
		{"localhost:5000/app:2", lang.Target{Ecosystem: "oci", Package: "localhost:5000/app", Version: "2"}},
	} {
		if got := image(c.ref); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.ref, got, c.want)
		}
	}
}

// The same YAML is read as a workflow, an action or a GitLab pipeline depending on
// where it is, so where it is has to be part of the cache key.
//
// Verifies: REQ-CI-001
func TestTheKindOfFileIsPartOfTheCacheKey(t *testing.T) {
	src := []byte("runs:\n  using: composite\n")
	key := func(p string) string {
		return cache.Key(Plugin{}.Name(), Plugin{}.Version(), lang.ClassOf(Plugin{}, &scan.File{Path: p}), src)
	}
	if key(".github/workflows/x.yml") == key("action.yml") || key("action.yml") == key(".gitlab-ci.yml") {
		t.Error("files read differently share a cache entry")
	}
	if key(".github/workflows/a.yml") != key(".github/workflows/b.yml") {
		t.Error("two workflows with the same content do not share one")
	}
}

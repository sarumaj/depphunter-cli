package ci

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// cSpell: ignore octo

// Verifies: REQ-CI-016
func TestActionDependencies(t *testing.T) {
	for _, test := range []struct {
		name, source string
		want         []lang.Target
		dockerfile   string
	}{
		{"composite", `runs:
  using: composite
  steps:
    - uses: actions/cache/restore@v4
    - uses: ./local-in-the-callers-workspace
    - uses: docker://alpine:3.19
    - run: echo hi
`, []lang.Target{
			{Ecosystem: "actions", Package: "actions/cache", Version: "v4", Registry: "restore"},
			{Ecosystem: "oci", Package: "alpine", Version: "3.19"},
		}, ""},
		{"docker image", "runs:\n  using: docker\n  image: docker://ghcr.io/acme/tool:1.2\n",
			[]lang.Target{{Ecosystem: "oci", Package: "ghcr.io/acme/tool", Version: "1.2"}}, ""},
		{"dockerfile", "runs:\n  using: docker\n  image: build/Dockerfile\n", nil, "build/Dockerfile"},
		{"javascript", "runs:\n  using: node20\n  main: dist/index.js\n", nil, ""},
		{"not yaml", "runs: [", nil, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, dockerfile := ActionDependencies([]byte(test.source))
			if !reflect.DeepEqual(got, test.want) || dockerfile != test.dockerfile {
				t.Errorf("got %+v, %q; want %+v, %q", got, dockerfile, test.want, test.dockerfile)
			}
		})
	}
}

// Verifies: REQ-CI-016
func TestWorkflowDependencies(t *testing.T) {
	source := `on: workflow_call
jobs:
  build:
    runs-on: ubuntu-latest
    container: golang:1.27
    steps:
      - uses: actions/checkout@v4
      - uses: ./.github/actions/in-the-callers-workspace
  nested:
    uses: ./.github/workflows/nested.yml
  other:
    uses: octo-org/shared/.github/workflows/x.yml@v1
`
	got := WorkflowDependencies([]byte(source), "acme/workflows", "v2")
	want := []lang.Target{
		{Ecosystem: "oci", Package: "golang", Version: "1.27"},
		{Ecosystem: "actions", Package: "actions/checkout", Version: "v4"},
		{Ecosystem: "actions", Package: "acme/workflows", Version: "v2", Registry: ".github/workflows/nested.yml"},
		{Ecosystem: "actions", Package: "octo-org/shared", Version: "v1", Registry: ".github/workflows/x.yml"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

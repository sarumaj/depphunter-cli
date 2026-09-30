// Package ci analyzes continuous-integration configuration as what it is: a list of
// dependencies. A workflow pulls in actions, reusable workflows and container images,
// a GitLab pipeline pulls in includes, components, templates and images - all of them
// third-party code that runs with the repository's secrets, and none of it visible in
// any package manifest.
//
// What counts as pinned here is stricter than in a package ecosystem: only an
// immutable reference pins. A git tag can be moved and a container tag can be
// republished, so actions/checkout@v4 and nginx:1.25.3 float; a commit and an OCI
// digest do not.
package ci

import (
	"path"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/oci"
	"github.com/sarumaj/depphunter-cli/internal/lang/yamlnode"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecosystemActions = "actions"
	ecosystemGitLab  = "gitlab-ci"
)

// yamlReader follows aliases: a pipeline shares job settings through anchors.
var yamlReader = yamlnode.Reader{Aliases: true}

// Import kinds, carried in RawImport.Name so the resolver knows how to read Module.
const (
	kindAction    = "action"    // a step's uses:
	kindWorkflow  = "workflow"  // a job's uses:, a reusable workflow
	kindImage     = "image"     // container image
	kindLocal     = "local"     // a path inside this repository
	kindProject   = "project"   // GitLab include:project
	kindTemplate  = "template"  // GitLab include:template
	kindRemote    = "remote"    // GitLab include:remote
	kindComponent = "component" // GitLab CI/CD component
)

type Plugin struct{}

func (Plugin) Name() string { return "ci" }
func (Plugin) Version() int { return 1 }

// Claims takes the files a CI platform reads: GitHub workflows and composite actions,
// and GitLab pipeline files, including the ones a pipeline includes from .gitlab/.
//
// Implements: REQ-CI-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary {
		return false
	}
	p := f.Path
	if extension := path.Ext(p); extension != ".yml" && extension != ".yaml" {
		return false
	}
	base := path.Base(p)
	switch {
	case strings.HasPrefix(p, ".github/workflows/"):
		return true
	case base == "action.yml", base == "action.yaml":
		return true
	case base == ".gitlab-ci.yml", base == ".gitlab-ci.yaml", strings.HasSuffix(base, ".gitlab-ci.yml"):
		return true
	case strings.HasPrefix(p, ".gitlab/"):
		return true
	}
	return false
}

// Implements: REQ-CI-008
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecosystemActions, Name: "GitHub Actions"},
		{ID: ecosystemGitLab, Name: "GitLab CI"},
		oci.Island, // shared with the docker plugin
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all), nil
}

// Class is which of the three kinds of file f is (lang.Classifier): Extract reads the
// same YAML differently for each, so the kind is part of the cache key and a cached
// extraction is never read back for a file that has moved between them.
//
// Implements: REQ-CI-001
func (Plugin) Class(f *scan.File) string {
	base := path.Base(f.Path)
	switch {
	case base == "action.yml" || base == "action.yaml":
		return "action"
	case strings.HasPrefix(f.Path, ".github/workflows/"):
		return "workflow"
	}
	return "gitlab"
}

// Extract reads one configuration file, as the kind Class says it is.
//
// Implements: REQ-CI-001
func (p Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	root := yamlReader.Mapping(yamlnode.Parse(source))
	if root == nil {
		return &lang.Extraction{}, nil // a pipeline that does not parse has no dependencies
	}
	switch p.Class(f) {
	case "action":
		return extractAction(root), nil
	case "workflow":
		return extractWorkflow(root), nil
	}
	return extractGitLab(root), nil
}

// comment returns the version a pinned reference documents beside itself: the
// hardening guides tell you to pin an action to a commit, so the readable version
// survives only as "# v4.1.1" at the end of the line.
//
// Implements: REQ-CI-014
func comment(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	c := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(n.LineComment), "#"))
	if i := strings.IndexAny(c, " \t"); i >= 0 {
		c = c[:i] // "v4.1.1 (latest)" - only the version is of use
	}
	return c
}

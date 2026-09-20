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
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoActions = "actions"
	ecoGitLab  = "gitlab-ci"
	ecoOCI     = "oci"
)

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
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary {
		return false
	}
	p := f.Path
	if ext := path.Ext(p); ext != ".yml" && ext != ".yaml" {
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

func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoActions, Name: "GitHub Actions"},
		{ID: ecoGitLab, Name: "GitLab CI"},
		{ID: ecoOCI, Name: "Container images"},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all), nil
}

// Extract reads one configuration file. Which platform's it is follows from the path,
// which is part of the cache key, so a cached extraction is never read back for a file
// that has moved between the two.
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return &lang.Extraction{}, nil // a pipeline that does not parse has no dependencies
	}
	root := mapping(&doc)
	if root == nil {
		return &lang.Extraction{}, nil
	}
	base := path.Base(f.Path)
	if base == "action.yml" || base == "action.yaml" {
		return extractAction(root), nil
	}
	if strings.HasPrefix(f.Path, ".github/workflows/") {
		return extractWorkflow(root), nil
	}
	return extractGitLab(root), nil
}

// ---------------------------------------------------------------- YAML helpers

// mapping unwraps a document node and returns it when it is a mapping.
func mapping(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		n = n.Content[0]
	}
	if n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	return n
}

// field returns the value of key in a mapping, or nil.
func field(n *yaml.Node, key string) *yaml.Node {
	m := mapping(n)
	if m == nil {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			v := m.Content[i+1]
			if v.Kind == yaml.AliasNode {
				v = v.Alias
			}
			return v
		}
	}
	return nil
}

type pair struct{ key, value *yaml.Node }

// pairs lists a mapping's entries in order.
func pairs(n *yaml.Node) []pair {
	m := mapping(n)
	if m == nil {
		return nil
	}
	var out []pair
	for i := 0; i+1 < len(m.Content); i += 2 {
		v := m.Content[i+1]
		if v.Kind == yaml.AliasNode {
			v = v.Alias
		}
		out = append(out, pair{m.Content[i], v})
	}
	return out
}

// items lists a sequence's entries; a lone value counts as a sequence of one, which is
// how both platforms let a single include or service be written without a dash.
func items(n *yaml.Node) []*yaml.Node {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	if n.Kind != yaml.SequenceNode {
		return []*yaml.Node{n}
	}
	out := make([]*yaml.Node, 0, len(n.Content))
	for _, c := range n.Content {
		if c.Kind == yaml.AliasNode {
			c = c.Alias
		}
		out = append(out, c)
	}
	return out
}

// text returns a scalar's value, "" for anything else.
func text(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return strings.TrimSpace(n.Value)
}

// comment returns the version a pinned reference documents beside itself: the
// hardening guides tell you to pin an action to a commit, so the readable version
// survives only as "# v4.1.1" at the end of the line.
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

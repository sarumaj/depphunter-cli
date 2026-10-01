package ci

import (
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/yamlnode"
)

// reserved are the top-level keys that configure the pipeline; everything else at that
// level is a job, including the hidden ".template" ones jobs extend.
//
// Implements: REQ-CI-010
var reserved = map[string]bool{
	"image": true, "services": true, "stages": true, "types": true, "before_script": true,
	"after_script": true, "variables": true, "cache": true, "include": true,
	"default": true, "workflow": true,
}

// extractGitLab reads a GitLab pipeline: its jobs are the symbols, and its includes,
// components, templates and images are the dependencies.
//
// Implements: REQ-CI-005, REQ-CI-006, REQ-CI-007, REQ-CI-010
func extractGitLab(root *yaml.Node) *lang.Extraction {
	extraction := &lang.Extraction{}
	for _, include := range yamlnode.List(yamlnode.Get(root, "include")) {
		addInclude(extraction, include)
	}
	addImages(extraction, root, "")
	// What default or a job takes through a merge key ("<<: *defaults", GitLab's way
	// of sharing settings besides extends:) is its own.
	addImages(extraction, yamlnode.Merged(yamlnode.Get(root, "default")), "default")

	for _, job := range yamlnode.Pairs(root) {
		name := yamlnode.Text(job.Key)
		if name == "" || reserved[name] || yamlnode.Mapping(job.Value) == nil {
			continue
		}
		extraction.Symbols = append(extraction.Symbols, lang.Symbol{Name: name, Kind: "job", Line: job.Key.Line})
		body := yamlnode.Merged(job.Value)
		addImages(extraction, body, name)
		// A bridge job runs another pipeline, which is a dependency like any include.
		for _, include := range yamlnode.List(yamlnode.Get(body, "trigger", "include")) {
			addInclude(extraction, include)
		}
	}
	return extraction
}

// addInclude records one include entry in any of the forms GitLab accepts.
//
// Implements: REQ-CI-005, REQ-CI-006
func addInclude(extraction *lang.Extraction, n *yaml.Node) {
	if n == nil {
		return
	}
	add := func(spec, module, kind string, line int) {
		if module != "" {
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
		}
	}
	if s := yamlnode.Text(n); s != "" { // include: "path" or a bare URL
		kind := kindLocal
		if isURL(s) {
			kind = kindRemote
		}
		add("include: "+s, s, kind, n.Line)
		return
	}
	switch {
	case yamlnode.Get(n, "local") != nil:
		v := yamlnode.Get(n, "local")
		add("include local: "+yamlnode.Text(v), yamlnode.Text(v), kindLocal, v.Line)
	case yamlnode.Get(n, "project") != nil:
		v := yamlnode.Get(n, "project")
		project, reference := yamlnode.Text(v), yamlnode.Text(yamlnode.Get(n, "ref"))
		files := []string{}
		for _, f := range yamlnode.List(yamlnode.Get(n, "file")) {
			files = append(files, yamlnode.Text(f))
		}
		spec := "include project: " + project
		if reference != "" {
			spec += "@" + reference
		}
		if len(files) > 0 {
			spec += " " + strings.Join(files, ", ")
		}
		add(spec, project+"@"+reference, kindProject, v.Line)
	case yamlnode.Get(n, "template") != nil:
		v := yamlnode.Get(n, "template")
		add("include template: "+yamlnode.Text(v), yamlnode.Text(v), kindTemplate, v.Line)
	case yamlnode.Get(n, "remote") != nil:
		v := yamlnode.Get(n, "remote")
		add("include remote: "+yamlnode.Text(v), yamlnode.Text(v), kindRemote, v.Line)
	case yamlnode.Get(n, "component") != nil:
		v := yamlnode.Get(n, "component")
		add("include component: "+yamlnode.Text(v), yamlnode.Text(v), kindComponent, v.Line)
	}
}

// addImages records the image a job runs in and the services beside it. owner names
// the job for the import's label; "" is the pipeline-wide setting.
//
// Implements: REQ-CI-007
func addImages(extraction *lang.Extraction, n *yaml.Node, owner string) {
	if n == nil {
		return
	}
	label := func(what string) string {
		if owner == "" {
			return what
		}
		return owner + " " + what
	}
	add := func(v *yaml.Node, what string) {
		if v == nil {
			return
		}
		reference := yamlnode.Text(v)
		if reference == "" { // image: { name: …, entrypoint: … }
			if name := yamlnode.Get(v, "name"); name != nil {
				reference, v = yamlnode.Text(name), name
			}
		}
		if reference != "" && !strings.Contains(reference, "$") { // a variable we cannot expand
			extraction.Imports = append(extraction.Imports, lang.RawImport{
				Spec: label(what) + ": " + reference, Module: reference, Name: kindImage, Line: v.Line,
			})
		}
	}
	add(yamlnode.Get(n, "image"), "image")
	for _, service := range yamlnode.List(yamlnode.Get(n, "services")) {
		add(service, "service")
	}
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

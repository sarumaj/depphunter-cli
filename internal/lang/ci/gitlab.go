package ci

import (
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// reserved are the top-level keys that configure the pipeline; everything else at that
// level is a job, including the hidden ".template" ones jobs extend.
var reserved = map[string]bool{
	"image": true, "services": true, "stages": true, "types": true, "before_script": true,
	"after_script": true, "variables": true, "cache": true, "include": true,
	"default": true, "workflow": true,
}

// extractGitLab reads a GitLab pipeline: its jobs are the symbols, and its includes,
// components, templates and images are the dependencies.
func extractGitLab(root *yaml.Node) *lang.Extraction {
	ex := &lang.Extraction{}
	for _, inc := range items(field(root, "include")) {
		addInclude(ex, inc)
	}
	addImages(ex, root, "")
	addImages(ex, field(root, "default"), "default")

	for _, job := range pairs(root) {
		name := text(job.key)
		if name == "" || reserved[name] || mapping(job.value) == nil {
			continue
		}
		ex.Symbols = append(ex.Symbols, lang.Symbol{Name: name, Kind: "job", Line: job.key.Line})
		addImages(ex, job.value, name)
		// A bridge job runs another pipeline, which is a dependency like any include.
		for _, inc := range items(field(field(job.value, "trigger"), "include")) {
			addInclude(ex, inc)
		}
	}
	return ex
}

// addInclude records one include entry in any of the forms GitLab accepts.
func addInclude(ex *lang.Extraction, n *yaml.Node) {
	if n == nil {
		return
	}
	add := func(spec, module, kind string, line int) {
		if module != "" {
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
		}
	}
	if s := text(n); s != "" { // include: "path" or a bare URL
		kind := kindLocal
		if isURL(s) {
			kind = kindRemote
		}
		add("include: "+s, s, kind, n.Line)
		return
	}
	switch {
	case field(n, "local") != nil:
		v := field(n, "local")
		add("include local: "+text(v), text(v), kindLocal, v.Line)
	case field(n, "project") != nil:
		v := field(n, "project")
		project, ref := text(v), text(field(n, "ref"))
		files := []string{}
		for _, f := range items(field(n, "file")) {
			files = append(files, text(f))
		}
		spec := "include project: " + project
		if ref != "" {
			spec += "@" + ref
		}
		if len(files) > 0 {
			spec += " " + strings.Join(files, ", ")
		}
		add(spec, project+"@"+ref, kindProject, v.Line)
	case field(n, "template") != nil:
		v := field(n, "template")
		add("include template: "+text(v), text(v), kindTemplate, v.Line)
	case field(n, "remote") != nil:
		v := field(n, "remote")
		add("include remote: "+text(v), text(v), kindRemote, v.Line)
	case field(n, "component") != nil:
		v := field(n, "component")
		add("include component: "+text(v), text(v), kindComponent, v.Line)
	}
}

// addImages records the image a job runs in and the services beside it. owner names
// the job for the import's label; "" is the pipeline-wide setting.
func addImages(ex *lang.Extraction, n *yaml.Node, owner string) {
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
		ref := text(v)
		if ref == "" { // image: { name: …, entrypoint: … }
			if name := field(v, "name"); name != nil {
				ref, v = text(name), name
			}
		}
		if ref != "" && !strings.Contains(ref, "$") { // a variable we cannot expand
			ex.Imports = append(ex.Imports, lang.RawImport{
				Spec: label(what) + ": " + ref, Module: ref, Name: kindImage, Line: v.Line,
			})
		}
	}
	add(field(n, "image"), "image")
	for _, svc := range items(field(n, "services")) {
		add(svc, "service")
	}
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

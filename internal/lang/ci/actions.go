package ci

import (
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// extractWorkflow reads a GitHub Actions workflow: its jobs are the symbols, and every
// step's uses:, every reusable workflow and every container image is a dependency.
//
// Implements: REQ-CI-002, REQ-CI-003, REQ-CI-010
func extractWorkflow(root *yaml.Node) *lang.Extraction {
	ex := &lang.Extraction{}
	for _, job := range pairs(field(root, "jobs")) {
		name := text(job.key)
		if name == "" {
			continue
		}
		ex.Symbols = append(ex.Symbols, lang.Symbol{Name: name, Kind: "job", Line: job.key.Line})
		// A job that calls a reusable workflow has no steps of its own.
		if u := field(job.value, "uses"); u != nil {
			addUses(ex, u, kindWorkflow)
		}
		addContainers(ex, job.value)
		for _, step := range items(field(job.value, "steps")) {
			if u := field(step, "uses"); u != nil {
				addUses(ex, u, kindAction)
			}
		}
	}
	return ex
}

// extractAction reads a composite action's own steps; a JavaScript or Docker action
// declares what it runs in runs.image.
//
// Implements: REQ-CI-002, REQ-CI-004
func extractAction(root *yaml.Node) *lang.Extraction {
	ex := &lang.Extraction{}
	runs := field(root, "runs")
	for _, step := range items(field(runs, "steps")) {
		if u := field(step, "uses"); u != nil {
			addUses(ex, u, kindAction)
		}
	}
	// A Docker action names its image here, with the same docker:// prefix as a step.
	if img := field(runs, "image"); img != nil {
		if ref := strings.TrimPrefix(text(img), "docker://"); ref != "" && !strings.HasSuffix(ref, "Dockerfile") {
			ex.Imports = append(ex.Imports, lang.RawImport{
				Spec: "image: " + text(img), Module: ref, Name: kindImage, Line: img.Line,
			})
		}
	}
	return ex
}

// addUses records a step's or job's uses:, which is either an action in another
// repository, a path inside this one, or a container image.
//
// Implements: REQ-CI-002, REQ-CI-003, REQ-CI-004, REQ-CI-014
func addUses(ex *lang.Extraction, n *yaml.Node, kind string) {
	ref := text(n)
	if ref == "" {
		return
	}
	imp := lang.RawImport{Spec: "uses: " + ref, Module: ref, Name: kind, Line: n.Line}
	switch {
	case strings.HasPrefix(ref, "./"), strings.HasPrefix(ref, "../"):
		imp.Name = kindLocal
	case strings.HasPrefix(ref, "docker://"):
		imp.Module, imp.Name = strings.TrimPrefix(ref, "docker://"), kindImage
	default:
		// "actions/checkout@abc123… # v4.1.1": pinning to a commit is what the
		// hardening guides ask for, and it leaves the readable version in a comment.
		if c := comment(n); c != "" {
			imp.Spec, imp.Name = imp.Spec+" # "+c, kind+"\x00"+c
		}
	}
	ex.Imports = append(ex.Imports, imp)
}

// addContainers records the images a job runs in and the services beside it.
//
// Implements: REQ-CI-004
func addContainers(ex *lang.Extraction, job *yaml.Node) {
	add := func(n *yaml.Node, label string) {
		if n == nil {
			return
		}
		ref := text(n)
		if ref == "" { // container: { image: … }
			if img := field(n, "image"); img != nil {
				ref, n = text(img), img
			}
		}
		if ref != "" {
			ex.Imports = append(ex.Imports, lang.RawImport{
				Spec: label + ": " + ref, Module: ref, Name: kindImage, Line: n.Line,
			})
		}
	}
	add(field(job, "container"), "container")
	for _, svc := range pairs(field(job, "services")) {
		add(svc.value, "service "+text(svc.key))
	}
}

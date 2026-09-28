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
	extraction := &lang.Extraction{}
	for _, job := range pairs(field(root, "jobs")) {
		name := text(job.key)
		if name == "" {
			continue
		}
		extraction.Symbols = append(extraction.Symbols, lang.Symbol{Name: name, Kind: "job", Line: job.key.Line})
		// A job that calls a reusable workflow has no steps of its own.
		if u := field(job.value, "uses"); u != nil {
			addUses(extraction, u, kindWorkflow)
		}
		addContainers(extraction, job.value)
		for _, step := range items(field(job.value, "steps")) {
			if u := field(step, "uses"); u != nil {
				addUses(extraction, u, kindAction)
			}
		}
	}
	return extraction
}

// extractAction reads a composite action's own steps; a JavaScript or Docker action
// declares what it runs in runs.image.
//
// Implements: REQ-CI-002, REQ-CI-004
func extractAction(root *yaml.Node) *lang.Extraction {
	extraction := &lang.Extraction{}
	runs := field(root, "runs")
	for _, step := range items(field(runs, "steps")) {
		if u := field(step, "uses"); u != nil {
			addUses(extraction, u, kindAction)
		}
	}
	// A Docker action names its image here, with the same docker:// prefix as a step.
	if image := field(runs, "image"); image != nil {
		if reference := strings.TrimPrefix(text(image), "docker://"); reference != "" && !strings.HasSuffix(reference, "Dockerfile") {
			extraction.Imports = append(extraction.Imports, lang.RawImport{
				Spec: "image: " + text(image), Module: reference, Name: kindImage, Line: image.Line,
			})
		}
	}
	return extraction
}

// addUses records a step's or job's uses:, which is either an action in another
// repository, a path inside this one, or a container image.
//
// Implements: REQ-CI-002, REQ-CI-003, REQ-CI-004, REQ-CI-014
func addUses(extraction *lang.Extraction, n *yaml.Node, kind string) {
	reference := text(n)
	if reference == "" {
		return
	}
	rawImport := lang.RawImport{Spec: "uses: " + reference, Module: reference, Name: kind, Line: n.Line}
	switch {
	case strings.HasPrefix(reference, "./"), strings.HasPrefix(reference, "../"):
		rawImport.Name = kindLocal
	case strings.HasPrefix(reference, "docker://"):
		rawImport.Module, rawImport.Name = strings.TrimPrefix(reference, "docker://"), kindImage
	default:
		// "actions/checkout@abc123… # v4.1.1": pinning to a commit is what the
		// hardening guides ask for, and it leaves the readable version in a comment.
		if c := comment(n); c != "" {
			rawImport.Spec, rawImport.Name = rawImport.Spec+" # "+c, kind+"\x00"+c
		}
	}
	extraction.Imports = append(extraction.Imports, rawImport)
}

// addContainers records the images a job runs in and the services beside it.
//
// Implements: REQ-CI-004
func addContainers(extraction *lang.Extraction, job *yaml.Node) {
	add := func(n *yaml.Node, label string) {
		if n == nil {
			return
		}
		reference := text(n)
		if reference == "" { // container: { image: … }
			if image := field(n, "image"); image != nil {
				reference, n = text(image), image
			}
		}
		if reference != "" {
			extraction.Imports = append(extraction.Imports, lang.RawImport{
				Spec: label + ": " + reference, Module: reference, Name: kindImage, Line: n.Line,
			})
		}
	}
	add(field(job, "container"), "container")
	for _, service := range pairs(field(job, "services")) {
		add(service.value, "service "+text(service.key))
	}
}

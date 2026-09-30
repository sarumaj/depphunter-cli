package ci

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/oci"
	"github.com/sarumaj/depphunter-cli/internal/lang/yamlnode"
)

// ActionDependencies reads what an action of another repository runs, from its
// metadata file (action.yml), as the plugin reads a composite action in this one:
// a composite action's steps' uses:, a Docker action's image. A JavaScript action
// runs no other action. dockerfile is the Dockerfile a Docker action is built from,
// relative to the metadata file's directory, whose FROM names the image instead.
//
// A step's `./path` names a directory of the workspace the calling workflow checked
// out, not of the action's repository, so it is not an answer here.
//
// Implements: REQ-CI-016
func ActionDependencies(source []byte) (dependencies []lang.Target, dockerfile string) {
	root := yamlReader.Mapping(yamlnode.Parse(source))
	if root == nil {
		return nil, ""
	}
	for _, rawImport := range extractAction(root).Imports {
		if t := remoteTarget(rawImport, "", ""); t.Package != "" {
			dependencies = append(dependencies, t)
		}
	}
	if image := yamlReader.Text(root, "runs", "image"); strings.EqualFold(yamlReader.Text(root, "runs", "using"), "docker") &&
		!strings.HasPrefix(image, "docker://") && strings.HasSuffix(image, "Dockerfile") {
		dockerfile = image
	}
	return dependencies, dockerfile
}

// WorkflowDependencies reads what a reusable workflow of repository, at reference,
// runs: its steps' actions, the workflows its jobs call and the images they run in.
// A job's `./.github/workflows/<file>` is a workflow of the same repository at the
// same reference, which is how GitHub runs it; a step's `./path` is the calling
// workflow's workspace, and is left out.
//
// Implements: REQ-CI-016
func WorkflowDependencies(source []byte, repository, reference string) []lang.Target {
	root := yamlReader.Mapping(yamlnode.Parse(source))
	if root == nil {
		return nil
	}
	var out []lang.Target
	for _, rawImport := range extractWorkflow(root).Imports {
		if t := remoteTarget(rawImport, repository, reference); t.Package != "" {
			out = append(out, t)
		}
	}
	return out
}

// remoteTarget resolves a reference read from a file of repository at reference,
// which has no files of this repository to resolve a local path against.
func remoteTarget(rawImport lang.RawImport, repository, reference string) lang.Target {
	kind, requested, _ := strings.Cut(rawImport.Name, "\x00")
	switch kind {
	case kindImage:
		return oci.Image(rawImport.Module)
	case kindAction, kindWorkflow:
		return action(rawImport.Module, requested)
	case kindLocal:
		p := path.Clean(strings.TrimPrefix(rawImport.Module, "./"))
		extension := path.Ext(p)
		if repository == "" || reference == "" || !strings.HasPrefix(p, ".github/workflows/") ||
			extension != ".yml" && extension != ".yaml" {
			return lang.Target{}
		}
		return action(repository+"/"+p+"@"+reference, "")
	}
	return lang.Target{}
}

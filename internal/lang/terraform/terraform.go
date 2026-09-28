// Package terraform analyzes Terraform and OpenTofu configurations (.tf, .tofu,
// .tf.json), variable files (.tfvars), dependency lock files (.terraform.lock.hcl) and
// Terragrunt configurations (terragrunt.hcl and the .hcl files it includes).
//
// A Terraform module is a directory: its files are read together. A module call
// resolves to the directory of a local module, else to the terraform-module island -
// a registry module (namespace/name/provider, its host kept unless it is the public
// Terraform or OpenTofu registry) or a module fetched from git, a web server or a
// bucket, named by its normalized URL. The providers a module requires, configures
// or uses by a resource type's prefix make the terraform-provider island, pinned by
// the lock file. References to variables, locals, modules, data sources and resources
// declared in another file of the module, and the files file() and templatefile()
// read, are edges to those files (resolve.go).
//
// HCL is read by a scanner of its own (hcl.go), not the vendored tree-sitter
// grammar, which read every sampled file correctly but took twenty to fifty times
// longer (REQ-TERRAFORM-011).
package terraform

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoModule   = "terraform-module"
	ecoProvider = "terraform-provider"
)

// The kinds of file, as Class names them.
const (
	classConfig     = "config"     // .tf, .tofu
	classJSON       = "json"       // .tf.json, .tofu.json
	classVars       = "vars"       // .tfvars
	classVarsJSON   = "vars-json"  // .tfvars.json
	classLock       = "lock"       // .terraform.lock.hcl
	classTerragrunt = "terragrunt" // any other .hcl
)

// Implements: REQ-TERRAFORM-001
type Plugin struct{}

func (Plugin) Name() string { return "terraform" }
func (Plugin) Version() int { return 1 }

// fileClass says which kind of file p is, "" for none of them.
func fileClass(p string) string {
	base := strings.ToLower(path.Base(p))
	switch {
	case strings.HasSuffix(base, ".tf") || strings.HasSuffix(base, ".tofu"):
		return classConfig
	case strings.HasSuffix(base, ".tf.json") || strings.HasSuffix(base, ".tofu.json"):
		return classJSON
	case strings.HasSuffix(base, ".tfvars"):
		return classVars
	case strings.HasSuffix(base, ".tfvars.json"):
		return classVarsJSON
	case base == ".terraform.lock.hcl":
		return classLock
	case strings.HasSuffix(base, ".hcl") && !strings.HasSuffix(base, ".pkr.hcl") && !strings.HasSuffix(base, ".nomad.hcl"):
		return classTerragrunt
	}
	return ""
}

// ignored is where Terraform and Terragrunt keep what they download.
func ignored(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == ".terraform" || seg == ".terragrunt-cache" {
			return true
		}
	}
	return false
}

// Claims takes Terraform and OpenTofu configurations (.tf, .tofu and their .json
// forms), variable files (.tfvars, .tfvars.json), dependency lock files and other
// .hcl files as Terragrunt configurations (Packer's .pkr.hcl and Nomad's .nomad.hcl
// excepted), outside the .terraform and .terragrunt-cache directories.
//
// Implements: REQ-TERRAFORM-001
func (Plugin) Claims(f *scan.File) bool {
	return !f.Binary && !ignored(f.Path) && fileClass(f.Path) != ""
}

// Class tells the kinds of file apart where they share an extension (.tf.json and
// .tfvars.json, .terraform.lock.hcl and terragrunt.hcl).
//
// Implements: REQ-TERRAFORM-001
func (Plugin) Class(f *scan.File) string { return fileClass(f.Path) }

// Implements: REQ-TERRAFORM-004, REQ-TERRAFORM-007
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoModule, Name: "Terraform modules"},
		{ID: ecoProvider, Name: "Terraform providers"},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Implements: REQ-TERRAFORM-001, REQ-TERRAFORM-011
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	return read(fileClass(f.Path), src).extraction(), nil
}

func read(class string, src []byte) *fileInfo {
	switch class {
	case classJSON:
		return readModule(parseJSON(src))
	case classVars:
		return readVars(parse(src))
	case classVarsJSON:
		return readVars(parseJSONVars(src))
	case classLock:
		return readLock(parse(src))
	case classTerragrunt:
		return readTerragrunt(parse(src))
	}
	return readModule(parse(src))
}

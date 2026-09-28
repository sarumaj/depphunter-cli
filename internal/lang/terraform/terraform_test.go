package terraform

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/cache"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// A root module split over several files (required_providers with a public, an
// OpenTofu-registry and a private-registry provider, aliased provider configurations,
// local, registry, private-registry, git, GitHub, scp-like and S3 module sources,
// implicit providers, the built-in terraform provider, templatefile and file, a
// heredoc, a moved block, a for expression) with its lock file and a tfvars file in
// both syntaxes; a local module called by it (pinned through the root's lock, a
// dynamic block's iterator); a legacy module (provider version, bare directory
// source); a module in JSON syntax calling an OpenTofu module; and a Terragrunt live
// tree (root.hcl, an _envcommon include whose locals build the source, dependency
// and dependencies blocks, a tfr:// source, a local source with a //subdirectory).
//
// Verifies: REQ-TERRAFORM-001, REQ-TERRAFORM-002, REQ-TERRAFORM-003, REQ-TERRAFORM-004, REQ-TERRAFORM-005
// Verifies: REQ-TERRAFORM-006, REQ-TERRAFORM-007, REQ-TERRAFORM-008, REQ-TERRAFORM-009, REQ-TERRAFORM-010
func TestModulesProvidersAndTerragrunt(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	aws := lang.Target{Ecosystem: ecosystemProvider, Package: "hashicorp/aws", Version: "5.31.0", Requested: "~> 5.0", Pinned: true}
	random := lang.Target{Ecosystem: ecosystemProvider, Package: "hashicorp/random", Version: "3.6.0", Pinned: true}
	cloudflare := lang.Target{Ecosystem: ecosystemProvider, Package: "cloudflare/cloudflare", Version: ">= 4.0, < 5.0"}
	vpc := lang.Target{Ecosystem: ecosystemModule, Package: "terraform-aws-modules/vpc/aws", Version: "5.1.2", Pinned: true}
	local := func(p string) lang.Target { return lang.Target{Local: p} }
	imports := map[string]map[string]lang.Target{
		".terraform.lock.hcl": {
			`provider "registry.terraform.io/hashicorp/aws"`:    aws,
			`provider "registry.opentofu.org/hashicorp/random"`: random,
		},
		"data.tf": {"provider aws (aws_ami)": aws},
		"locals.tf": {
			"random_id.suffix":    local("main.tf"),
			`file("policy.json")`: local("policy.json"),
		},
		"main.tf": {
			"required_providers aws":    aws,
			"required_providers random": random,
			"required_providers acme":   {Ecosystem: ecosystemProvider, Package: "tf.corp.test/acme/acme", Version: "1.4.0", Pinned: true},
			`provider "aws"`:            aws,
			"var.region":                local("variables.tf"),
			"var.cidr":                  local("variables.tf"),
			"var.subnets":               local("variables.tf"),
			"data.aws_ami.ubuntu":       local("data.tf"),
			"local.name":                local("locals.tf"),
			`module "network"`:          local("modules/network"),
			`module "vpc"`:              vpc,
			`module "iam"`:              {Ecosystem: ecosystemModule, Package: "terraform-aws-modules/iam/aws//modules/iam-role", Version: "~> 6.0"},
			`module "label"`:            {Ecosystem: ecosystemModule, Package: "cloudposse/label/null", Floating: true},
			`module "private"`:          {Ecosystem: ecosystemModule, Package: "app.terraform.io/acme/db/aws", Version: "2.0.1", Pinned: true},
			`module "storage"`: {Ecosystem: ecosystemModule, Package: "github.com/acme/tf-modules//storage", Version: "v1.2.0",
				Origin: "git::https://github.com/acme/tf-modules.git//storage"},
			`module "dns"`: {Ecosystem: ecosystemModule, Package: "github.com/acme/tf-dns", Version: "0123456789abcdef0123456789abcdef01234567",
				Pinned: true, Origin: "github.com/acme/tf-dns"},
			`module "queue"`: {Ecosystem: ecosystemModule, Package: "github.com/acme/tf-queue", Floating: true, Origin: "git@github.com:acme/tf-queue.git"},
			`module "cdn"`: {Ecosystem: ecosystemModule, Package: "s3-eu-west-1.amazonaws.com/acme-modules/cdn.zip", Floating: true,
				Origin: "s3::https://s3-eu-west-1.amazonaws.com/acme-modules/cdn.zip"},
			`templatefile("${path.module}/templates/init.sh.tpl")`: local("templates/init.sh.tpl"),
			"provider google (google_storage_bucket)":              {Ecosystem: ecosystemProvider, Package: "hashicorp/google", Floating: true},
		},
		"outputs.tf": {
			"aws_instance.web": local("main.tf"),
			"local.name":       local("locals.tf"),
			"module.network":   local("main.tf"),
		},
		"modules/network/main.tf": {
			"provider aws (aws_subnet)":          aws,
			"var.cidr":                           local("modules/network/variables.tf"),
			"var.tags":                           local("modules/network/variables.tf"),
			`file("${path.module}/policy.json")`: local("modules/network/policy.json"),
		},
		"modules/network/outputs.tf": {"aws_subnet.this": local("modules/network/main.tf")},
		"modules/legacy/main.tf": {
			`provider "aws"`:     {Ecosystem: ecosystemProvider, Package: "hashicorp/aws", Version: "~> 2.0"},
			`module "old_style"`: local("modules/legacy/network"),
		},
		"modules/dns/main.tf.json": {
			"required_providers cloudflare": cloudflare,
			"cloudflare_zone.main":          local("modules/dns/zones.tf"),
			`module "cdn"`:                  local("modules/cdn"),
		},
		"modules/dns/zones.tf": {
			"provider cloudflare (cloudflare_zone)": cloudflare,
			"var.domain":                            local("modules/dns/main.tf.json"),
		},
		"modules/cdn/main.tofu": {},
		"live/prod/app/terragrunt.hcl": {
			`find_in_parent_folders("root.hcl")`:                                local("live/root.hcl"),
			`${dirname(find_in_parent_folders("root.hcl"))}/_envcommon/app.hcl`: local("live/_envcommon/app.hcl"),
			"terraform.source": {Ecosystem: ecosystemModule, Package: "github.com/acme/infra-modules//app", Version: "v0.8.0",
				Origin: "git::git@github.com:acme/infra-modules.git//app"},
			`dependency "vpc"`:                  local("live/prod/vpc/terragrunt.hcl"),
			`dependencies "../dns"`:             local("live/prod/dns/terragrunt.hcl"),
			`find_in_parent_folders("env.hcl")`: local("live/prod/env.hcl"),
		},
		"live/prod/vpc/terragrunt.hcl": {
			`find_in_parent_folders("root.hcl")`: local("live/root.hcl"),
			"terraform.source":                   vpc,
		},
		"live/prod/dns/terragrunt.hcl": {"terraform.source": local("modules/dns")},
		"terraform.tfvars":             {},
		"prod.tfvars.json":             {},
	}
	for file, want := range imports {
		t.Run(file, func(t *testing.T) { langtest.CheckImports(t, results[file], want) })
	}

	symbols := map[string]map[string]string{
		"main.tf": {
			"provider.aws": "provider", "provider.aws.west": "provider",
			"module.network": "module", "module.vpc": "module", "module.iam": "module", "module.label": "module",
			"module.private": "module", "module.storage": "module", "module.dns": "module", "module.queue": "module",
			"module.cdn": "module", "aws_instance.web": "resource", "random_id.suffix": "resource",
			"acme_thing.x": "resource", "google_storage_bucket.b": "resource", "terraform_data.noop": "resource",
		},
		"data.tf":          {"data.aws_ami.ubuntu": "data", "data.terraform_remote_state.net": "data"},
		"variables.tf":     {"var.region": "variable", "var.cidr": "variable", "var.subnets": "variable"},
		"locals.tf":        {"local.name": "local", "local.policy": "local"},
		"outputs.tf":       {"output.ip": "output", "output.banner": "output", "output.subnet": "output"},
		"terraform.tfvars": {"region": "value", "cidr": "value", "subnets": "value"},
		"prod.tfvars.json": {"region": "value", "cidr": "value"},
		"modules/dns/main.tf.json": {
			"var.domain": "variable", "cloudflare_record.www": "resource", "module.cdn": "module",
		},
		"modules/cdn/main.tofu": {"var.origin": "variable", "output.origin": "output"},
		"live/prod/app/terragrunt.hcl": {
			"include.root": "include", "include.envcommon": "include", "dependency.vpc": "dependency", "local.env": "local",
		},
		"live/prod/vpc/terragrunt.hcl": {"include": "include"},
		"live/_envcommon/app.hcl":      {"local.base_source_url": "local"},
	}
	for file, want := range symbols {
		t.Run("symbols "+file, func(t *testing.T) { langtest.CheckSymbols(t, results[file], want) })
	}
	lines := map[string]int{}
	for _, s := range results["main.tf"].Symbols {
		lines[s.Name] = s.Line
	}
	if lines["aws_instance.web"] != 65 || lines["module.vpc"] != 30 {
		t.Errorf("lines: %v", lines)
	}
	lines = map[string]int{}
	for _, s := range results["modules/dns/main.tf.json"].Symbols {
		lines[s.Name] = s.Line
	}
	if lines["cloudflare_record.www"] != 15 || lines["module.cdn"] != 22 {
		t.Errorf("JSON lines: %v", lines)
	}
}

// Module source addresses in every form go-getter, the registries and Terragrunt
// know, and what each is named.
//
// Verifies: REQ-TERRAFORM-004
func TestModuleSources(t *testing.T) {
	for source, want := range map[string]moduleSource{
		"./modules/x":                     {local: "./modules/x"},
		"../shared":                       {local: "../shared"},
		"hashicorp/consul/aws":            {packageName: "hashicorp/consul/aws"},
		"Terraform-AWS-Modules/VPC/aws":   {packageName: "terraform-aws-modules/vpc/aws"},
		"registry.opentofu.org/a/b/aws":   {packageName: "a/b/aws"},
		"example.com:8443/a/b/aws//sub/":  {packageName: "example.com:8443/a/b/aws//sub"},
		"tfr:///a/b/aws?version=1.0.0":    {packageName: "a/b/aws", reference: "1.0.0"},
		"tfr://tf.corp.test/a/b/aws":      {packageName: "tf.corp.test/a/b/aws"},
		"github.com/org/repo//sub?ref=v1": {packageName: "github.com/org/repo//sub", reference: "v1", origin: "github.com/org/repo//sub"},
		"bitbucket.org/org/repo":          {packageName: "bitbucket.org/org/repo", origin: "bitbucket.org/org/repo"},
		"git::ssh://git@Example.com:2222/org/repo.git?ref=main": {packageName: "example.com/org/repo", reference: "main",
			origin: "git::ssh://git@Example.com:2222/org/repo.git"},
		"git::https://user:pw@git.corp.test/infra/modules.git//vpc?ref=v2&depth=1": {packageName: "git.corp.test/infra/modules//vpc",
			reference: "v2", origin: "git::https://user:pw@git.corp.test/infra/modules.git//vpc"},
		"hg::http://example.com/vpc.hg?ref=v1": {packageName: "example.com/vpc.hg", reference: "v1", origin: "hg::http://example.com/vpc.hg"},
		"https://example.com/vpc-module.zip":   {packageName: "example.com/vpc-module.zip", origin: "https://example.com/vpc-module.zip", archive: true},
		"https://example.com/vpc?archive=zip":  {packageName: "example.com/vpc", origin: "https://example.com/vpc", archive: true},
		"gcs::https://www.googleapis.com/storage/v1/modules/foo.zip": {packageName: "www.googleapis.com/storage/v1/modules/foo.zip",
			origin: "gcs::https://www.googleapis.com/storage/v1/modules/foo.zip", archive: true},
	} {
		got, ok := parseSource(source)
		if !ok || got != want {
			t.Errorf("%s: got %+v (%v), want %+v", source, got, ok, want)
		}
	}
	for _, source := range []string{"", "hashicorp/consul", "/abs/path"} {
		if got, ok := parseSource(source); ok {
			t.Errorf("%s: got %+v, want nothing", source, got)
		}
	}
}

// Verifies: REQ-TERRAFORM-007
func TestProviderSources(t *testing.T) {
	for in, want := range map[string]string{
		"hashicorp/aws":                          "hashicorp/aws",
		"registry.terraform.io/hashicorp/aws":    "hashicorp/aws",
		"registry.opentofu.org/Hashicorp/Aws":    "hashicorp/aws",
		"aws":                                    "hashicorp/aws",
		"tf.corp.test/acme/thing":                "tf.corp.test/acme/thing",
		"terraform.io/builtin/terraform":         "terraform.io/builtin/terraform",
		" registry.terraform.io/datadog/datadog": "datadog/datadog",
	} {
		if got := providerSource(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

// The scanner keeps going past what it does not understand, reads strings and
// heredocs as templates (escapes, $${ and %{ directives), one-line blocks, and
// comments of all three kinds.
//
// Verifies: REQ-TERRAFORM-011
func TestScanner(t *testing.T) {
	// cSpell: ignore: ufeff
	source := []byte("\ufeff# comment aws_x.y\n// another\n/* multi\nline */\n" +
		"locals { a = \"x\\\"${var.one}\" }\n" +
		"??? garbage {\n  b = 1\n}\n" +
		"resource \"t_x\" \"n\" {\n  c = <<EOT\n  $${literal} %{ if local.z }yes%{ endif }\n  EOT\n  d = [for k in var.two : k.v]\n}\n" +
		"output \"o\" { value = t_x.n.id }\n")
	root := parse(source)
	var types []string
	for _, b := range root.blocks {
		types = append(types, b.typeName)
	}
	if want := []string{"locals", "resource", "output"}; !reflect.DeepEqual(types, want) {
		t.Fatalf("blocks %v, want %v", types, want)
	}
	if a := root.blocks[0].attributes[0]; a.name != "a" || a.line != 5 || a.expression[0].text != `x"${var.one}` || a.expression[0].literal {
		t.Errorf("string: %+v", a)
	}
	r := root.blocks[1]
	if r.line != 9 || len(r.attributes) != 2 || r.attributes[1].name != "d" || r.attributes[1].line != 13 {
		t.Errorf("resource: %+v", r)
	}
	if o := root.blocks[2]; o.line != 15 || len(o.attributes) != 1 {
		t.Errorf("one-line block: %+v", o)
	}
	fileInfo := readModule(root)
	var references []string
	for _, rawImport := range fileInfo.imports {
		if rawImport.Name == kindReference {
			references = append(references, rawImport.Module)
		}
	}
	if want := []string{"var.one", "local.z", "var.two"}; !reflect.DeepEqual(references, want) {
		t.Errorf("refs %v, want %v", references, want)
	}
}

// Verifies: REQ-TERRAFORM-001
func TestClaimsAndClasses(t *testing.T) {
	for p, want := range map[string]string{
		"main.tf": classConfig, "x/main.tofu": classConfig, "main.tf.json": classJSON, "a.tfvars": classVariables,
		"a.auto.tfvars.json": classVariablesJSON, ".terraform.lock.hcl": classLock, "live/terragrunt.hcl": classTerragrunt,
		"root.hcl": classTerragrunt, "image.pkr.hcl": "", "job.nomad.hcl": "", "package.json": "", "main.go": "",
	} {
		if got := fileClass(p); got != want {
			t.Errorf("%s: class %q, want %q", p, got, want)
		}
		if got := (Plugin{}).Claims(&scan.File{Path: p}); got != (want != "") {
			t.Errorf("%s: claimed %v", p, got)
		}
	}
	for _, p := range []string{".terraform/modules/vpc/main.tf", "live/.terragrunt-cache/x/main.tf"} {
		if (Plugin{}).Claims(&scan.File{Path: p}) {
			t.Errorf("%s: claimed", p)
		}
	}
	key := func(p string) string {
		return cache.Key(Plugin{}.Name(), Plugin{}.Version(), lang.ClassOf(Plugin{}, &scan.File{Path: p}), []byte("{}"))
	}
	if key("a.tf.json") == key("a.tfvars.json") || key(".terraform.lock.hcl") == key("terragrunt.hcl") {
		t.Error("files read differently share a cache key")
	}
}

// .terraform/modules/modules.json records what `terraform init` installed: a
// registry call its constraint leaves open is pinned at the installed version
// (in a called local module too, through its caller's file), and the installed
// module's own calls, through its local modules, are its dependencies. A garbage
// or missing file changes nothing.
//
// Verifies: REQ-TERRAFORM-008, REQ-TERRAFORM-009
func TestInstalledModules(t *testing.T) {
	files := map[string]string{
		"main.tf": `module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"
}
module "net" {
  source = "./modules/net"
}
module "label" {
  source = "cloudposse/label/null"
}
module "exact" {
  source  = "terraform-aws-modules/iam/aws"
  version = "5.2.0"
}
`,
		"modules/net/main.tf": `module "sg" {
  source  = "terraform-aws-modules/security-group/aws"
  version = ">= 4"
}
`,
		".terraform/modules/modules.json": `{"Modules": [
  {"Key": "", "Source": "", "Dir": "."},
  {"Key": "exact", "Source": "registry.terraform.io/terraform-aws-modules/iam/aws", "Version": "5.3.0", "Dir": ".terraform/modules/exact"},
  {"Key": "label", "Source": "registry.terraform.io/cloudposse/label/null", "Version": "0.25.0", "Dir": ".terraform/modules/label"},
  {"Key": "net", "Source": "./modules/net", "Dir": "modules/net"},
  {"Key": "net.sg", "Source": "registry.terraform.io/terraform-aws-modules/security-group/aws", "Version": "4.17.2", "Dir": ".terraform/modules/net.sg"},
  {"Key": "vpc", "Source": "registry.terraform.io/terraform-aws-modules/vpc/aws", "Version": "5.1.2", "Dir": ".terraform/modules/vpc"},
  {"Key": "vpc.git", "Source": "git::https://github.com/acme/tf-dns.git?ref=0123456789abcdef0123456789abcdef01234567", "Dir": ".terraform/modules/vpc.git"},
  {"Key": "vpc.inner", "Source": "./modules/inner", "Dir": ".terraform/modules/vpc/modules/inner"},
  {"Key": "vpc.inner.label", "Source": "registry.terraform.io/cloudposse/label/null", "Version": "0.25.0", "Dir": ".terraform/modules/vpc.inner.label"},
  {"Key": "vpc.inner.label.deep", "Source": "registry.terraform.io/acme/deep/null", "Version": "1.0.0", "Dir": ".terraform/modules/deep"}
]}`,
	}
	vpc := lang.Target{Ecosystem: ecosystemModule, Package: "terraform-aws-modules/vpc/aws", Version: "5.1.2", Requested: "~> 5.0", Pinned: true}
	label := lang.Target{Ecosystem: ecosystemModule, Package: "cloudposse/label/null", Version: "0.25.0", Pinned: true}
	exact := lang.Target{Ecosystem: ecosystemModule, Package: "terraform-aws-modules/iam/aws", Version: "5.2.0", Pinned: true}
	sg := lang.Target{Ecosystem: ecosystemModule, Package: "terraform-aws-modules/security-group/aws", Version: "4.17.2", Requested: ">= 4", Pinned: true}
	root := langtest.Write(t, files)
	results := langtest.Analyze(t, Plugin{}, root)
	langtest.CheckImports(t, results["main.tf"], map[string]lang.Target{
		`module "vpc"`:   vpc,
		`module "net"`:   {Local: "modules/net"},
		`module "label"`: label,
		`module "exact"`: exact, // the configuration's exact version stands
	})
	langtest.CheckImports(t, results["modules/net/main.tf"], map[string]lang.Target{`module "sg"`: sg})
	r := newResolver(root, langtest.Files(t, root))
	dns := lang.Target{Ecosystem: ecosystemModule, Package: "github.com/acme/tf-dns", Version: "0123456789abcdef0123456789abcdef01234567",
		Pinned: true, Origin: "git::https://github.com/acme/tf-dns.git"}
	if got, want := r.Dependencies(vpc), []lang.Target{dns, label}; !reflect.DeepEqual(got, want) || !r.Installed(vpc) {
		t.Errorf("vpc depends on %+v, want %+v", got, want)
	}
	if got := r.Dependencies(label); got != nil || !r.Installed(label) {
		t.Errorf("label depends on %+v", got)
	}
	other := lang.Target{Ecosystem: ecosystemModule, Package: "terraform-aws-modules/vpc/aws", Version: "4.0.0"}
	if got := r.Dependencies(other); got != nil || r.Installed(other) || r.Installed(lang.Target{Ecosystem: ecosystemProvider, Package: "hashicorp/aws"}) {
		t.Errorf("another version: %+v", got)
	}

	for _, garbage := range []string{"{{ not json", `{"Modules": "x"}`, ""} {
		files[".terraform/modules/modules.json"] = garbage
		if garbage == "" {
			delete(files, ".terraform/modules/modules.json")
		}
		root := langtest.Write(t, files)
		langtest.CheckImports(t, langtest.Analyze(t, Plugin{}, root)["main.tf"], map[string]lang.Target{
			`module "vpc"`:   {Ecosystem: ecosystemModule, Package: "terraform-aws-modules/vpc/aws", Version: "~> 5.0"},
			`module "net"`:   {Local: "modules/net"},
			`module "label"`: {Ecosystem: ecosystemModule, Package: "cloudposse/label/null", Floating: true},
			`module "exact"`: exact,
		})
		if r := newResolver(root, langtest.Files(t, root)); r.Dependencies(vpc) != nil || r.Installed(vpc) {
			t.Errorf("%q: installed", garbage)
		}
	}
}

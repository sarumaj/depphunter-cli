package scan

import "testing"

// Verifies: REQ-LANG-015, REQ-DOCKER-001
func TestLanguageByName(t *testing.T) {
	for p, want := range map[string]string{
		"a.go":                           "Go",
		"x.py":                           "Python",
		"views/home.phtml":               "PHP",
		"Gemfile":                        "Ruby",
		"lib/tasks/db.rake":              "Ruby",
		"app.gemspec":                    "Ruby",
		"lib/shop/cart.ex":               "Elixir",
		"mix.exs":                        "Elixir",
		"include/state.hrl":              "Erlang",
		"rebar.config":                   "Erlang",
		"src/shop.app.src":               "Erlang",
		"R/cart.R":                       "R",
		"DESCRIPTION":                    "R",
		"renv.lock":                      "R",
		"vignettes/intro.Rmd":            "R Markdown",
		"report.qmd":                     "Quarto",
		"src/Data/Shop.hs":               "Haskell",
		"src/Data/Shop.hs-boot":          "Haskell",
		"doc/Tutorial.lhs":               "Haskell",
		"shop.cabal":                     "Cabal",
		"cabal.project":                  "Cabal",
		"stack.yaml":                     "Haskell",
		"infra/main.tf":                  "Terraform",
		"infra/main.tf.json":             "Terraform",
		"prod.tfvars":                    "Terraform",
		"modules/x/main.tofu":            "OpenTofu",
		".terraform.lock.hcl":            "Terraform",
		"live/terragrunt.hcl":            "Terragrunt",
		"live/root.hcl":                  "HCL",
		"api/v1/shop.proto":              "Protobuf",
		"buf.yaml":                       "Buf",
		"proto/buf.lock":                 "Buf",
		"buf.work.yaml":                  "Buf",
		"buf.gen.yaml":                   "Buf",
		"Dockerfile":                     "Docker",
		"build/Containerfile":            "Docker",
		"Dockerfile.dev":                 "Docker",
		"api.Dockerfile":                 "Docker",
		"docker/worker.dockerfile":       "Docker",
		"Dockerfile.dockerignore":        "",
		"docs/dockerfile-guide.md":       "Markdown",
		"notes.unknownext":               "",
		"compose.yaml":                   "YAML",
		"deploy/docker-compose.prod.yml": "YAML",
	} {
		if got := Language(p); got != want {
			t.Errorf("%s: got %q, want %q", p, got, want)
		}
	}
}

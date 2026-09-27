---
id: REQ-LANG-015
uuid: 938c55d0-f066-4280-87ab-f1e080beaef0
title: Language detection by name
scope: lang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** determine a file's language from its extension,
case-insensitively, or from well-known file names (for example `Dockerfile`,
`Makefile`, `go.mod`, `Gemfile`, `Rakefile`, `rebar.config`, an OTP
`*.app.src`, R's `DESCRIPTION`, `NAMESPACE`, `renv.lock` and `packrat.lock`,
Haskell's `cabal.project`, `stack.yaml` and `package.yaml`, Terraform's
`.terraform.lock.hcl` and `*.tf.json`, Terragrunt's `terragrunt.hcl`, Buf's
`buf.yaml`, `buf.work.yaml`, `buf.lock` and `buf.gen.yaml`, the shells'
and direnv's startup files (`.bashrc`, `.zshrc`, `.profile`, `.envrc` and
the others of REQ-SHELL-001), and the Dockerfile names of REQ-DOCKER-001),
or, for a file neither names, from a `#!` line running a shell
(REQ-SHELL-001), and **shall** leave the language empty when none is known.

## Rationale

Detecting by name needs no reading of the file and is enough for the language
colors and filters. An extensionless script says what it is only in its `#!`
line, which the scan reads from the bytes it already peeks at to tell binary
files apart.

## Acceptance criteria

1. `a.go` is Go, `x.py` is Python, `Dockerfile`, `Containerfile`,
   `Dockerfile.dev` and `api.Dockerfile` are Docker; `Gemfile`,
   `lib/tasks/db.rake` and `app.gemspec` are Ruby; `mix.exs` is Elixir,
   `include/state.hrl`, `rebar.config` and `src/shop.app.src` are Erlang;
   `R/cart.R`, `DESCRIPTION` and `renv.lock` are R, `vignettes/intro.Rmd` is
   R Markdown and `report.qmd` is Quarto; `src/Data/Shop.hs`, `.hs-boot`,
   `doc/Tutorial.lhs` and `stack.yaml` are Haskell, `shop.cabal` and
   `cabal.project` are Cabal; `infra/main.tf`, `main.tf.json`, `prod.tfvars`
   and `.terraform.lock.hcl` are Terraform, `main.tofu` is OpenTofu,
   `terragrunt.hcl` is Terragrunt and `root.hcl` is HCL; `shop.proto` is
   Protobuf and `buf.yaml`, `buf.work.yaml`, `buf.lock` and `buf.gen.yaml`
   are Buf; `scripts/build.sh`, `test/app.bats`, a `.zsh-theme`, `.envrc`,
   `.zshrc` and `.bash_profile` are Shell, and so is an extensionless file
   starting `#!/usr/bin/env -S bash -e` or `#! /bin/sh`, while one starting
   `#!/usr/bin/python3` has no language and `lib/x.py` stays Python.
2. A file with an unknown extension has no `lang`.

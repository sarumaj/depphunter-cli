---
id: REQ-TERRAFORM-001
title: Terraform, OpenTofu and Terragrunt files claimed
scope: terraform
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Terraform plugin **shall** claim Terraform and OpenTofu configuration files
(`.tf`, `.tofu`, `.tf.json`, `.tofu.json`), variable files (`.tfvars`,
`.tfvars.json`), dependency lock files (`.terraform.lock.hcl`) and every other
`.hcl` file as a Terragrunt configuration except Packer's `.pkr.hcl` and
Nomad's `.nomad.hcl`, and **shall not** claim files under `.terraform` or
`.terragrunt-cache`. The kind of file is part of the extraction cache key, as
`.tf.json` and `.tfvars.json`, and `.terraform.lock.hcl` and `terragrunt.hcl`,
share an extension.

## Rationale

A Terraform module is spread over several files of one directory; Terragrunt
keeps its configuration in `.hcl` files of any name (`root.hcl`,
`_envcommon/*.hcl`, `env.hcl`). What `terraform init` and Terragrunt download
into `.terraform` and `.terragrunt-cache` is not the project's code.

## Acceptance criteria

1. `main.tf`, `main.tofu`, `main.tf.json`, `a.tfvars`, `a.auto.tfvars.json`,
   `.terraform.lock.hcl`, `terragrunt.hcl` and `root.hcl` are claimed;
   `image.pkr.hcl`, `job.nomad.hcl`, `.terraform/modules/vpc/main.tf` and
   `live/.terragrunt-cache/x/main.tf` are not.
2. `a.tf.json` and `a.tfvars.json` do not share a cache key, nor do
   `.terraform.lock.hcl` and `terragrunt.hcl`.

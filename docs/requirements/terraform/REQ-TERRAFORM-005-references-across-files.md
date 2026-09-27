---
id: REQ-TERRAFORM-005
uuid: bdd7a033-2e2f-46a9-ae11-acb1a482331d
title: References across the files of a module
scope: terraform
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record the `var.x`, `local.x`, `module.x`, `data.t.n`,
`ephemeral.t.n` and `t.n` references of a configuration file's expressions and
string templates (heredocs included) that the file does not declare itself,
except in `moved`, `removed`, `import` and `terraform` blocks, for-expression
variables and dynamic blocks' iterators, and **shall** resolve each to the file
of the same directory that declares it, dropping it when none does.

## Rationale

A Terraform module is one namespace spread over files by convention
(`variables.tf`, `outputs.tf`, `main.tf`); which file uses which is the
module's internal structure, and it is cheap to know exactly.

## Acceptance criteria

1. The fixture's `outputs.tf` imports `main.tf` (for `aws_instance.web` and
   `module.network`) and `locals.tf` (for `local.name` in a heredoc).
2. A reference in a comment, the `from` of a `moved` block, `$${...}` and a
   dynamic block's `tag.key` are not imports.

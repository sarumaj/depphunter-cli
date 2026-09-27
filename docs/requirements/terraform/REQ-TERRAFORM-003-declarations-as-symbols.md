---
id: REQ-TERRAFORM-003
uuid: ccde8b96-b93b-454a-8fef-d5c4e04c9f2c
title: Declarations as symbols
scope: terraform
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record as symbols of a configuration file its managed
resources (`type.name`, kind `resource`), data sources (`data.type.name`) and
ephemeral resources (`ephemeral.type.name`), module calls (`module.name`),
variables (`var.name`), outputs (`output.name`), checks (`check.name`), locals
(`local.name`, one per entry) and provider configurations (`provider.name`,
`provider.name.alias`); of a variable file the variables it sets (kind
`value`); and of a Terragrunt configuration its locals, dependencies
(`dependency.name`) and includes (`include` or `include.name`), each with its
line.

## Rationale

These are the names by which Terraform code refers to things, so they are
what a reader of the map looks for.

## Acceptance criteria

1. The fixture's `main.tf` has the symbols of its provider configurations
   (`provider.aws.west` for the alias), module calls and resources, with
   `aws_instance.web` on line 65; `terraform.tfvars` has `region`, `cidr` and
   `subnets`.
2. A `.tf.json` file's symbols carry the lines of their keys.

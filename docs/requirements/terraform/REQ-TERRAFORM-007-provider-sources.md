---
id: REQ-TERRAFORM-007
uuid: 29286448-f2bb-4735-9ce9-c8acd6d58048
title: Provider sources
scope: terraform
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A provider import **shall** resolve to the `terraform-provider` package named
by the source address the module's `required_providers` gives its local name
(across all of the module's files), else `hashicorp/<local name>` as Terraform
assumes; the address lower-cased and without the host when it is
`registry.terraform.io` or `registry.opentofu.org`. The module's version
constraints for that name (from `required_providers` and legacy
`provider { version }` arguments) are joined, separated by commas. The built-in
`terraform.io/builtin/terraform` is dropped.

## Rationale

The source address, not the local name, identifies a provider;
`registry.opentofu.org/hashicorp/aws` and `hashicorp/aws` are one provider.

## Acceptance criteria

1. In the fixture, `random` from `registry.opentofu.org/hashicorp/random`
   is `hashicorp/random`, `acme` keeps its host `tf.corp.test`, the implicit
   `google` is `hashicorp/google`, and the JSON module's `cloudflare` has the
   constraint `>= 4.0, < 5.0` in `zones.tf` too.

---
id: REQ-TERRAFORM-008
uuid: 01ef863c-b86f-4fba-a180-91ec54751a3c
title: Dependency lock files
scope: terraform
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read `.terraform.lock.hcl`: each `provider` block is an
import of that provider pinned to its `version`, and it pins the providers of
its own module and of every module that module calls through local sources,
directly or not, with the module's constraints (else the lock's
`constraints`) as the requested version. A lock file in a directory above a
module that does not call it is not used.

## Rationale

Terraform and OpenTofu write one lock file per root module, and it decides
the provider versions of every module the root calls.

## Acceptance criteria

1. The fixture's `hashicorp/aws` is `5.31.0` requested as `~> 5.0` in the
   root module and in `modules/network`, which the root calls; the uncalled
   `modules/legacy` keeps its constraint `~> 2.0`.

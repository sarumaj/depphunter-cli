---
id: REQ-TERRAFORM-008
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

The plugin **shall** read `.terraform/modules/modules.json`, which
`terraform init` writes in a root module (from disk: `.terraform/` is not
scanned): each entry's key (the module calls from the root down, `vpc` or
`network.vpc`), source and installed version. A registry module call is pinned
to the version installed for it (REQ-TERRAFORM-009), looked up in the calling
module's own file (key `<call>`) or that of a module calling it, directly or
not (a key ending in `.<call>`). The entries one key segment below an
installed registry module, through its local module calls, **shall** be its
dependencies (`--resolve-depth`), registry ones pinned at their installed
version, others by their source's rule. A garbage or missing file changes
nothing.

## Rationale

Terraform and OpenTofu write one lock file per root module, and it decides
the provider versions of every module the root calls.

## Acceptance criteria

1. The fixture's `hashicorp/aws` is `5.31.0` requested as `~> 5.0` in the
   root module and in `modules/network`, which the root calls; the uncalled
   `modules/legacy` keeps its constraint `~> 2.0`.
2. With a `modules.json`, `module "vpc"` (`~> 5.0`) is `5.1.2` requested as
   `~> 5.0`, `module "label"` (no version) `0.25.0`, and `modules/net`'s
   `module "sg"` (`>= 4`) `4.17.2` through the root's file; `vpc` depends on
   the git module `tf-dns` and, through its local module `inner`, on
   `cloudposse/label/null` 0.25.0.

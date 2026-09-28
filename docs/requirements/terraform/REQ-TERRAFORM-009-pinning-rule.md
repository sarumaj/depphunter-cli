---
id: REQ-TERRAFORM-009
title: Pinning rule for modules and providers
scope: terraform
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A registry module **shall** be pinned when its `version` allows one version
(`1.2.3`, `= 1.2.3`), shown bare; a range (`~> 6.0`, `>= 1.0`) is kept as its
version and neither pinned nor floating; no version floats - unless
`terraform init` installed the call (REQ-TERRAFORM-008): then it is pinned to
the installed version, the range requested. A remote module **shall** be
pinned only by a full commit `ref`; another ref (a tag, a branch) is its
version, neither pinned nor floating (a tag can be moved); no ref floats, and
so does an archive or bucket object without one. A provider **shall** be
pinned by a lock file (REQ-TERRAFORM-008) or a single exact constraint, else
keep its constraints, and float without any.

## Rationale

The rule of GitHub Actions (REQ-CI-011) for git references; Terraform does
not lock module versions, so only the call, or what `terraform init`
installed for it, pins a module.

## Acceptance criteria

1. The fixture's `module "vpc"` (`5.1.2`) and `module "private"`
   (`= 2.0.1`) are pinned, `module "iam"` (`~> 6.0`) is not, `module "label"`
   floats; `module "dns"` (a commit) is pinned, `module "storage"` (`v1.2.0`)
   is neither, `module "queue"` and the S3 `module "cdn"` float.

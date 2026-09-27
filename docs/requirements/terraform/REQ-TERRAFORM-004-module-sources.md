---
id: REQ-TERRAFORM-004
title: Module sources resolved
scope: terraform
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A module source **shall** resolve as follows: a local path (`./`, `../`, or
a bare directory name as Terraform 0.11 read one) to that directory of the
repository; a registry address `namespace/name/provider` with an optional
host and `//subdirectory` (Terragrunt's `tfr://host/namespace/name/provider`
too) to the `terraform-module` package of that name, lower-cased, without the
host when it is `registry.terraform.io` or `registry.opentofu.org`; any other
address go-getter fetches (`git::`, `hg::`, `s3::`, `gcs::`, GitHub and
Bitbucket shorthands, scp-like `git@host:path`, http(s) archives) to the
`terraform-module` package named by its normalized location - getter prefix,
scheme, credentials, `.git` and the query removed, host lower-cased,
`//subdirectory` kept - with the source as written (without the query) as its
origin and the `ref` as its version.

## Rationale

Local modules are code of the repository and draw as edges to their
directory; registry and remote modules are external dependencies. Both public
registries serve the same namespaces, so one module is one package whichever
tool installs it; a remote module is known by where it is fetched from, and
its origin keeps it from being named to any index.

## Acceptance criteria

1. `./modules/network` resolves to the directory `modules/network`,
   `registry.terraform.io/terraform-aws-modules/iam/aws//modules/iam-role` to
   `terraform-aws-modules/iam/aws//modules/iam-role`,
   `app.terraform.io/acme/db/aws` keeps its host.
2. `git::https://github.com/acme/tf-modules.git//storage?ref=v1.2.0` is
   `github.com/acme/tf-modules//storage` at `v1.2.0` with origin
   `git::https://github.com/acme/tf-modules.git//storage`;
   `git::ssh://git@Example.com:2222/org/repo.git` is `example.com/org/repo`.

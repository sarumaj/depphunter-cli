---
id: REQ-AUTH-015
uuid: 1da5a7af-e0aa-4334-ae88-79527363e5ae
title: Terraform registry tokens
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read Terraform and OpenTofu registry tokens from the
`credentials "<host>"` blocks of the CLI configuration (the file
`TF_CLI_CONFIG_FILE` names, else `~/.terraformrc` and `~/.tofurc`), from
`~/.terraform.d/credentials.tfrc.json` and
`~/.config/opentofu/credentials.tfrc.json`, and from `TF_TOKEN_<host>`
variables (`.` as `_`, `-` as `__`) for `app.terraform.io` and the hosts those
files name, and **shall** file each as a Bearer credential for its host. A host
named by a `credentials` or `host` block, with or without a token, is a
registry this machine knows.

## Rationale

`terraform login` and HCP Terraform keep private-registry tokens there; the
variables are how a pipeline supplies them, and cannot be listed.

## Acceptance criteria

1. Tokens from `~/.terraformrc`, `credentials.tfrc.json`,
   `TF_TOKEN_app_terraform_io` and `TF_TOKEN_mirror_corp__x_test` (for a host
   a `host` block names) are filed under their hosts.
2. With `TF_CLI_CONFIG_FILE` set, `~/.terraformrc` is not read.

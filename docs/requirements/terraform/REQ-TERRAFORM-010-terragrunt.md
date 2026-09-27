---
id: REQ-TERRAFORM-010
uuid: 2d31fad4-0e26-4888-9d10-cd4f2b0794a4
title: Terragrunt configurations
scope: terraform
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

In a Terragrunt configuration the plugin **shall** resolve the `terraform`
block's `source` as a module source (REQ-TERRAFORM-004) relative to the
configuration, after substituting the file's string locals and the string
locals of an included file (`${include.<name>.locals.<local>}`); a
`dependency` block's `config_path` and each of `dependencies { paths }` to
the `terragrunt.hcl` of that directory, else the directory; and
`find_in_parent_folders("name")` (`terragrunt.hcl` without an argument), in an
include's `path` or anywhere else such as `read_terragrunt_config`, to the
nearest file of that name in a directory above the configuration's, with
`${dirname(find_in_parent_folders(...))}/rest` resolving to `rest` beside it.

## Rationale

Terragrunt live repositories consist of little else: which module each unit
deploys, which units it depends on, and which shared files it includes.

## Acceptance criteria

1. The fixture's `live/prod/app/terragrunt.hcl` resolves its source, built
   from `_envcommon/app.hcl`'s `base_source_url`, to
   `github.com/acme/infra-modules//app` at `v0.8.0`, its dependency and
   dependencies to the other units' `terragrunt.hcl`, and its includes and
   `read_terragrunt_config` to `live/root.hcl`, `live/_envcommon/app.hcl` and
   `live/prod/env.hcl`.
2. `tfr:///terraform-aws-modules/vpc/aws?version=5.1.2` is that registry
   module, pinned, and `../../..//modules/dns` the directory `modules/dns`.

---
id: REQ-TERRAFORM-002
uuid: 31eaf906-847b-4500-b66e-ba941bbbc330
title: Module calls and providers as imports
scope: terraform
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

For a configuration file the plugin **shall** record as imports: each
`module` block with a literal `source` (and its `version`); each entry of a
`terraform { required_providers { ... } }` block, in the object form
(`source`, `version`) and the legacy string form (a version constraint); each
`provider` configuration block; and, once per provider local name the file
neither requires nor configures, the provider a `resource`, `data` or
`ephemeral` block uses: the prefix of its type before the first underscore
(`aws_instance`: `aws`), or the local name of its `provider` argument
(`aws.west`: `aws`). The built-in `terraform` provider (`terraform_data`,
`terraform_remote_state`) is not an import.

## Rationale

A module's dependencies are the modules it calls and the providers it
installs; a module that uses a provider without declaring it still installs
it, as HashiCorp's provider of that name.

## Acceptance criteria

1. The fixture's root module has imports for its nine module calls, its three
   required providers, its `aws` provider configuration and the `google`
   provider its `google_storage_bucket` uses, and none for `terraform_data`
   or `terraform_remote_state`.

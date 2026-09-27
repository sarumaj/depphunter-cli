---
id: REQ-SUP-050
title: Terraform module dependencies from a module registry
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read a Terraform registry module's dependencies
from its registry's module service: the public Terraform Registry
(`https://registry.terraform.io/v1/modules/`) for a module named without a
host, else the host the name starts with, whose `modules.v1` path is read from
its `/.well-known/terraform.json` (once per registry). The module's
`<namespace>/<name>/<provider>/versions` document gives the providers
(`terraform-provider`, named as the plugin names them) and registry modules
the chosen version's root module - or the submodule named after `//` -
requires; the chosen version is the one asked for, else the newest release
(not a pre-release) the version constraint allows (`=`, `!=`, `<`, `<=`, `>`,
`>=`, `~>`). A host is known when this machine's Terraform configuration names
it (REQ-AUTH-015) or the user vouches for it (`--trust-index`); providers are
not asked about.

## Rationale

The registry protocol lists every published version with the providers and
modules it declares, which is what a module brings into a configuration.

## Acceptance criteria

1. Against a stub registry reached through service discovery, `acme/vpc/aws`
   at `~> 5.1` answers with 5.2.0's providers `hashicorp/aws` (`>= 5.20`) and
   `hashicorp/random` and module `cloudposse/label/null` (pinned `0.25.0`),
   not with its local or git dependencies; `//modules/endpoints` answers with
   that submodule's; no version answers with the newest release, not a
   pre-release.
2. `tf.corp.test/acme/vpc/aws` is served by `https://tf.corp.test`, known when
   `~/.terraformrc` has a credentials block for it; an unnamed host is not
   known until `--trust-index` names it.

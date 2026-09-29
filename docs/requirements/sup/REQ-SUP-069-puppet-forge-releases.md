---
id: REQ-SUP-069
title: The Puppet Forge's releases for module dependencies
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

With `--online`, the index client **shall** answer what a module of the
`puppet-forge` island depends on from the Forge's v3 API: the `metadata`
(metadata.json) of `<api>/v3/releases/<slug>-<version>` for an exact version,
else of the newest release `<api>/v3/modules/<slug>` lists that the target's
requirement admits (`>= 4.13.1 < 10.0.0`, `1.x`, `~ 1.2`, a hyphen range, `||`
alternatives; none for the newest), deleted releases and pre-releases left
out, the listed `current_release` used without another request when it is the
one. Each dependency **shall** be named by its slug (`puppetlabs/stdlib` is
`puppetlabs-stdlib`) with its `version_requirement` as written. A name that
is not a slug (a git module named by its repository) is not asked.

forgeapi.puppet.com is the public index. A Puppetfile's `forge "<url>"` line
**shall** be recorded as the repository's source (a bare host is https; the
public Forge's own hosts are not recorded) and `forge: baseurl` of r10k's
configuration (`/etc/puppetlabs/r10k/r10k.yaml`, else `/etc/r10k.yaml`, the
first that exists) as this machine's; either replaces the public Forge.

## Rationale

A Forge module's dependencies are in its metadata.json, which the Forge's API
serves per release; r10k installs only what a Puppetfile lists, so what a
module needs is otherwise unknown.

## Acceptance criteria

1. puppetlabs-apache 12.0.0 yields puppetlabs-concat and puppetlabs-stdlib
   with their requirements, from one request; `>= 11.0.0 < 13.0.0` asks the
   module only (the current release); `11.x` skips the deleted 11.2.0 and asks
   11.1.0.
2. A Puppetfile naming forgeapi.puppetlabs.com records nothing; one naming
   another Forge records it untrusted; r10k.yaml's baseurl is trusted.

## Notes

The sandbox proxy blocks forgeapi.puppet.com, so the API's shape was taken from
Puppet's documentation and the puppet_forge gem's client and verified with a
stub server only. Puppet's own `module_repository` setting (puppet.conf) and
r10k's `allow_puppetfile_override` are not read.

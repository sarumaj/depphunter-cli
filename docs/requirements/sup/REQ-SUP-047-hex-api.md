---
id: REQ-SUP-047
title: Hex dependencies from the Hex API
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read a Hex package's dependencies from the Hex API
(`https://hex.pm/api` unless `HEX_API_URL` names another): the package
(`<api>/packages/<name>`) for its releases and latest stable release, then the
release asked for, or else that one (`<api>/packages/<name>/releases/<v>`), its
non-optional requirements keyed by package name. Gleam packages are Hex
packages and are asked the same way (REQ-GLEAM-009). The API **shall** be
`HEX_API_URL`, `HEX_API` or the `api_url` of Hex's `hex.config`
([REQ-SUP-064](REQ-SUP-064-tool-configuration-locations.md)) when set.

A package of a private organization (repository `hexpm:<organization>`,
[REQ-BEAM-013](../beam/REQ-BEAM-013-hex-organization-packages.md)) **shall**
be asked of `<api>/repos/<organization>/packages/<name>` and its releases
alone, with this machine's key
([REQ-AUTH-028](../auth/REQ-AUTH-028-hex-keys.md)), and never of the public
packages; its requirements, which the API does not place in a repository,
**shall** be taken to be the organization's as well. Without a key for the
organization it **shall** not be asked, and the resolution report **shall**
say that the key is missing. A package of any other repository **shall** be
asked of no index.

## Rationale

`rebar.lock` records no edges and a library commits no lock, so what a Hex
package depends on is only on hex.pm; the repository mirrors `HEX_MIRROR` names
serve signed protobuf files, not this API.

## Acceptance criteria

1. Against a stub API, plug 1.15.0 depends on `mime` `~> 1.0 or ~> 2.0` and
   `plug_crypto` 2.0.0 (pinned).
2. A requirement names no release, so the latest stable release's requirements
   are returned, without the optional `jason`.
3. An organization's package is asked of `/api/repos/acme/` with the key, its
   requirements carry the organization, one the organization lacks is not
   asked of the public packages, and a public package is asked without the
   key.
4. Without a key nothing is asked and the report gives the reason; a
   repository other than `hexpm:<organization>` has no index.

## Notes

A repository with a URL of its own (a mini_repo, `HEX_MIRROR`) serves Hex's
signed protobuf registry, not this API, and is not read. A rebar3
dependency's repository is not read: its organization packages are asked of
the public packages like any other.

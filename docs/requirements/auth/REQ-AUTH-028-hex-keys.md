---
id: REQ-AUTH-028
title: Hex API keys for private organizations
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

The system **shall** read what Mix's Hex authenticates to the Hex API with,
and send it only to the private organizations' part of that API
(`<api>/repos/`), the API being `HEX_API_URL`, `HEX_API`, the `api_url` of
`hex.config`, else `https://hex.pm/api`:

- the user's key: `HEX_API_KEY`, else the `api_key` of `hex.config`, else
  the OAuth access token `mix hex.user auth` stores (`$oauth_token`) while it
  has not expired; it serves every organization;
- when there is none of those: the `auth_key` (or unexpired OAuth token) of
  each `hexpm:<organization>` repository in `hex.config`'s `$repos`, as `mix
  hex.organization auth <organization> --key KEY` writes it, for that
  organization's path only, and `HEX_REPOS_KEY` for the other organizations.

`hex.config` **shall** be found in `HEX_HOME`, else `$XDG_CONFIG_HOME/hex`
(`~/.config/hex`) under `MIX_XDG=1` or `true`, else `~/.hex`. A key **shall**
be sent as the whole `Authorization` header and a token as a Bearer token, as
Hex sends them. A value that is not a token's characters **shall** send
nothing.

## Rationale

A private organization's packages answer only to a key; without one hex.pm
says "not found", which reads as a package that does not exist.

## Acceptance criteria

1. `HEX_API_KEY` wins over `api_key`, which wins over the OAuth token; each
   reaches `<api>/repos/<any organization>/` and no public package's URL.
2. Without them, a `hexpm:acme` `auth_key` reaches `/repos/acme/` alone, an
   expired OAuth token nothing, and `HEX_REPOS_KEY` the other organizations.
3. `HEX_API_URL` moves the key to its host.
4. End to end, a `hex.config` found through `HEX_HOME` makes the
   organization's package answer.

## Notes

Organization keys that `mix hex.organization auth` generated itself (without
`--key`) carry only the `repository:<organization>` permission, while hex.pm's
API asks for `api:read`: such a key is sent, hex.pm refuses it, and the report
says so. A user key, a user's OAuth token, or an organization key generated
with `--permission api:read` answers. The encrypted keys of older Hex versions
(`$encrypted_key`, `$write_key`) need the local password and are not read;
nor is a refresh token exchanged for a new access token. rebar3's own Hex
configuration (`~/.config/rebar3/hex.config`) is not read.

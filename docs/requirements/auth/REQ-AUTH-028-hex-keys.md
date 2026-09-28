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

The system **shall** read what Mix's Hex and rebar3 authenticate to the Hex
API with, and send it only to the private organizations' part of that API
(`<api>/repos/`), the API being `HEX_API_URL`, `HEX_API`, the `api_url` of
`hex.config`, else `https://hex.pm/api`:

- `HEX_API_KEY`, for every organization, over anything a file holds;
- an organization's own `api_key` in rebar3's `hex.config`
  (`<<"hexpm:<organization>">> => #{api_key => ...}`), for that
  organization's path, as rebar3 takes it before the user's;
- the user's key: the `api_key` of Mix's `hex.config`, else the OAuth access
  token `mix hex.user auth` stores (`$oauth_token`) while it has not expired,
  else the `api_key` of rebar3's `hexpm` repository, else the unexpired
  OAuth access token `rebar3 hex user auth` stores (`$oauth`); it serves
  every organization;
- when there is none of those: the `auth_key` (or unexpired OAuth token) of
  each `hexpm:<organization>` repository in Mix's `$repos`, as `mix
  hex.organization auth <organization> --key KEY` writes it, else the
  `repo_key` (or `auth_key`, or unexpired `oauth_token`) of that repository
  in rebar3's `hex.config`, as `rebar3 hex organization auth
  hexpm:<organization>` writes it, for that organization's path only, and
  `HEX_REPOS_KEY` for the other organizations.

Mix's `hex.config` **shall** be found in `HEX_HOME`, else
`$XDG_CONFIG_HOME/hex` (`~/.config/hex`) under `MIX_XDG=1` or `true`, else
`~/.hex`. rebar3's **shall** be `.config/rebar3/hex.config` under
`REBAR_GLOBAL_CONFIG_DIR`, else `REBAR_CACHE_DIR`, else the home directory: one
map from repository name to its keys, the atom `undefined` being no key. A key **shall**
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
5. From rebar3's `hex.config`: a `hexpm:acme` `repo_key` reaches `/repos/acme/`
   alone (end to end too), an organization's `api_key` wins over the user's
   for it, `hexpm`'s `api_key` wins over `$oauth` and serves every
   organization, Mix's keys come before rebar3's, `HEX_API_KEY` is over all,
   and `REBAR_GLOBAL_CONFIG_DIR` moves the file.

## Notes

Organization keys that `mix hex.organization auth` generated itself (without
`--key`) carry only the `repository:<organization>` permission, while hex.pm's
API asks for `api:read`: such a key is sent, hex.pm refuses it, and the report
says so. A user key, a user's OAuth token, or an organization key generated
with `--permission api:read` answers. The encrypted keys of older Hex versions
(`$encrypted_key`, `$write_key`) need the local password and are not read;
nor is a refresh token exchanged for a new access token. rebar3's
`repo_key` is its repository key, like Mix's `auth_key`, and may meet the same
refusal.

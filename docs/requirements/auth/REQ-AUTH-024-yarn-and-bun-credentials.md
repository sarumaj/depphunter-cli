---
id: REQ-AUTH-024
title: Yarn Berry and Bun registry credentials
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

The system **shall** read the registry credentials of this machine's Yarn
Berry configuration (the file `YARN_RC_FILENAME` names, else `.yarnrc.yml`, in
the home directory): the top-level `npmAuthToken` and `npmAuthIdent`, those of
each `npmScopes` entry and those of each `npmRegistries` entry (a key written
`//host/path` meaning `https:`), with `YARN_NPM_REGISTRY_SERVER`,
`YARN_NPM_AUTH_TOKEN` and `YARN_NPM_AUTH_IDENT` replacing the top level. A
token **shall** be sent as a Bearer token and an ident (`user:password`, or
that pair base64) as Basic credentials, to the registry each belongs to: an
`npmRegistries` entry's to its key, a scope's to the scope's
`npmRegistryServer` (else the default), the top level's to the default
registry (the file's `npmRegistryServer`, else Yarn's registry.yarnpkg.com and
npm's registry.npmjs.org). `${NAME}`, `${NAME-x}` and `${NAME:-x}` **shall** be
resolved from the machine's environment; a value naming an unset variable
without a fallback **shall** be dropped.

The system **shall** read the credentials of Bun's global bunfig
(`$XDG_CONFIG_HOME/.bunfig.toml` when it exists, else `~/.bunfig.toml`): the
`token`, or the `username` and `password`, of `[install] registry` and of each
`[install.scopes]` entry, in a table or written into the URL
(`https://user:password@host/`, `https://:token@host/`), `$NAME` and `${NAME}`
resolved from the environment.

Each credential **shall** serve its registry's path only
([REQ-AUTH-025](REQ-AUTH-025-npm-path-scoped-credentials.md)); a credential
npm's configuration holds for the same registry **shall** be kept.

## Rationale

Yarn Berry does not read the npmrc, so a developer or pipeline using it keeps
the company registry's token in `.yarnrc.yml` or `YARN_NPM_AUTH_TOKEN`, and a
Bun user in `bunfig.toml`; without them the private registry answers 401.

## Acceptance criteria

1. A scope's registry and token in the home `.yarnrc.yml` are used for the
   scope's packages, end to end.
2. An `npmRegistries` ident is sent as Basic credentials to its registry's
   path, and nowhere else on the host; a base64 ident is decoded.
3. `${VAR}` and `${VAR:-fallback}` are resolved; an unset variable without a
   fallback yields nothing.
4. `YARN_NPM_*` replace the file's top level, and `YARN_RC_FILENAME` names
   the file.
5. Bun's table token, URL `user:password` and URL `:token` are sent, `$VAR`
   resolved, and `$XDG_CONFIG_HOME/.bunfig.toml` wins over the home file.
6. The npmrc's token for a registry wins over Yarn's.

## Notes

A repository's `.yarnrc.yml` and `bunfig.toml` are never read for this
machine's credentials; a variable they refer to is lent only to a registry
this machine vouches for
([REQ-AUTH-023](REQ-AUTH-023-repository-feed-credentials-from-the-environment.md)).
The `.yarnrc.yml` files Yarn finds in the directories between the project and
the root are not read. `npmAlwaysAuth` is not needed: a credential is always
sent to its registry. Credentials are filed by registry, not by package scope:
two scopes on one registry path share one credential, the first scope by name
winning. `YARN_NPM_SCOPES` and the other structured `YARN_*` settings are not
read.

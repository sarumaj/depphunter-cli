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
Berry configuration: the files of the directories above the analyzed one
that lie outside its checkout (the name `YARN_RC_FILENAME` gives them, else
`.yarnrc.yml`;
[REQ-SUP-079](../sup/REQ-SUP-079-configuration-files-above-the-analyzed-directory.md)),
then the home directory's `.yarnrc.yml`, merged key by key at every depth,
the closest winning, as Yarn merges them. It **shall** take the top-level
`npmAuthToken` and `npmAuthIdent`, those of each `npmScopes` entry and those
of each `npmRegistries` entry (a key written `//host/path` meaning
`https:`), with `YARN_NPM_REGISTRY_SERVER`, `YARN_NPM_AUTH_TOKEN`,
`YARN_NPM_AUTH_IDENT` and `YARN_NPM_ALWAYS_AUTH` replacing the top level. A
token **shall** be sent as a Bearer token and an ident (`user:password`, or
that pair base64) as Basic credentials, to the registry each belongs to: an
`npmRegistries` entry's to its key, a scope's to the scope's
`npmRegistryServer` (else the default), the top level's to the default
registry (the file's `npmRegistryServer`, else Yarn's registry.yarnpkg.com and
npm's registry.npmjs.org) unless an `npmRegistries` entry names that registry.
`${NAME}`, `${NAME-x}` and `${NAME:-x}` **shall** be resolved from the
machine's environment; a value naming an unset variable without a fallback
**shall** be dropped.

A credential **shall** go with the requests Yarn sends it with when it fetches
a package's metadata: a scope's with that scope's packages only; the top
level's and an `npmRegistries` entry's with every package when its
`npmAlwaysAuth` is `true` (or `1`), else with the scoped packages only, which
Yarn always authenticates. A scope's credential **shall** win over its
registry's for the scope's packages.

The system **shall** read the credentials of Bun's global bunfig
(`$XDG_CONFIG_HOME/.bunfig.toml` when it exists, else `~/.bunfig.toml`): the
`token`, or the `username` and `password`, of `[install] registry` and of each
`[install.scopes]` entry, in a table or written into the URL
(`https://user:password@host/`, `https://:token@host/`), `$NAME` and `${NAME}`
resolved from the environment.

Each credential **shall** serve its registry's path only
([REQ-AUTH-025](REQ-AUTH-025-npm-path-scoped-credentials.md)); a credential
npm's configuration holds for the same registry **shall** be kept, for the
scoped packages too.

## Rationale

Yarn Berry does not read the npmrc, so a developer or pipeline using it keeps
the company registry's token in `.yarnrc.yml` or `YARN_NPM_AUTH_TOKEN`, and a
Bun user in `bunfig.toml`; without them the private registry answers 401.
Sending a credential only with the requests Yarn sends it with keeps it off
those the registry answers without one, the link checker's included.

## Acceptance criteria

1. A scope's registry and token in the home `.yarnrc.yml` are used for the
   scope's packages, end to end.
2. An `npmRegistries` ident is sent as Basic credentials to its registry's
   path, and nowhere else on the host; a base64 ident is decoded.
3. `${VAR}` and `${VAR:-fallback}` are resolved; an unset variable without a
   fallback yields nothing.
4. `YARN_NPM_*` replace the file's top level; `YARN_RC_FILENAME` does not
   rename the home file, and `YARN_NPM_SCOPES` gives nothing.
5. Bun's table token, URL `user:password` and URL `:token` are sent, `$VAR`
   resolved, and `$XDG_CONFIG_HOME/.bunfig.toml` wins over the home file.
6. The npmrc's token for a registry wins over Yarn's, for a scoped package
   too.
7. Without `npmAlwaysAuth` a registry's token goes with a scoped package's
   request and not with an unscoped one's, end to end; with it (or
   `YARN_NPM_ALWAYS_AUTH`), with both; a scope's token wins for its scope and
   no other (`@acmex` is not `@acme`).
8. A `.yarnrc.yml` above the analyzed checkout is merged with the home's (a
   scope's token there, the scope's registry in the home file), and one
   inside the checkout gives nothing.

## Notes

A repository's `.yarnrc.yml` and `bunfig.toml` are never read for this
machine's credentials; a variable they refer to is lent only to a registry
this machine vouches for
([REQ-AUTH-023](REQ-AUTH-023-repository-feed-credentials-from-the-environment.md)).
Credentials are filed by registry, and a scope's by registry and scope: two
scopes on one registry path keep their own. Yarn sends the top level's
credential to a scope's registry too when the scope has none; here it serves
the default registry only. Yarn's `onConflict` directives are not followed.
`YARN_NPM_SCOPES` and `YARN_NPM_REGISTRIES` are not read: Yarn takes a
variable as a string, and refuses to run when a map setting gets one. Bun
sends its credentials with every request.

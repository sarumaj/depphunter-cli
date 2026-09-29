---
id: REQ-AUTH-023
title: A repository's feed gets an environment secret only when vouched for
scope: auth
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

A NuGet or Paket feed the repository names **shall** be given a credential only
when this machine supplies the secret - a `password: "%NAME%"` of a
`paket.dependencies` `source` line, a `ClearTextPassword` of `%NAME%` in the
repository's `nuget.config`, a `NuGetPackageSourceCredentials_<source>`
variable for a source the repository defines, or a credential of this
machine's `NuGet.Config` for a key the repository gives another URL - and the
feed is on the host of a NuGet source this machine's configuration names or is
vouched for with `--trust-index` (by URL or host). A password the repository
writes out **shall** be discarded, a Paket `authtype` other than `basic`
**shall** give nothing, and a host that already has a credential of this
machine's **shall** keep it.

The same **shall** hold for an npm-family registry the repository names: an
`npmAuthToken` or `npmAuthIdent` of a repository `.yarnrc.yml` that is exactly
`${NAME}` (or `${NAME:-x}` with `NAME` set), and a `token` or `password` of a
repository `bunfig.toml` that is exactly `$NAME` or `${NAME}`, **shall** be
lent only to a registry vouched for with `--trust-index` or on the host of a
registry this machine's npm, Yarn or Bun configuration names, and only for
that registry's path
([REQ-AUTH-025](REQ-AUTH-025-npm-path-scoped-credentials.md)). A token, ident
or password written out, one only a fallback fills, and a user:password in a
repository registry URL **shall** be discarded. A repository `.yarnrc.yml`
**shall** be read merged with those of the directories above it up to the
top of the checkout, as Yarn merges them
([REQ-SUP-079](../sup/REQ-SUP-079-configuration-files-above-the-analyzed-directory.md)),
and a credential lent from it **shall** go only with the requests Yarn sends
it with (without `npmAlwaysAuth`, those for scoped packages;
[REQ-AUTH-024](REQ-AUTH-024-yarn-and-bun-credentials.md)); it **shall not**
be lent for a registry this machine already holds a credential for.

The same **shall** hold for a Python index the repository names
([REQ-AUTH-026](REQ-AUTH-026-python-tool-credentials.md)): this machine's
`UV_INDEX_<NAME>_*` variables, Poetry http-basic credential or PDM
`[pypi.<name>]` credential for a repository index of that name, and a user
name or password that is exactly `$NAME` or `${NAME}` in a repository
`Pipfile` or PDM source URL, **shall** be lent only to an index vouched for
with `--trust-index` or on the host of a PyPI index this machine's
configuration names (or of a Poetry repository or explicit uv index of this
machine's), and only for that index's path. A password written into a
repository index URL **shall** be discarded.

## Rationale

The usual CI setup is a repository `nuget.config` or `paket.dependencies`
naming the company feed and a pipeline variable holding its token. The secret
is the machine's, but the URL it would be sent to is the repository's: sending
it anywhere a repository names would let the repository collect it
([REQ-AUTH-012](REQ-AUTH-012-url-credential-from-machine-only.md)), so the
user's say-so, or this machine's own configuration of that host, decides.

## Acceptance criteria

1. Without `--trust-index` and without a machine source on the host, no
   repository feed gets a credential.
2. Vouched for, a Paket `%NAME%` password, a `nuget.config` `%NAME%`
   `ClearTextPassword` and a `NuGetPackageSourceCredentials_` variable for a
   repository source are sent; a written-out password in either file and a
   Paket `authtype: "ntlm"` are not.
3. On the host of a machine source, the Paket password is sent; a machine
   credential for the host is kept over a lent one.
4. A vouched Paket feed is asked with the lent credential, end to end.
5. A repository `.yarnrc.yml` `${VAR}` token or ident and a `bunfig.toml`
   `$VAR` password are lent to a vouched registry or one on the host of a
   machine registry, and to nothing else; a written-out token, a fallback and
   a written-out URL password are not; a machine token for the host is kept,
   for a scope's packages too; a token of a nested `.yarnrc.yml` goes to the
   registry a file above it names.
6. A repository uv, Poetry or PDM index gets this machine's credential of its
   name, and a `Pipfile` or PDM `${VAR}` URL credential is lent, only when
   vouched for or on the host of a machine uv index, Poetry repository or PDM
   source, and only under the index's path; a written-out URL password is
   not lent.

## Notes

A user name may be written out (in a Paket source and in a `bunfig.toml`
table). A repository registry URL made of a variable is not recorded: its
value would come from this machine's environment and be drawn on the map.
Paket's own credential store
(`paket config add-credentials`, `paket.config` in the user's application data
directory) keeps passwords encrypted with a salt, and is not read.

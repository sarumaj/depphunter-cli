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

## Notes

A user name may be written out. Paket's own credential store
(`paket config add-credentials`, `paket.config` in the user's application data
directory) keeps passwords encrypted with a salt, and is not read.

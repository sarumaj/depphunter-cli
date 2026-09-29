---
id: REQ-SUP-043
title: Only the user may vouch for an index
scope: sup
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read `trust_indexes` only from the user's own
configuration, the environment and the command line; a `trust_indexes` entry in
a project configuration file **shall** be ignored.

A credential this machine holds for a host **shall not** vouch for an index the
repository names on that host: such an index is known only when the user vouches
for it, or when this machine's own configuration names it. The exceptions are the
ecosystems whose package names carry their host, container images
(`ghcr.io/org/app`), Terraform registry modules (`app.terraform.io/org/name/aws`)
and Buf Schema Registry modules (`buf.example.com/org/module`, whose token
buf keeps; REQ-SUP-072): there the host is the package's own, not an index put
in front of it, and a credential for it means this machine pulls from it
already.

## Rationale

A repository that could clear its own warning would leave no guard at all, which
is the whole point of the marking.

A credential says the user can reach a host, not that everything on it is theirs.
The hosts that carry credentials are mostly shared: GitHub Packages, GitLab,
Artifactory, Nexus and Azure Artifacts serve many organizations' feeds from one
host. Were a credential enough, a cloned repository could name another feed on
that host, have it asked ahead of the public index (dependency confusion) and, for
the ecosystems whose credentials go by host, have the user's secret sent to it.

## Acceptance criteria

1. A project configuration naming `trust_indexes` changes nothing.
2. The user's configuration and `--trust-index` are both honored.
3. A repository's index on a host this machine holds a credential for is not
   known, and its packages are marked, until the user vouches for it.
4. A container image, Terraform registry module or Buf Schema Registry module
   on a host this machine holds a credential for is known.

## Notes

`DEPPHUNTER_TRUST_INDEXES` is also read.

Counting a machine credential as trust was considered and rejected, for the reason
given above.

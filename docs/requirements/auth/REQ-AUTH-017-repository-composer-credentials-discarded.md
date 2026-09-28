---
id: REQ-AUTH-017
title: Repository Composer credentials discarded
scope: auth
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall not** take a credential from a repository's Composer files:
an `auth.json` beside a `composer.json`, the `"config"` of a `composer.json`
and a credential in a `composer` repository URL it names contribute nothing,
while the repository URL itself is still an index
([REQ-SUP-015](../sup/REQ-SUP-015-index-configuration-sources.md)).

## Rationale

As for [REQ-AUTH-012](REQ-AUTH-012-url-credential-from-machine-only.md): a
repository that could supply a credential could also choose where it is sent.

## Acceptance criteria

1. A checkout with `composer.json` and `auth.json` naming credentials for its
   repository host, in the working directory or under the home directory,
   makes no request carry a credential.
2. The repository's `composer` repositories are recorded without the
   credential.
